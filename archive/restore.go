package archive

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"runtime"

	"github.com/dogechain-lab/dogechain/helper/common"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	"github.com/klauspost/compress/zstd"
)

const (
	WriteBlockSource = "archive"
)

var zstdMagic = []byte{0x28, 0xb5, 0x2f, 0xfd} // zstd Magic number

type blockchainInterface interface {
	Genesis() types.Hash
	GetHeaderNumber() (uint64, bool)
	GetBlockByNumber(uint64, bool) (*types.Block, bool)
	GetHashByNumber(uint64) types.Hash
	WriteBlock(block *types.Block, source string) error
	VerifyFinalizedBlock(*types.Block) error
}

// RestoreChain reads blocks from the archive and write to the chain
func RestoreChain(log hclog.Logger, chain blockchainInterface, filePath string) error {
	fp, err := os.OpenFile(filePath, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer fp.Close()

	fbuf := bufio.NewReaderSize(fp, 8*1024*1024) // 8MB buffer

	// check whether the file is compressed
	fileMagic, err := fbuf.Peek(len(zstdMagic))
	if err != nil {
		return err
	}

	var readBuf io.Reader

	if bytes.Equal(fileMagic[:], zstdMagic[:]) {
		zstdReader, err := zstd.NewReader(fbuf)
		if err != nil {
			return err
		}
		defer zstdReader.Close()

		log.Info("archive is compressed with zstd")

		readBuf = zstdReader
	} else {
		readBuf = fbuf
	}

	blockStream := newBlockStream(readBuf)

	return importBlocks(log, chain, blockStream)
}

// import blocks scans all blocks from stream and write them to chain
func importBlocks(log hclog.Logger, chain blockchainInterface, blockStream *blockStream) error {
	shutdownCh := common.GetTerminationSignalCh()

	metadata, err := blockStream.getMetadata()
	if err != nil {
		return err
	}

	if metadata == nil {
		return errors.New("expected metadata in archive but doesn't exist")
	}

	// check whether the local chain has the latest block already
	latestBlock, ok := chain.GetBlockByNumber(metadata.Latest, false)
	if ok && latestBlock.Hash() == metadata.LatestHash {
		return nil
	}

	maxFetchBlockNum := runtime.NumCPU() * 2

	// create channel to next block stream
	nextBlockCh := make(chan *types.Block, maxFetchBlockNum)

	storeLatestBlkNumber, exist := chain.GetHeaderNumber()
	if !exist {
		storeLatestBlkNumber = 0
	}

	// skip genesis block
	for {
		firstBlock, err := blockStream.nextBlock()
		if err != nil {
			return err
		}

		if firstBlock == nil {
			return nil
		}

		if firstBlock.Number() > 0 && firstBlock.Number() > storeLatestBlkNumber {
			nextBlockCh <- firstBlock

			break
		}

		log.Info("block exist, skip", "block", firstBlock.Number())
	}

	go func() {
		for {
			// check shutdown signal
			select {
			case <-shutdownCh:
				return
			default:
			}

			nextBlock, err := blockStream.nextBlock()
			if err != nil {
				log.Error("failed to read block", "err", err)
			}

			// end of stream
			if nextBlock == nil {
				nextBlockCh <- nil

				return
			}

			nextBlockCh <- nextBlock
		}
	}()

	for {
		nextBlock := <-nextBlockCh

		// end of stream
		if nextBlock == nil {
			break
		}

		storageBlk, exist := chain.GetBlockByNumber(nextBlock.Number(), false)

		if exist &&
			storageBlk.Number() == nextBlock.Number() &&
			storageBlk.Hash() != nextBlock.Hash() {
			return fmt.Errorf(
				"block %d has different hash in storage (%s) and archive (%s)",
				nextBlock.Number(),
				storageBlk.Hash(),
				nextBlock.Hash(),
			)
		}

		// skip existing blocks
		if !exist {
			if err := chain.VerifyFinalizedBlock(nextBlock); err != nil {
				return err
			}

			if err := chain.WriteBlock(nextBlock, WriteBlockSource); err != nil {
				return err
			}

			log.Info("block imported", "block", nextBlock.Number())
		} else {
			log.Info("block exist, skip", "block", nextBlock.Number())
		}

		select {
		case <-shutdownCh:
			return nil
		default:
		}
	}

	return nil
}

// blockStream parse RLP-encoded block from stream and consumed the used bytes
type blockStream struct {
	input  io.Reader
	buffer []byte
}

func newBlockStream(input io.Reader) *blockStream {
	return &blockStream{
		input:  input,
		buffer: make([]byte, 0, 1024), // impossible to estimate block size but minimum block size is about 900 bytes
	}
}

// getMetadata consumes some bytes from input and returns parsed Metadata
func (b *blockStream) getMetadata() (*Metadata, error) {
	size, err := b.loadRLPArray()
	if err != nil {
		return nil, err
	}

	if size == 0 {
		return nil, nil
	}

	return b.parseMetadata(size)
}

// nextBlock consumes some bytes from input and returns parsed block
func (b *blockStream) nextBlock() (*types.Block, error) {
	size, err := b.loadRLPArray()
	if err != nil {
		return nil, err
	}

	if size == 0 {
		return nil, nil
	}

	block, err := b.parseBlock(size)
	if err != nil {
		return nil, err
	}

	return block, nil
}

// loadRLPArray loads RLP encoded array from input to buffer
func (b *blockStream) loadRLPArray() (uint64, error) {
	prefix, err := b.loadRLPPrefix()
	if errors.Is(io.EOF, err) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	// read information from RLP array header
	headerSize, payloadSize, err := b.loadPrefixSize(1, prefix)
	if err != nil {
		return 0, err
	}

	if err = b.loadPayload(headerSize, payloadSize); err != nil {
		return 0, err
	}

	return headerSize + payloadSize, nil
}

// loadRLPPrefix loads first byte of RLP encoded data from input
func (b *blockStream) loadRLPPrefix() (byte, error) {
	buf := b.buffer[:1]
	if _, err := io.ReadFull(b.input, buf); err != nil {
		return 0, err
	}

	return buf[0], nil
}

// loadPrefixSize loads array's size from input and return RLP header size and payload size
// basically block should be array in RLP encoded value
// because block has 3 fields on the top: Header, Transactions, Uncles
func (b *blockStream) loadPrefixSize(offset uint64, prefix byte) (uint64, uint64, error) {
	switch {
	case prefix >= 0xc0 && prefix <= 0xf7:
		// an array whose size is less than 56
		return 1, uint64(prefix - 0xc0), nil
	case prefix >= 0xf8:
		// an array whose size is greater than or equal to 56
		// size of the data representing the size of payload
		payloadSizeSize := uint64(prefix - 0xf7)

		b.reserveCap(offset + payloadSizeSize)
		payloadSizeBytes := b.buffer[offset : offset+payloadSizeSize]

		n, err := io.ReadFull(b.input, payloadSizeBytes)
		if errors.Is(io.EOF, err) {
			return 0, 0, io.EOF
		}

		if err != nil {
			return 0, 0, err
		}

		if uint64(n) < payloadSizeSize {
			// couldn't load required amount of bytes
			return 0, 0, io.ErrUnexpectedEOF
		}

		payloadSize := new(big.Int).SetBytes(payloadSizeBytes).Int64()

		return payloadSizeSize + 1, uint64(payloadSize), nil
	}

	return 0, 0, errors.New("expected array but got bytes")
}

// loadPayload loads payload data from stream and store to buffer
func (b *blockStream) loadPayload(offset uint64, size uint64) error {
	b.reserveCap(offset + size)
	buf := b.buffer[offset : offset+size]

	if _, err := io.ReadFull(b.input, buf); err != nil {
		return err
	}

	return nil
}

// parseMetadata parses RLP encoded Metadata in buffer
func (b *blockStream) parseMetadata(size uint64) (*Metadata, error) {
	data := b.buffer[:size]
	metadata := &Metadata{}

	if err := metadata.UnmarshalRLP(data); err != nil {
		return nil, err
	}

	return metadata, nil
}

// parseBlock parses RLP encoded Block in buffer
func (b *blockStream) parseBlock(size uint64) (*types.Block, error) {
	data := b.buffer[:size]
	block := &types.Block{}

	if err := block.UnmarshalRLP(data); err != nil {
		return nil, err
	}

	return block, nil
}

// reserveCap makes sure the internal buffer has given size
func (b *blockStream) reserveCap(size uint64) {
	if diff := int64(size) - int64(cap(b.buffer)); diff > 0 {
		b.buffer = append(b.buffer[:cap(b.buffer)], make([]byte, diff)...)
	} else {
		b.buffer = b.buffer[:size]
	}
}
