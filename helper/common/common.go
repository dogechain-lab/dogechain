package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/dogechain-lab/dogechain/helper/hex"
	"github.com/dogechain-lab/dogechain/types"
)

var (
	// MaxSafeJSInt represents max value which JS support
	// It is used for smartContract fields
	// Our staking repo is written in JS, as are many other clients
	// If we use higher value JS will not be able to parse it
	MaxSafeJSInt = uint64(math.Pow(2, 53) - 2)

	MaxGrpcMsgSize = 16 * 1024 * 1024 // 16MB
)

// MinInt returns the strictly lower number(int)
func MinInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}

// MinUint64 returns the strictly lower number
func MinInt64(a, b int64) int64 {
	if a < b {
		return a
	}

	return b
}

// MinUint64 returns the strictly lower number
func MinUint64(a, b uint64) uint64 {
	if a < b {
		return a
	}

	return b
}

// ClampInt64ToInt returns the int64 value clamped to the range of an int
func ClampInt64ToInt(v int64) int {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}

	if v < math.MinInt32 {
		return math.MinInt32
	}

	return int(v)
}

// MaxInt returns the strictly bigger int number
func MaxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
}

// MaxUint64 returns the strictly bigger number
func MaxUint64(a, b uint64) uint64 {
	if a > b {
		return a
	}

	return b
}

func ConvertUnmarshalledInt(x interface{}) (int64, error) {
	switch tx := x.(type) {
	case float64:
		return roundFloat(tx), nil
	case string:
		v, err := types.ParseUint64orHex(&tx)
		if err != nil {
			return 0, err
		}

		return int64(v), nil
	default:
		return 0, errors.New("unsupported type for unmarshalled integer")
	}
}

func roundFloat(num float64) int64 {
	return int64(num + math.Copysign(0.5, num))
}

func ToFixedFloat(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))

	return float64(roundFloat(num*output)) / output
}

// SetupDataDir sets up the data directory and the corresponding sub-directories
func SetupDataDir(dataDir string, paths []string) error {
	if err := createDir(dataDir); err != nil {
		return fmt.Errorf("failed to create data dir: (%s): %w", dataDir, err)
	}

	for _, path := range paths {
		path := filepath.Join(dataDir, path)
		if err := createDir(path); err != nil {
			return fmt.Errorf("failed to create path: (%s): %w", path, err)
		}
	}

	return nil
}

// DirectoryExists checks if the directory at the specified path exists
func DirectoryExists(directoryPath string) bool {
	// Grab the absolute filepath
	pathAbs, err := filepath.Abs(directoryPath)
	if err != nil {
		return false
	}

	// Check if the directory exists, and that it's actually a directory if there is a hit
	if fileInfo, statErr := os.Stat(pathAbs); os.IsNotExist(statErr) || (fileInfo != nil && !fileInfo.IsDir()) {
		return false
	}

	return true
}

// createDir creates a file system directory if it doesn't exist
func createDir(path string) error {
	_, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if os.IsNotExist(err) {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			return err
		}
	}

	return nil
}

// JSONNumber is the number represented in decimal or hex in json
type JSONNumber struct {
	Value uint64
}

func (d *JSONNumber) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, hex.EncodeUint64(d.Value))), nil
}

func (d *JSONNumber) UnmarshalJSON(data []byte) error {
	var rawValue interface{}
	if err := json.Unmarshal(data, &rawValue); err != nil {
		return err
	}

	val, err := ConvertUnmarshalledInt(rawValue)
	if err != nil {
		return err
	}

	if val < 0 {
		return errors.New("must be positive value")
	}

	d.Value = uint64(val)

	return nil
}

// GetTerminationSignalCh returns a channel to emit signals by ctrl + c
func GetTerminationSignalCh() <-chan os.Signal {
	// wait for the user to quit with ctrl-c
	signalCh := make(chan os.Signal, 1)
	signal.Notify(
		signalCh,
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGHUP,
	)

	return signalCh
}

// PadLeftOrTrim left-pads the passed in byte array to the specified size,
// or trims the array if it exceeds the passed in size
func PadLeftOrTrim(bb []byte, size int) []byte {
	l := len(bb)
	if l == size {
		return bb
	}

	if l > size {
		return bb[l-size:]
	}

	tmp := make([]byte, size)
	copy(tmp[size-l:], bb)

	return tmp
}

// Substr returns a substring of the input string
func Substr(input string, start int, size int) string {
	strRunes := []rune(input)

	if start < 0 {
		start = 0
	}

	if start >= len(strRunes) {
		return ""
	}

	if (start + size) > len(strRunes) {
		size = len(strRunes) - start
	}

	return string(strRunes[start : start+size])
}

// GetOutboundIP returns the preferred outbound ip of this machine
func GetOutboundIP() (net.IP, error) {
	// any public address will do
	conn, err := net.Dial("udp", "1.1.1.1")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	address, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, errors.New("cannot assert UDPAddr")
	}

	ipaddress := address.IP

	return ipaddress, nil
}
