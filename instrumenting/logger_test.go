package instrumenting

import (
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestWriteContract(t *testing.T) {
	callFile, _ := os.Create("/tmp/calls.txt")
	createFile, _ := os.Create("/tmp/contracts.txt")

	stats := NewStats(callFile, createFile)

	from := common.BytesToAddress([]byte{0})
	to := common.BytesToAddress([]byte{0})
	value := new(big.Int).SetUint64(1)
	code := &[]byte{}

	stats.LogCreate(from, to, value, code)
	stats.WriteCreatedContracts()

}
