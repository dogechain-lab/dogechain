package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/network/common"
	"github.com/dogechain-lab/dogechain/server/proto"
	"github.com/dogechain-lab/dogechain/txpool"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/libp2p/go-libp2p/core/peer"
	empty "google.golang.org/protobuf/types/known/emptypb"
)

type systemService struct {
	proto.UnimplementedSystemServer

	server *Server
}

// GetStatus returns the current system status, in the form of:
//
// Network: <chainID>
//
// Current: { Number: <blockNumber>; Hash: <headerHash> }
//
// P2PAddr: <libp2pAddress>
func (s *systemService) GetStatus(ctx context.Context, req *empty.Empty) (*proto.ServerStatus, error) {
	header := s.server.blockchain.Header()

	status := &proto.ServerStatus{
		Network: int64(s.server.chain.Params.ChainID),
		Current: &proto.ServerStatus_Block{
			Number: int64(header.Number),
			Hash:   header.Hash.String(),
		},
		P2PAddr: common.AddrInfoToString(s.server.network.AddrInfo()),
	}

	return status, nil
}

// Subscribe implements the blockchain event subscription service
func (s *systemService) Subscribe(req *empty.Empty, stream proto.System_SubscribeServer) error {
	sub := s.server.blockchain.SubscribeEvents()
	defer sub.Unsubscribe()

	for {
		if sub.IsClosed() {
			break
		}

		blockEvent, ok := <-sub.GetEvent()
		if blockEvent == nil || !ok {
			continue
		}

		pEvent := &proto.BlockchainEvent{
			Added:   []*proto.BlockchainEvent_Header{},
			Removed: []*proto.BlockchainEvent_Header{},
		}

		for _, h := range blockEvent.NewChain {
			pEvent.Added = append(
				pEvent.Added,
				&proto.BlockchainEvent_Header{Hash: h.Hash.String(), Number: int64(h.Number)},
			)
		}

		for _, h := range blockEvent.OldChain {
			pEvent.Removed = append(
				pEvent.Removed,
				&proto.BlockchainEvent_Header{Hash: h.Hash.String(), Number: int64(h.Number)},
			)
		}

		err := stream.Send(pEvent)

		if err != nil {
			s.server.logger.Error("error sending blockchain event", "error", err)

			break
		}
	}

	return nil
}

// PeersAdd implements the 'peers add' operator service
func (s *systemService) PeersAdd(_ context.Context, req *proto.PeersAddRequest) (*proto.PeersAddResponse, error) {
	if joinErr := s.server.JoinPeer(req.Id, req.Static); joinErr != nil {
		return &proto.PeersAddResponse{
			Message: "Unable to successfully add peer",
		}, joinErr
	}

	return &proto.PeersAddResponse{
		Message: "Peer address marked ready for dialing",
	}, nil
}

// PeersStatus implements the 'peers status' operator service
func (s *systemService) PeersStatus(ctx context.Context, req *proto.PeersStatusRequest) (*proto.Peer, error) {
	peerID, err := peer.Decode(req.Id)
	if err != nil {
		return nil, err
	}

	peer, err := s.getPeer(peerID)
	if err != nil {
		return nil, err
	}

	return peer, nil
}

// getPeer returns a specific proto.Peer using the peer ID
func (s *systemService) getPeer(id peer.ID) (*proto.Peer, error) {
	protocols, err := s.server.network.GetProtocols(id)
	if err != nil {
		return nil, err
	}

	info := s.server.network.GetPeerInfo(id)

	addrs := []string{}
	for _, addr := range info.Addrs {
		addrs = append(addrs, addr.String())
	}

	peer := &proto.Peer{
		Id:        id.String(),
		Protocols: protocols,
		Addrs:     addrs,
	}

	return peer, nil
}

// PeersList implements the 'peers list' operator service
func (s *systemService) PeersList(
	ctx context.Context,
	req *empty.Empty,
) (*proto.PeersListResponse, error) {
	resp := &proto.PeersListResponse{
		Peers: []*proto.Peer{},
	}

	peers := s.server.network.Peers()
	for _, p := range peers {
		peer, err := s.getPeer(p.Info.ID)
		if err != nil {
			return nil, err
		}

		resp.Peers = append(resp.Peers, peer)
	}

	return resp, nil
}

// BlockByNumber implements the BlockByNumber operator service
func (s *systemService) BlockByNumber(
	ctx context.Context,
	req *proto.BlockByNumberRequest,
) (*proto.BlockResponse, error) {
	block, ok := s.server.blockchain.GetBlockByNumber(req.Number, true)
	if !ok {
		return nil, fmt.Errorf("block #%d not found", req.Number)
	}

	return &proto.BlockResponse{
		Data: block.MarshalRLP(),
	}, nil
}

