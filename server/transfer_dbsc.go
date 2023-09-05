package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"math/rand"
	"time"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/types"

	helperCommon "github.com/dogechain-lab/dogechain/helper/common"

	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/secrets"

	dbscCommon "github.com/ethereum/go-ethereum/common"
	dbscForkId "github.com/ethereum/go-ethereum/core/forkid"
	dbscTypes "github.com/ethereum/go-ethereum/core/types"
	dbscCrypto "github.com/ethereum/go-ethereum/crypto"
	dbscLog "github.com/ethereum/go-ethereum/log"
	dbscP2p "github.com/ethereum/go-ethereum/p2p"
	dbscNat "github.com/ethereum/go-ethereum/p2p/nat"
	dbscRlp "github.com/ethereum/go-ethereum/rlp"

	dbscEthProto "github.com/ethereum/go-ethereum/eth/protocols/eth"
)

const (
	softResponseLimit = 2 * 1024 * 1024
	maxHeadersServe   = 1024
	maxReceiptsServe  = 1024

	// maxBodiesServe is the maximum number of block bodies to serve. This number
	// is mostly there to limit the number of disk lookups. With 24KB block sizes
	// nowadays, the practical limit will always be softResponseLimit.
	maxBodiesServe = 1024

	// maxMessageSize is the maximum cap on the size of a protocol message.
	maxMessageSize = 10 * 1024 * 1024
)

var (
	errMsgTooLarge = errors.New("message too long")
	errDecode      = errors.New("invalid message")
)

var txHashCache *fastcache.Cache

func init() {
	rand.Seed(time.Now().Unix())

	txHashCache = fastcache.New(64 * 1024 * 1024) // cache size 64Mib
}

func createDbscServer(config *network.Config) (*dbscP2p.Server, error) {
	maxPeers := config.MaxInboundPeers + config.MaxOutboundPeers

	prikeyBytes, err := config.SecretsManager.GetSecret(secrets.NetworkKey)
	if err != nil {
		return nil, err
	}

	libp2pKey, err := network.ParseLibp2pKey(prikeyBytes)
	if err != nil {
		return nil, err
	}

	rawPri, err := libp2pKey.Raw()
	if err != nil {
		return nil, err
	}

	prikey, err := dbscCrypto.ToECDSA(rawPri)
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
			ListenAddr:  fmt.Sprintf(":%d", config.Addr.Port+100),
			Logger:      dbscLog.Root(),
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
				ctx, cancel := context.WithCancel(s.ctx)
				defer cancel()

				go s.broadcastBlocksToDbscPeer(ctx, p, rw)

				for {
					if err := s.handleDbscMessage(p, rw); err != nil {
						if errors.Is(err, io.EOF) {
							break
						}

						s.logger.Error("Message handling failed in `eth`", "err", err)

						return err
					}
				}

				return nil
			},
			NodeInfo:       nil,
			PeerInfo:       nil,
			DialCandidates: nil,
		}
	}

	return protocols
}

