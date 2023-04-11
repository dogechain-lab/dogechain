package server

import (
	"errors"
	"fmt"
	"time"

	"github.com/dogechain-lab/dogechain/blockchain"
	helperCommon "github.com/dogechain-lab/dogechain/helper/common"

	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/secrets"

	dbscCommon "github.com/ethereum/go-ethereum/common"
	dbscForkId "github.com/ethereum/go-ethereum/core/forkid"
	dbscCrypto "github.com/ethereum/go-ethereum/crypto"
	dbscP2p "github.com/ethereum/go-ethereum/p2p"
	dbscNat "github.com/ethereum/go-ethereum/p2p/nat"

	dbscEthProto "github.com/ethereum/go-ethereum/eth/protocols/eth"
)

func createDbscServer(config *network.Config) (*dbscP2p.Server, error) {
	maxPeers := config.MaxInboundPeers + config.MaxOutboundPeers

	prikeyBin, err := config.SecretsManager.GetSecret(secrets.NetworkKey)
	if err != nil {
		return nil, err
	}

	prikey, err := dbscCrypto.ToECDSA(prikeyBin)
	if err != nil {
		return nil, err
	}

	return &dbscP2p.Server{
		Config: dbscP2p.Config{
			PrivateKey:  prikey,
			MaxPeers:    helperCommon.ClampInt64ToInt(maxPeers),
			NAT:         dbscNat.Any(),
			NoDial:      true,
			NoDiscovery: true,
		},
	}, nil
}

// makeBscProtocols creates the P2P protocols used by the BSC service.
func makeBscProtocols(s *Server) []dbscP2p.Protocol {
	protocolVersions := []uint{dbscEthProto.ETH66}
	protocols := make([]dbscP2p.Protocol, len(protocolVersions))
	protocolLengths := map[uint]uint64{dbscEthProto.ETH67: 18, dbscEthProto.ETH66: 17}

	for i, version := range protocolVersions {
		version := version // Closure

		protocols[i] = dbscP2p.Protocol{
			Name:    dbscEthProto.ProtocolName,
			Version: version,
			Length:  protocolLengths[version],
			Run: func(p *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
				go s.broadcastBlocksToDbscPeer(p, rw)

				for {
					if err := s.handleDbscMessage(p, rw); err != nil {
						s.logger.Error("Message handling failed in `eth`", "err", err)

						return err
					}
				}
			},
			NodeInfo:       nil,
			PeerInfo:       nil,
			DialCandidates: nil,
		}
	}

	return protocols
}

func (s *Server) broadcastBlocksToDbscPeer(p *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) {
	newBlockSub := s.blockchain.SubscribeEvents()
	defer newBlockSub.Unsubscribe()

	for {
		if newBlockSub.IsClosed() {
			return
		}

		e, ok := <-newBlockSub.GetEvent()
		if e == nil || !ok {
			return
		}

		if e.Type == blockchain.EventFork {
			continue
		}

		// this should not happen
		if len(e.NewChain) == 0 {
			continue
		}

		hash := e.NewChain[0].Hash
		number := e.NewChain[0].Number

		request := make(dbscEthProto.NewBlockHashesPacket, 1)

		request[0].Hash = (dbscCommon.Hash)(hash)
		request[0].Number = number

		err := dbscP2p.Send(rw, dbscEthProto.NewBlockHashesMsg, request)
		if err != nil {
			s.logger.Error("Failed to send NewBlockHashes to dbsc peer", "peer", p, "err", err)

			return
		}
	}
}

// maxMessageSize is the maximum cap on the size of a protocol message.
const maxMessageSize = 10 * 1024 * 1024

type msgHandler func(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error
type Decoder interface {
	Decode(val interface{}) error
	Time() time.Time
}

var eth66Handler = map[uint64]msgHandler{
	dbscEthProto.NewBlockHashesMsg:             handleDbscNewBlockhashes,
	dbscEthProto.NewBlockMsg:                   handleDbscNewBlock,
	dbscEthProto.TransactionsMsg:               handleDbscTransactions,
	dbscEthProto.NewPooledTransactionHashesMsg: handleDbscNewPooledTransactionHashes,
	dbscEthProto.GetBlockHeadersMsg:            handleDbscGetBlockHeaders,
	dbscEthProto.BlockHeadersMsg:               handleDbscBlockHeaders,
	dbscEthProto.GetBlockBodiesMsg:             handleDbscGetBlockBodies,
	dbscEthProto.BlockBodiesMsg:                handleDbscBlockBodies,
	dbscEthProto.GetNodeDataMsg:                handleDbscGetNodeData,
	dbscEthProto.NodeDataMsg:                   handleDbscNodeData,
	dbscEthProto.GetReceiptsMsg:                handleDbscGetReceipts,
	dbscEthProto.ReceiptsMsg:                   handleDbscReceipts,
	dbscEthProto.GetPooledTransactionsMsg:      handleDbscGetPooledTransactions,
	dbscEthProto.PooledTransactionsMsg:         handleDbscPooledTransactions,
	dbscEthProto.StatusMsg:                     handleDbscStatus,
}

func handleDbscNewBlockhashes(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	return nil
}

func handleDbscNewBlock(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	return nil
}

func handleDbscTransactions(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	return nil
}

func handleDbscNewPooledTransactionHashes(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	return nil
}

func handleDbscGetBlockHeaders(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	return nil
}

func handleDbscBlockHeaders(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	return nil
}

func handleDbscGetBlockBodies(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	return nil
}

func handleDbscBlockBodies(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	return nil
}

func handleDbscGetNodeData(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	return nil
}

func handleDbscNodeData(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	return nil
}

func handleDbscGetReceipts(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	return nil
}

func handleDbscReceipts(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	return nil
}

func handleDbscGetPooledTransactions(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	return nil
}

func handleDbscPooledTransactions(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	return nil
}

func handleDbscStatus(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	currentHeader := s.blockchain.Header()
	diff, _ := s.blockchain.GetTD(currentHeader.Hash)
	genesisHash := (dbscCommon.Hash)(s.blockchain.Genesis())

	forkID := dbscForkId.NewID(
		s.config.DbscChainConfig,
		genesisHash,
		currentHeader.Number,
	)

	dbscP2p.Send(rw, dbscEthProto.StatusMsg, &dbscEthProto.StatusPacket{
		ProtocolVersion: dbscEthProto.ETH66,
		NetworkID:       s.config.DbscChainConfig.ChainID.Uint64(),
		TD:              diff,
		Head:            (dbscCommon.Hash)(currentHeader.Hash),
		Genesis:         genesisHash,
		ForkID:          forkID,
	})

	return nil
}

var (
	errMsgTooLarge    = errors.New("message too long")
	errInvalidMsgCode = errors.New("invalid message code")
)

func (s *Server) handleDbscMessage(peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	msg, err := rw.ReadMsg()
	if err != nil {
		return err
	}

	if msg.Size > maxMessageSize {
		return fmt.Errorf("%w: %v > %v", errMsgTooLarge, msg.Size, maxMessageSize)
	}
	defer msg.Discard()

	if handler := eth66Handler[msg.Code]; handler != nil {
		return handler(s, msg, peer, rw)
	}

	return fmt.Errorf("%w: %v", errInvalidMsgCode, msg.Code)
}
