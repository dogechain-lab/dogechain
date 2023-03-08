package network

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/dogechain-lab/dogechain/helper/telemetry"
	"github.com/dogechain-lab/dogechain/network/common"
	"github.com/hashicorp/go-hclog"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	doubleSleepDuration      = DefaultBackgroundTaskSleep * 2
	connectFullSleepDuration = DefaultBackgroundTaskSleep * 6
	waitDiscoverDuration     = DefaultBackgroundTaskSleep * 12
)

// FSM for keep available status
type keepAvailableStatus int

const (
	kasSleeping              keepAvailableStatus = iota // default sleep status
	kasDiscoveryWaiting                                 // Discovery service is not ready
	kasMaxPeerSleeping                                  // Max peer sleep
	kasMarkConnectionPending                            // Mark the connection as pending
	kasRandomDialed                                     // Random dialed peer
	kasDialSleeping                                     // Dial sleep
	kasWeakUp                                           // Weak up
)

// keepAvailable keep available service
// clear the peerstore of peers that are not connected
// and will attempt to make new connections
// if the active peer count is lesser than the specified limit.
type keepAvailable struct {
	// server reference
	server *DefaultServer

	// logger
	logger hclog.Logger

	// tracer for keep available
	tracer telemetry.Tracer

	// Keep available status
	status keepAvailableStatus

	// Keep available timer
	timer *time.Timer

	// random instance
	randIns *rand.Rand

	// self id
	selfID peer.ID

	// max peers count
	maxPeers int

	// batch dial peers count
	batchDialPeers int

	// pending connect mark
	pendingConnectMark map[peer.ID]struct{}

	// closed channel
	closed chan struct{}

	// close sync
	closeWG sync.WaitGroup
}

// newKeepAvailable create a new keep available service
func newKeepAvailable(server *DefaultServer) *keepAvailable {
	//#nosec G404
	randIns := rand.New(rand.NewSource(time.Now().UnixNano()))
	selfID := server.host.ID()
	maxPeers := int(server.connectionCounts.maxInboundConnCount() + server.connectionCounts.maxOutboundConnCount())
	batchDialPeers := (int(server.connectionCounts.maxOutboundConnCount()) / 4) + 1 // 25% of max outbound peers
	pendingConnectMark := make(map[peer.ID]struct{})
	logger := server.logger.Named("keep_available")
	tracer := server.tracer.GetTraceProvider().NewTracer("keepAvailable")

	return &keepAvailable{
		server:             server,
		tracer:             tracer,
		logger:             logger,
		status:             kasSleeping,
		timer:              time.NewTimer(DefaultBackgroundTaskSleep),
		randIns:            randIns,
		selfID:             selfID,
		maxPeers:           maxPeers,
		batchDialPeers:     batchDialPeers,
		pendingConnectMark: pendingConnectMark,
		closed:             make(chan struct{}),
	}
}

// Start start run the keep available
func (ka *keepAvailable) Start() {
	go ka.run()
}

// Close stop the keep available
func (ka *keepAvailable) Close() {
	close(ka.closed)

	ka.timer.Stop()
	ka.closeWG.Wait()
}

// run run the keep available loop
func (ka *keepAvailable) run() {
	for {
		select {
		case <-ka.timer.C:
			// reset state to weakup
			ka.status = kasWeakUp
			ka.fsm()
		case <-ka.closed:
			return
		}
	}
}

// fsm keep available fsm
func (ka *keepAvailable) fsm() {
	// add wait group
	ka.closeWG.Add(1)
	defer ka.closeWG.Done()

	span := ka.tracer.Start("keepAvailable.fsm")
	defer span.End()

	// **status change sequence**
	// kasWeakUp -> checkDiscoveryServiceReady()
	//    if Discovery service not ready -> state set kasDiscoveryWaiting
	//    if Discovery service ready -> state set kasMarkPandingConnection
	// kasDiscoveryWaiting -> set timer wait next loop
	// kasMarkPandingConnection -> markPendingConnection()
	//    if peer count < max peers -> state set kasMaxPeerSleep
	//    if not free outbound connect slot -> state set kasSleep
	//    otherwise -> state set kasRandomDialed
	// kasMaxPeerSleep -> set timer wait next loop
	// kasRandomDialed -> randomDial()
	//    if dial peer -> state set kasDialSleeping
	//    if not dial peer -> state set kasSleep
	// kasDialSleeping -> set timer wait next loop
	// kasSleep -> set timer wait next loop

	for {
		// close check
		select {
		case <-ka.closed:
			return
		default:
		}

		ka.logger.Debug("keep available status", "status", ka.status)

		switch ka.status {
		case kasWeakUp:
			// first check discovery service is ready
			ka.checkDiscoveryServiceReady(span.Context())
		case kasDiscoveryWaiting:
			ka.timer.Reset(waitDiscoverDuration)

			return
		case kasMarkConnectionPending:
			ka.markPendingConnection(span.Context())
		case kasMaxPeerSleeping:
			ka.timer.Reset(connectFullSleepDuration)

			return
		case kasRandomDialed:
			ka.randomDial(span.Context())
		case kasDialSleeping:
			ka.timer.Reset(doubleSleepDuration)

			return
		case kasSleeping:
			ka.timer.Reset(DefaultBackgroundTaskSleep)

			return
		}
	}
}