func (s *systemService) Export(req *proto.ExportRequest, stream proto.System_ExportServer) error {
	var (
		from uint64 = 0
		to   *uint64
	)

	if req.From != from {
		from = req.From
	}

	if req.To != 0 {
		if from >= req.To {
			return errors.New("to must be greater than from")
		}

		to = &req.To
	}

	canLoop := func(i uint64) bool {
		if to == nil {
			current := s.server.blockchain.Header()

			return current != nil && i <= current.Number
		} else {
			return i <= *to
		}
	}

	writer := newBlockStreamWriter(stream, s.server.blockchain, defaultMaxGRPCPayloadSize)
	i := from

	for canLoop(i) {
		block, ok := s.server.blockchain.GetBlockByNumber(i, true)
		if !ok {
			break
		}

		if err := writer.appendBlock(block); err != nil {
			return err
		}

		i++
	}

	if err := writer.flush(); err != nil {
		return err
	}

	return nil
}

func (s *systemService) WhitelistAddList(
	ctx context.Context,
	req *proto.WhitelistAddListRequest,
) (*proto.WhitelistAddListResponse, error) {
	count := s.server.txpool.AddWhitelistContracts(req.Contracts)

	return &proto.WhitelistAddListResponse{
		Count:   int64(count),
		Message: "OK",
	}, nil
}

func (s *systemService) WhitelistDeleteList(
	ctx context.Context,
	req *proto.WhitelistDeleteListRequest,
) (*proto.WhitelistDeleteListResponse, error) {
	count := s.server.txpool.DeleteWhitelistContracts(req.Contracts)

	return &proto.WhitelistDeleteListResponse{
		Count:   int64(count),
		Message: "OK",
	}, nil
}

func (s *systemService) DDOSContractList(
	ctx context.Context,
	req *empty.Empty,
) (*proto.DDOSContractListResponse, error) {
	rsp := new(proto.DDOSContractListResponse)

	rsp.Blacklist = make(map[string]int64)
	rsp.Whitelist = make(map[string]int64)

	ret := s.server.txpool.GetDDosContractList()

	bl := ret[txpool.DDosBlackList]
	for addr, count := range bl {
		rsp.Blacklist[addr.String()] = int64(count)
	}

	wl := ret[txpool.DDosWhiteList]
	for addr, count := range wl {
		rsp.Whitelist[addr.String()] = int64(count)
	}

	return rsp, nil
}

const (
	defaultMaxGRPCPayloadSize uint64 = 4 * 1024 * 1024 // 4MB
)

type blockStreamWriter struct {
	buf         bytes.Buffer
	blockchain  *blockchain.Blockchain
	stream      proto.System_ExportServer
	maxPayload  uint64
	pendingFrom *uint64 // first block height in buffer
	pendingTo   *uint64 // last block height in buffer
}

func newBlockStreamWriter(
	stream proto.System_ExportServer,
	blockchain *blockchain.Blockchain,
	maxPayload uint64,
) *blockStreamWriter {
	return &blockStreamWriter{
		buf:        *bytes.NewBuffer(make([]byte, 0, maxPayload)),
		blockchain: blockchain,
		stream:     stream,
		maxPayload: maxPayload,
	}
}

func (w *blockStreamWriter) appendBlock(b *types.Block) error {
	data := b.MarshalRLP()

	if uint64(w.buf.Len()+len(data)) >= w.maxPayload {
		// send buffered data to client first
		if err := w.flush(); err != nil {
			return err
		}
	}

	w.buf.Write(data)

	n := b.Number()
	if w.pendingFrom == nil {
		w.pendingFrom = &n
	}

	w.pendingTo = &n

	return nil
}

func (w *blockStreamWriter) flush() error {
	// nothing happens in case of empty buffer
	if w.buf.Len() == 0 {
		return nil
	}

	if w.pendingFrom == nil || w.pendingTo == nil {
		// should not reach
		return errors.New("pendingFrom or pendingTo is nil")
	}

	err := w.stream.Send(&proto.ExportEvent{
		From:   *w.pendingFrom,
		To:     *w.pendingTo,
		Latest: w.blockchain.Header().Number,
		Data:   w.buf.Bytes(),
	})

	if err != nil {
		return err
	}

	w.reset()

	return nil
}

func (w *blockStreamWriter) reset() {
	w.buf.Reset()
	w.pendingFrom = nil
	w.pendingTo = nil
}
