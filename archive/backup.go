package archive

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/dogechain-lab/dogechain/helper/common"
	"github.com/dogechain-lab/dogechain/server/proto"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	"github.com/klauspost/compress/zstd"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// CreateBackup fetches blockchain data with the specific range via gRPC
// and save this data as binary archive to given path
func CreateBackup(
	conn *grpc.ClientConn,
	logger hclog.Logger,
	from uint64,
	to *uint64,
	outPath string,
	overwriteFile bool,
	enableZstdCompression bool,
	zstdLevel int,
) (resFrom uint64, resTo uint64, err error) {
	resFrom = 0
	resTo = 0
	err = nil

	// allow to overwrite the overwrites file only if it's explicitly set
	fileFlag := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if overwriteFile {
		fileFlag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}

	fp, err := os.OpenFile(outPath, fileFlag, 0644)
	if err != nil {
		return
	}

	defer func() {
		err = fp.Close()
	}()

	fbuf := bufio.NewWriterSize(fp, 1*1024*1024)

	defer func() {
		if err != nil {
			return
		}

		err = fbuf.Flush()
	}()

	var writeBuf io.Writer

	if enableZstdCompression {
		var zstdWriter *zstd.Encoder

		zstdWriter, err = zstd.NewWriter(fbuf,
			zstd.WithEncoderLevel(
				zstd.EncoderLevelFromZstd(zstdLevel),
			))
		if err != nil {
			return
		}

		defer func() {
			if err != nil {
				return
			}

			err = zstdWriter.Close()
		}()

		writeBuf = zstdWriter
	} else {
		writeBuf = fbuf
	}

	signalCh := common.GetTerminationSignalCh()
	ctx, cancelFn := context.WithCancel(context.Background())

	defer cancelFn()

	go func() {
		<-signalCh
		logger.Info("Caught termination signal, shutting down...")
		cancelFn()
	}()

	clt := proto.NewSystemClient(conn)

	var reqTo uint64

	var reqToHash types.Hash

	reqTo, reqToHash, err = determineTo(ctx, clt, to)
	if err != nil {
		return
	}

	var stream proto.System_ExportClient

	stream, err = clt.Export(ctx, &proto.ExportRequest{
		From: from,
		To:   reqTo,
	})
	if err != nil {
		return
	}

	if err = writeMetadata(writeBuf, logger, reqTo, reqToHash); err != nil {
		return
	}

	resFrom, resTo, err = processExportStream(stream, logger, writeBuf, from, reqTo)
	if err != nil {
		return
	}

	return
}

func determineTo(ctx context.Context, clt proto.SystemClient, to *uint64) (uint64, types.Hash, error) {
	status, err := clt.GetStatus(ctx, &emptypb.Empty{})
	if err != nil {
		return 0, types.Hash{}, err
	}

	if to != nil && *to < uint64(status.Current.Number) {
		// check the existence of the block when you have targetTo
		resp, err := clt.BlockByNumber(ctx, &proto.BlockByNumberRequest{Number: *to})
		if err == nil && resp != nil {
			block := types.Block{}
			if err := block.UnmarshalRLP(resp.Data); err == nil {
				// can use targetTo only if the node has the block at the specific height
				return block.Number(), block.Hash(), nil
			}
		}
	}

	// otherwise use latest block number as to
	return uint64(status.Current.Number), types.StringToHash(status.Current.Hash), nil
}

// writeMetadata writes the latest block height and the block hash to the writer
func writeMetadata(writer io.Writer, logger hclog.Logger, to uint64, toHash types.Hash) error {
	metadata := Metadata{
		Latest:     to,
		LatestHash: toHash,
	}

	// tips: writer.Write() not necessarily write all data, use io.Copy() instead
	_, err := io.Copy(writer, bytes.NewBuffer(metadata.MarshalRLP()))
	if err != nil {
		return err
	}

	logger.Info("Wrote metadata to backup", "latest", to, "hash", toHash)

	return err
}

func processExportStream(
	stream proto.System_ExportClient,
	logger hclog.Logger,
	writer io.Writer,
	targetFrom, targetTo uint64,
) (uint64, uint64, error) {
	var from, to, total uint64 = 0, 0, 0

	showProgress := func(event *proto.ExportEvent) {
		num := event.To - event.From
		total += num
		expectedTo := targetTo

		if targetTo == 0 {
			expectedTo = event.Latest
		}

		expectedTotal := expectedTo - targetFrom
		progress := 100 * (float64(event.To) - float64(targetFrom)) / float64(expectedTotal)

		logger.Info(
			fmt.Sprintf("%d blocks are written", num),
			"total", total,
			"from", targetFrom,
			"to", expectedTo,
			"progress", fmt.Sprintf("%.2f%%", progress),
		)
	}

	firstBlok := true

	for {
		event, err := stream.Recv()
		if errors.Is(io.EOF, err) || status.Code(err) == codes.Canceled {
			return from, to, nil
		}

		if err != nil {
			return from, to, err
		}

		// tips: writer.Write() not necessarily write all data, use io.Copy() instead
		if _, err := io.Copy(writer, bytes.NewBuffer(event.Data)); err != nil {
			return from, to, err
		}

		if firstBlok {
			from = event.From
			firstBlok = false
		}

		to = event.To

		showProgress(event)
	}
}