// checkDiscoveryServiceReady check the discovery service is ready
func (ka *keepAvailable) checkDiscoveryServiceReady(ctx context.Context) {
	span := ka.tracer.StartWithContext(ctx, "keepAvailable.checkDiscoveryServiceReady")
	defer span.End()

	if ka.server.discovery == nil {
		ka.status = kasDiscoveryWaiting

		span.SetAttribute("discovery", "nil")

		return
	}

	span.SetAttribute("discovery", "not nil")

	// next status is mark pending connection
	ka.status = kasMarkConnectionPending
}

// markPendingConnection mark the connection as pending
func (ka *keepAvailable) markPendingConnection(ctx context.Context) {
	span := ka.tracer.StartWithContext(ctx, "keepAvailable.markPendingConnection")
	defer span.End()

	var waitingPeersDisconnect sync.WaitGroup

	disconnectFlag := false

	peers := ka.server.host.Network().Peers()

	ka.logger.Debug("ready connections", "count", len(peers))
	span.SetAttribute("ready_connections", len(peers))

	for _, peerID := range peers {
		if peerID == ka.selfID {
			continue
		}

		// check and mark if peer has pending status
		if ka.server.identity.HasPendingStatus(peerID) {
			if _, ok := ka.pendingConnectMark[peerID]; !ok {
				ka.logger.Debug("peer has pending status", "peer", peerID)

				ka.pendingConnectMark[peerID] = struct{}{}

				continue
			}
		} else {
			// clear pending connect mark
			delete(ka.pendingConnectMark, peerID)
		}

		_, isMark := ka.pendingConnectMark[peerID]
		if !ka.server.HasPeer(peerID) && isMark {
			ka.logger.Error("peer session not exist, disconnect peer", "peer", "bye")

			disconnectFlag = true

			// disconnect peer, but peersLock is locked, so use goroutine
			waitingPeersDisconnect.Add(1)

			span.AddEvent("disconnect_peer", map[string]interface{}{
				"peer": peerID,
			})

			go func(peerID peer.ID) {
				defer waitingPeersDisconnect.Done()
				ka.server.DisconnectFromPeer(peerID, "bye")
			}(peerID)
		}

		// clear pending connect mark
		delete(ka.pendingConnectMark, peerID)
	}

	if disconnectFlag {
		// wait peer disconnect finish
		waitingPeersDisconnect.Wait()
	}

	// clear pending connect mark, remove not exist peers
	copyMark := make(map[peer.ID]struct{})

	for _, peerID := range peers {
		if _, ok := ka.pendingConnectMark[peerID]; ok {
			copyMark[peerID] = struct{}{}
		}
	}

	ka.pendingConnectMark = copyMark

	span.SetAttribute("pendingConnectMark", copyMark)

	// next check max peer count
	if len(peers) >= ka.maxPeers {
		ka.status = kasMaxPeerSleeping

		return
	}

	// if not free outbound connection or disconnect peer, wait next loop
	if !ka.server.connectionCounts.HasFreeOutboundConn() || disconnectFlag {
		ka.status = kasSleeping

		return
	}

	// next status is random dialed
	ka.status = kasRandomDialed
}

// randomDial random dialed peer
func (ka *keepAvailable) randomDial(ctx context.Context) {
	span := ka.tracer.StartWithContext(ctx, "keepAvailable.randomDial")
	defer span.End()

	// defer set status to sleep, and wait group done
	defer func() {
		ka.status = kasSleeping
	}()

	// get routingTable peers
	routTablePeers := ka.server.discovery.GetConfirmPeers()
	if len(routTablePeers) == 0 {
		ka.logger.Error("no peers found")

		return
	}

	ka.logger.Debug("attempting to connect to random peer")

	// shuffle peers
	ka.randIns.Shuffle(
		len(routTablePeers),
		func(i, j int) {
			routTablePeers[i], routTablePeers[j] = routTablePeers[j], routTablePeers[i]
		})

	isDial := false
	dialCount := 0

	for _, randPeer := range routTablePeers {
		/// dial unconnected peer
		if ka.selfID != randPeer &&
			!ka.server.identity.HasPendingStatus(randPeer) &&
			!ka.server.bootnodes.isBootnode(randPeer) &&
			!ka.server.HasPeer(randPeer) {
			// use discovery service save
			peerInfo := ka.server.discovery.GetConfirmPeerInfo(randPeer)
			if peerInfo != nil {
				ka.logger.Debug("dialing random peer", "peer", peerInfo)
				ka.server.addToDialQueue(span.Context(), peerInfo, common.PriorityRandomDial)

				isDial = true
				dialCount++
			} else {
				ka.logger.Error("peer not found in discovery service", "peer", randPeer)
			}

			if dialCount >= ka.batchDialPeers {
				break
			}
		}
	}

	if !isDial {
		ka.logger.Info("all peers add to dial queue, no random peer to dial")
	}

	ka.status = kasDialSleeping
}