func (s *Server) broadcastBlocksToDbscPeer(ctx context.Context, p *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) {
	newBlockSub := s.blockchain.SubscribeEvents()
	defer newBlockSub.Unsubscribe()

	for {
		if newBlockSub.IsClosed() {
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
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

		request[0].Hash = hashToDbscHash(hash)
		request[0].Number = number

		err := dbscP2p.Send(rw, dbscEthProto.NewBlockHashesMsg, request)
		if err != nil {
			s.logger.Error("Failed to send NewBlockHashes to dbsc peer", "peer", p, "err", err)
		}
	}
}

func hashToDbscHash(hash types.Hash) dbscCommon.Hash {
	return dbscCommon.BytesToHash(hash.Bytes())
}

func hashsToDbscHashs(hashs []types.Hash) []dbscCommon.Hash {
	dbscHashs := make([]dbscCommon.Hash, len(hashs))

	for i, hash := range hashs {
		dbscHashs[i] = hashToDbscHash(hash)
	}

	return dbscHashs
}

func dbscHashToHash(hash dbscCommon.Hash) types.Hash {
	return types.BytesToHash(hash.Bytes())
}

func addressToDbscAddress(address types.Address) dbscCommon.Address {
	return dbscCommon.BytesToAddress(address.Bytes())
}

func txToDbscTx(tx *types.Transaction) *dbscTypes.Transaction {
	var toAddress *dbscCommon.Address = nil

	if tx.To != nil {
		add := addressToDbscAddress(*tx.To)
		toAddress = &add
	}

	return dbscTypes.NewTx(&dbscTypes.LegacyTx{
		Nonce:    tx.Nonce,
		GasPrice: tx.GasPrice,
		Gas:      tx.Gas,
		To:       toAddress,
		Value:    tx.Value,
		Data:     tx.Input,
		V:        tx.V,
		R:        tx.R,
		S:        tx.S,
	})
}

func dbscTxToTx(signer dbscTypes.Signer, tx *dbscTypes.Transaction) (*types.Transaction, error) {
	var toAddress *types.Address = nil

	if tx.To() != nil {
		add := types.Address(*tx.To())
		toAddress = &add
	}

	v, r, s := tx.RawSignatureValues()
	send, err := dbscTypes.Sender(signer, tx)

	if err != nil {
		return nil, err
	}

	return &types.Transaction{
		Nonce:        tx.Nonce(),
		GasPrice:     tx.GasPrice(),
		Gas:          tx.Gas(),
		To:           toAddress,
		Value:        tx.Value(),
		Input:        tx.Data(),
		V:            v,
		R:            r,
		S:            s,
		From:         types.Address(send),
		ReceivedTime: tx.Time(),
	}, nil
}

func txsToDbscTxs(txs []*types.Transaction) []*dbscTypes.Transaction {
	result := make([]*dbscTypes.Transaction, 0, len(txs))

	for _, tx := range txs {
		result = append(result, txToDbscTx(tx))
	}

	return result
}

func blockToDbscBlockRlp(block *types.Block) (dbscRlp.RawValue, error) {
	dbscUncles := make([]*dbscTypes.Header, 0, len(block.Uncles))

	for _, uncle := range block.Uncles {
		dbscUncles = append(dbscUncles, headerToDbscHeader(uncle))
	}

	extBlk := struct {
		Txs    []*dbscTypes.Transaction
		Uncles []*dbscTypes.Header
	}{
		Txs:    txsToDbscTxs(block.Transactions),
		Uncles: dbscUncles,
	}

	return dbscRlp.EncodeToBytes(extBlk)
}

func headerToDbscHeader(header *types.Header) *dbscTypes.Header {
	return &dbscTypes.Header{
		ParentHash:  hashToDbscHash(header.ParentHash),
		UncleHash:   hashToDbscHash(header.Sha3Uncles),
		Coinbase:    addressToDbscAddress(header.Miner),
		Root:        hashToDbscHash(header.StateRoot),
		TxHash:      hashToDbscHash(header.TxRoot),
		ReceiptHash: hashToDbscHash(header.ReceiptsRoot),
		Bloom:       dbscTypes.BytesToBloom(header.LogsBloom[:]),
		Difficulty:  new(big.Int).SetUint64(header.Difficulty),
		Number:      new(big.Int).SetUint64(header.Number),
		GasLimit:    header.GasLimit,
		GasUsed:     header.GasUsed,
		Time:        header.Timestamp,
		Extra:       header.ExtraData,
		MixDigest:   hashToDbscHash(header.MixHash),
		Nonce:       dbscTypes.BlockNonce(header.Nonce),
	}
}

func logToDbscLog(log *types.Log) *dbscTypes.Log {
	return &dbscTypes.Log{
		Address: addressToDbscAddress(log.Address),
		Topics:  hashsToDbscHashs(log.Topics),
		Data:    log.Data,
	}
}

func logsToDbscLogs(logs []*types.Log) []*dbscTypes.Log {
	result := make([]*dbscTypes.Log, 0, len(logs))

	for _, log := range logs {
		result = append(result, logToDbscLog(log))
	}

	return result
}

func receiptToDbscReceipt(receipt *types.Receipt) *dbscTypes.Receipt {
	var contractAddress dbscCommon.Address
	if receipt.ContractAddress == nil {
		contractAddress = dbscCommon.Address{}
	} else {
		contractAddress = addressToDbscAddress(*receipt.ContractAddress)
	}

	postState := []byte{}
	status := dbscTypes.ReceiptStatusFailed

	if receipt.Status != nil {
		switch *receipt.Status {
		case types.ReceiptSuccess:
			status = dbscTypes.ReceiptStatusSuccessful
		case types.ReceiptFailed:
			status = dbscTypes.ReceiptStatusFailed
		}
	} else {
		postState = receipt.Root.Bytes()
	}

	return &dbscTypes.Receipt{
		Type:              dbscTypes.LegacyTxType,
		PostState:         postState,
		Status:            status,
		CumulativeGasUsed: receipt.CumulativeGasUsed,
		Bloom:             dbscTypes.BytesToBloom(receipt.LogsBloom[:]),
		Logs:              logsToDbscLogs(receipt.Logs),
		TxHash:            hashToDbscHash(receipt.TxHash),
		ContractAddress:   contractAddress,
		GasUsed:           receipt.GasUsed,
	}
}

func receiptsToDbscReceipts(receipts []*types.Receipt) []*dbscTypes.Receipt {
	result := make([]*dbscTypes.Receipt, 0, len(receipts))

	for _, receipt := range receipts {
		result = append(result, receiptToDbscReceipt(receipt))
	}

	return result
}

type msgHandler func(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error
type Decoder interface {
	Decode(val interface{}) error
	Time() time.Time
}

var eth66Handler = map[uint64]msgHandler{
	dbscEthProto.NewBlockHashesMsg:  handleNewBlockhashes,
	dbscEthProto.NewBlockMsg:        handleNewBlock,
	dbscEthProto.GetBlockHeadersMsg: handleDbscGetBlockHeaders,
	dbscEthProto.GetBlockBodiesMsg:  handleDbscGetBlockBodies,
	dbscEthProto.GetReceiptsMsg:     handleDbscGetReceipts,
	dbscEthProto.StatusMsg:          handleDbscStatus,
	// receive dbsc network transaction message
	dbscEthProto.TransactionsMsg:               handleDbscTransactions,
	dbscEthProto.PooledTransactionsMsg:         handleDbscPooledTransactions,
	dbscEthProto.NewPooledTransactionHashesMsg: handleDbscNewPooledTransactionHashes,
}

func replyBlockHeadersRLP(rw dbscP2p.MsgReadWriter, id uint64, headers []dbscRlp.RawValue) error {
	return dbscP2p.Send(rw, dbscEthProto.BlockHeadersMsg, &dbscEthProto.BlockHeadersRLPPacket66{
		RequestId:             id,
		BlockHeadersRLPPacket: headers,
	})
}

func replyBlockBodiesRLP(rw dbscP2p.MsgReadWriter, id uint64, bodies []dbscRlp.RawValue) error {
	// Not packed into BlockBodiesPacket to avoid RLP decoding
	return dbscP2p.Send(rw, dbscEthProto.BlockBodiesMsg, &dbscEthProto.BlockBodiesRLPPacket66{
		RequestId:            id,
		BlockBodiesRLPPacket: bodies,
	})
}

func replyReceiptsRLP(rw dbscP2p.MsgReadWriter, id uint64, receipts []dbscRlp.RawValue) error {
	return dbscP2p.Send(rw, dbscEthProto.ReceiptsMsg, &dbscEthProto.ReceiptsRLPPacket66{
		RequestId:         id,
		ReceiptsRLPPacket: receipts,
	})
}

func serviceGetBlockHeadersQuery(
	s *Server,
	query *dbscEthProto.GetBlockHeadersPacket,
	peer *dbscP2p.Peer,
	rw dbscP2p.MsgReadWriter,
) []dbscRlp.RawValue {
	if query.Skip == 0 {
		// The fast path
		return serviceContiguousBlockHeaderQuery(s, query, peer, rw)
	} else {
		return serviceNonContiguousBlockHeaderQuery(s, query, peer, rw)
	}
}

func serviceNonContiguousBlockHeaderQuery(
	s *Server,
	query *dbscEthProto.GetBlockHeadersPacket,
	peer *dbscP2p.Peer,
	rw dbscP2p.MsgReadWriter,
) []dbscRlp.RawValue {
	chain := s.blockchain
	hashMode := query.Origin.Hash != (dbscCommon.Hash{})
	first := true
	maxNonCanonical := uint64(100)

	// Gather headers until the fetch or network limits is reached
	var (
		bytes   dbscCommon.StorageSize
		headers []dbscRlp.RawValue
		unknown bool
		lookups int
	)

	for !unknown &&
		len(headers) < int(query.Amount) &&
		bytes < softResponseLimit &&
		len(headers) < maxHeadersServe &&
		lookups < 2*maxHeadersServe {
		// Retrieve the next header satisfying the query
		var origin *dbscTypes.Header
		lookups++

		if hashMode {
			if first {
				first = false

				header, ok := chain.GetHeaderByHash(dbscHashToHash(query.Origin.Hash))
				if ok {
					origin = headerToDbscHeader(header)
				}

				// origin = chain.GetHeaderByHash(query.Origin.Hash)
				if origin != nil {
					query.Origin.Number = origin.Number.Uint64()
				}
			} else {
				header, ok := chain.GetHeader(dbscHashToHash(query.Origin.Hash), query.Origin.Number)
				if ok {
					origin = headerToDbscHeader(header)
				}
			}
		} else {
			header, ok := chain.GetHeaderByNumber(query.Origin.Number)
			if ok {
				origin = headerToDbscHeader(header)
			}
		}

		if origin == nil {
			break
		}

		if rlpData, err := dbscRlp.EncodeToBytes(origin); err != nil {
			s.logger.Error("Unable to decode our own headers", "err", err)
		} else {
			headers = append(headers, dbscRlp.RawValue(rlpData))
			bytes += dbscCommon.StorageSize(len(rlpData))
		}

		// Advance to the next header of the query
		switch {
		case hashMode && query.Reverse:
			// Hash based traversal towards the genesis block
			ancestor := query.Skip + 1
			if ancestor == 0 {
				unknown = true
			} else {
				h, n := chain.GetAncestor(
					dbscHashToHash(query.Origin.Hash),
					query.Origin.Number,
					ancestor,
					&maxNonCanonical,
				)

				query.Origin.Hash = hashToDbscHash(h)
				query.Origin.Number = n

				unknown = (query.Origin.Hash == dbscCommon.Hash{})
			}
		case hashMode && !query.Reverse:
			// Hash based traversal towards the leaf block
			var (
				current = origin.Number.Uint64()
				next    = current + query.Skip + 1
			)

			if next <= current {
				infos, _ := json.MarshalIndent(peer.Info(), "", "  ")
				peer.Log().Warn(
					"GetBlockHeaders skip overflow attack",
					"current", current,
					"skip", query.Skip,
					"next", next,
					"attacker", infos,
				)

				unknown = true
			} else {
				if header, ok := chain.GetHeaderByNumber(next); header != nil && ok {
					nextHash := header.Hash
					expOldHash, _ := chain.GetAncestor(nextHash, next, query.Skip+1, &maxNonCanonical)
					dbscExpOldHash := hashToDbscHash(expOldHash)

					if dbscExpOldHash == query.Origin.Hash {
						query.Origin.Hash, query.Origin.Number = hashToDbscHash(nextHash), next
					} else {
						unknown = true
					}
				} else {
					unknown = true
				}
			}
		case query.Reverse:
			// Number based traversal towards the genesis block
			if query.Origin.Number >= query.Skip+1 {
				query.Origin.Number -= query.Skip + 1
			} else {
				unknown = true
			}

		case !query.Reverse:
			// Number based traversal towards the leaf block
			query.Origin.Number += query.Skip + 1
		}
	}

	return headers
}

func handleNewBlockhashes(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	// A batch of new block announcements just arrived
	ann := new(dbscEthProto.NewBlockHashesPacket)
	if err := msg.Decode(ann); err != nil {
		return fmt.Errorf("%w: message %v: %w", errDecode, msg, err)
	}

	return nil
}

func handleNewBlock(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	// Retrieve and decode the propagated block
	ann := new(dbscEthProto.NewBlockPacket)
	if err := msg.Decode(ann); err != nil {
		return fmt.Errorf("%w: message %v: %w", errDecode, msg, err)
	}

	return nil
}

func handleDbscGetBlockHeaders(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	// Decode the complex header query
	var query dbscEthProto.GetBlockHeadersPacket66
	if err := msg.Decode(&query); err != nil {
		return fmt.Errorf("%w: message %v: %s", errDecode, msg, err.Error())
	}

	response := serviceGetBlockHeadersQuery(s, query.GetBlockHeadersPacket, peer, rw)

	return replyBlockHeadersRLP(rw, query.RequestId, response)
}

func serviceContiguousBlockHeaderQuery(
	s *Server,
	query *dbscEthProto.GetBlockHeadersPacket,
	peer *dbscP2p.Peer,
	rw dbscP2p.MsgReadWriter,
) []dbscRlp.RawValue {
	chain := s.blockchain
	count := query.Amount

	if count > maxHeadersServe {
		count = maxHeadersServe
	}

	if query.Origin.Hash == (dbscCommon.Hash{}) {
		// Number mode, just return the canon chain segment. The backend
		// delivers in [N, N-1, N-2..] descending order, so we need to
		// accommodate for that.
		from := query.Origin.Number
		if !query.Reverse {
			from = from + count - 1
		}

		headers := chain.GetHeadersFrom(from, count)

		if !query.Reverse {
			for i, j := 0, len(headers)-1; i < j; i, j = i+1, j-1 {
				headers[i], headers[j] = headers[j], headers[i]
			}
		}

		var rlpHeaders []dbscRlp.RawValue

		for _, header := range headers {
			rlpData, _ := dbscRlp.EncodeToBytes(headerToDbscHeader(header))
			rlpHeaders = append(rlpHeaders, rlpData)
		}

		return rlpHeaders
	}

	// Hash mode.
	var (
		headers []dbscRlp.RawValue
		hash    = dbscHashToHash(query.Origin.Hash)
	)

	header, ok := chain.GetHeaderByHash(hash)

	if header != nil && ok {
		rlpData, _ := dbscRlp.EncodeToBytes(headerToDbscHeader(header))
		headers = append(headers, rlpData)
	} else {
		// We don't even have the origin header
		return headers
	}

	num := header.Number

	if !query.Reverse {
		// Theoretically, we are tasked to deliver header by hash H, and onwards.
		// However, if H is not canon, we will be unable to deliver any descendants of
		// H.
		if canonHash, ok := chain.GetCanonicalHash(num); canonHash != hash && ok {
			// Not canon, we can't deliver descendants
			return headers
		}

		descendants := chain.GetHeadersFrom(num+count-1, count-1)

		for i, j := 0, len(descendants)-1; i < j; i, j = i+1, j-1 {
			descendants[i], descendants[j] = descendants[j], descendants[i]
		}

		for _, header := range descendants {
			rlpData, _ := dbscRlp.EncodeToBytes(headerToDbscHeader(header))
			headers = append(headers, rlpData)
		}

		return headers
	}

	{ // Last mode: deliver ancestors of H
		for i := uint64(1); header != nil && i < count; i++ {
			header, ok = chain.GetHeaderByHash(header.ParentHash)
			if header == nil || !ok {
				break
			}

			rlpData, _ := dbscRlp.EncodeToBytes(headerToDbscHeader(header))
			headers = append(headers, rlpData)
		}

		return headers
	}
}

func handleDbscGetBlockBodies(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	// Decode the block body retrieval message
	var query dbscEthProto.GetBlockBodiesPacket66
	if err := msg.Decode(&query); err != nil {
		return fmt.Errorf("%w: message %v: %s", errDecode, msg, err.Error())
	}

	response, err := serviceGetBlockBodiesQuery(s, query.GetBlockBodiesPacket, peer, rw)
	if err != nil {
		return err
	}

	return replyBlockBodiesRLP(rw, query.RequestId, response)
}

func serviceGetBlockBodiesQuery(
	s *Server,
	query dbscEthProto.GetBlockBodiesPacket,
	peer *dbscP2p.Peer,
	rw dbscP2p.MsgReadWriter,
) ([]dbscRlp.RawValue, error) {
	chain := s.blockchain

	// Gather blocks until the fetch or network limits is reached
	var (
		bytes  int
		bodies []dbscRlp.RawValue
	)

	for lookups, hash := range query {
		if bytes >= softResponseLimit ||
			len(bodies) >= maxBodiesServe ||
			lookups >= 2*maxBodiesServe {
			break
		}

		if block, ok := chain.GetBlockByHash(dbscHashToHash(hash), true); ok {
			rlpData, err := blockToDbscBlockRlp(block)

			if err != nil {
				return nil, err
			}

			bodies = append(bodies, rlpData)
			bytes += len(rlpData)
		}
	}

	return bodies, nil
}

func handleDbscGetReceipts(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	// Decode the block receipts retrieval message
	var query dbscEthProto.GetReceiptsPacket66
	if err := msg.Decode(&query); err != nil {
		return fmt.Errorf("%w: message %v: %s", errDecode, msg, err.Error())
	}

	response, err := serviceGetReceiptsQuery(s, query.GetReceiptsPacket, peer, rw)
	if err != nil {
		return err
	}

	return replyReceiptsRLP(rw, query.RequestId, response)
}

func serviceGetReceiptsQuery(
	s *Server,
	query dbscEthProto.GetReceiptsPacket,
	peer *dbscP2p.Peer,
	rw dbscP2p.MsgReadWriter,
) ([]dbscRlp.RawValue, error) {
	chain := s.blockchain

	var (
		bytes    int
		receipts []dbscRlp.RawValue
	)

	for lookups, hash := range query {
		if bytes >= softResponseLimit || len(receipts) >= maxReceiptsServe ||
			lookups >= 2*maxReceiptsServe {
			break
		}

		hash := dbscHashToHash(hash)

		// Retrieve the requested block's receipts
		results, err := chain.GetReceiptsByHash(hash)
		if err != nil {
			s.logger.Error("Failed to retrieve receipts", "err", err)

			continue
		}

		if results == nil {
			header, nofind := chain.GetHeaderByHash(hash)

			if nofind || header == nil || header.ReceiptsRoot != types.EmptyRootHash {
				continue
			}
		}

		// If known, encode and queue for response packet
		if encoded, err := dbscRlp.EncodeToBytes(receiptsToDbscReceipts(results)); err != nil {
			s.logger.Error("Failed to encode receipt", "err", err)
		} else {
			receipts = append(receipts, encoded)
			bytes += len(encoded)
		}
	}

	return receipts, nil
}

func handleDbscStatus(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	currentHeader := s.blockchain.Header()
	diff, _ := s.blockchain.GetTD(currentHeader.Hash)
	genesisHash := hashToDbscHash(s.blockchain.Genesis())

	forkID := dbscForkId.NewID(
		s.config.DbscChainConfig,
		genesisHash,
		currentHeader.Number,
	)

	dbscP2p.Send(rw, dbscEthProto.StatusMsg, &dbscEthProto.StatusPacket{
		ProtocolVersion: dbscEthProto.ETH66,
		NetworkID:       s.config.DbscChainConfig.ChainID.Uint64(),
		TD:              diff,
		Head:            hashToDbscHash(currentHeader.Hash),
		Genesis:         genesisHash,
		ForkID:          forkID,
	})

	return nil
}

func handleDbscTransactions(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	var txs dbscEthProto.TransactionsPacket

	currentHeader := s.blockchain.Header()
	number := new(big.Int).SetUint64(currentHeader.Number)

	if err := msg.Decode(&txs); err != nil {
		return fmt.Errorf("%w: message %v: %s", errDecode, msg, err.Error())
	}

	for i, tx := range txs {
		// Validate and mark the remote transaction
		if tx == nil {
			return fmt.Errorf("%w: transaction %d is nil", errDecode, i)
		}

		// Skip non-legacy transactions
		if tx.Type() != dbscTypes.LegacyTxType {
			continue
		}

		signer := dbscTypes.MakeSigner(
			s.config.DbscChainConfig,
			number,
		)

		tx, err := dbscTxToTx(signer, tx)
		if err != nil {
			s.logger.Error("Failed to convert transaction", "err", err)
		}

		s.txpool.AddTx(tx)
	}

	return nil
}

func handleDbscPooledTransactions(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	if s.txpool == nil {
		return nil
	}

	var txs dbscEthProto.PooledTransactionsPacket66
	if err := msg.Decode(&txs); err != nil {
		return fmt.Errorf("%w: message %v: %w", errDecode, msg, err)
	}

	if len(txs.PooledTransactionsPacket) <= 0 {
		return nil
	}

	currentHeader := s.blockchain.Header()
	number := new(big.Int).SetUint64(currentHeader.Number)

	for i, tx := range txs.PooledTransactionsPacket {
		// Validate and mark the remote transaction
		if tx == nil {
			return fmt.Errorf("%w: transaction %d is nil", errDecode, i)
		}

		// Skip non-legacy transactions
		if tx.Type() != dbscTypes.LegacyTxType {
			continue
		}

		signer := dbscTypes.MakeSigner(
			s.config.DbscChainConfig,
			number,
		)

		tx, err := dbscTxToTx(signer, tx)
		if err != nil {
			s.logger.Error("Failed to convert transaction", "err", err)
		}

		s.txpool.AddTx(tx)
	}

	return nil
}

func handleDbscNewPooledTransactionHashes(s *Server, msg Decoder, peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	if s.txpool == nil {
		return nil
	}

	ann := new(dbscEthProto.NewPooledTransactionHashesPacket)
	if err := msg.Decode(ann); err != nil {
		return fmt.Errorf("%w: message %v: %w", errDecode, msg, err)
	}

	if ann == nil || len(*ann) <= 0 {
		return nil
	}

	pendingTxHashes := make([]types.Hash, 0, len(*ann))
	// check tx hash cache to avoid duplicate tx
	for _, hash := range *ann {
		hash := dbscHashToHash(hash)

		if txHashCache.Has(hash.Bytes()) {
			continue
		}

		pendingTxHashes = append(pendingTxHashes, hash)
		txHashCache.Set(hash.Bytes(), []byte{0x01})
	}

	if len(pendingTxHashes) <= 0 {
		return nil
	}

	// fetch txs from txpool
	dbscP2p.Send(rw, dbscEthProto.GetPooledTransactionsMsg, &dbscEthProto.GetPooledTransactionsPacket{})
	//nolint:gosec
	id := rand.Uint64()

	return dbscP2p.Send(rw, dbscEthProto.GetPooledTransactionsMsg, &dbscEthProto.GetPooledTransactionsPacket66{
		RequestId:                   id,
		GetPooledTransactionsPacket: hashsToDbscHashs(pendingTxHashes),
	})
}

func (s *Server) handleDbscMessage(peer *dbscP2p.Peer, rw dbscP2p.MsgReadWriter) error {
	msg, err := rw.ReadMsg()
	if err != nil {
		return err
	}

	if msg.Size > maxMessageSize {
		return fmt.Errorf("%w: %v > %v", errMsgTooLarge, msg.Size, maxMessageSize)
	}
	defer msg.Discard()

	if handler, ok := eth66Handler[msg.Code]; ok && handler != nil {
		return handler(s, msg, peer, rw)
	}

	s.logger.Error("no implementation for message", "code", msg.Code)

	return nil
}
