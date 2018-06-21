package instrumenting

import (
	"bufio"
	"fmt"
	"math/big"
	"os"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type methodType uint8

type CoinbaseReward struct {
	Coinbase common.Address
	Reward   *big.Int
}

const (
	createType methodType = iota
	callType
	callcodeType
	delegateCallType
	staticCallType
	preCompiledType
	opSelfdestructType
)

var (
	nextID    uint64
	idMapping = make(map[common.Address]uint64)
)

// CallStruct structure to log Call
type logStruct struct {
	from  uint64
	to    uint64
	value string
	code  *[]byte
}

type logStructRange []logStruct

func (a logStructRange) Len() int      { return len(a) }
func (a logStructRange) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a logStructRange) Less(i, j int) bool {
	if a[i].from == a[j].from {
		if a[i].to == a[j].to {
			bi := new(big.Int)
			bj := new(big.Int)
			return bi.Cmp(bj) == -1
		}
		return a[i].to < a[j].to
	}
	return a[i].from < a[j].from
}

// Stats logs statistics about the EVM code execution
type Stats struct {
	create          map[logStruct]uint32
	call            map[logStruct]uint32
	callCode        map[logStruct]uint32
	delegateCall    map[logStruct]uint32
	staticCall      map[logStruct]uint32
	preCompiledCode map[logStruct]uint32
	opSelfdestruct  map[logStruct]uint32
	failed          bool
	size            common.StorageSize

	coinbaseReward []CoinbaseReward

	LogBlocksFile           *bufio.Writer
	LogTransactionsFile     *bufio.Writer
	LogCreatedContractsFile *bufio.Writer
	idMappingsFile          *bufio.ReadWriter
	dontLog                 bool
}

func hasValue(logType methodType) bool {
	if (logType == callType) || (logType == createType) || (logType == callcodeType) ||
		(logType == opSelfdestructType) {
		return true
	}
	return false
}

// NewStats return a new statistics object
func NewStats(_logBlocksFile, _logTransactionsFile, _logCreatedContractsFile *bufio.Writer, _idMappingsFile *bufio.ReadWriter) *Stats {
	dontLog := false
	if _logBlocksFile == nil || _logTransactionsFile == nil || _logCreatedContractsFile == nil || _idMappingsFile == nil {
		dontLog = true
	}

	stats := &Stats{
		call:            make(map[logStruct]uint32),
		create:          make(map[logStruct]uint32),
		callCode:        make(map[logStruct]uint32),
		delegateCall:    make(map[logStruct]uint32),
		staticCall:      make(map[logStruct]uint32),
		preCompiledCode: make(map[logStruct]uint32),
		opSelfdestruct:  make(map[logStruct]uint32),

		LogBlocksFile:           _logBlocksFile,
		LogTransactionsFile:     _logTransactionsFile,
		LogCreatedContractsFile: _logCreatedContractsFile,
		idMappingsFile:          _idMappingsFile,
		dontLog:                 dontLog,
	}
	return stats
}

func SetNextId(idMappingFile *os.File) {
	nextID = 0
	var line common.Address
	scanner := bufio.NewScanner(idMappingFile)
	for scanner.Scan() {
		line = common.HexToAddress(scanner.Text())
		idMapping[line] = nextID
		nextID++
	}
	fmt.Printf("ID_MAPPING stop at: %d\n", nextID)
}

func (stats *Stats) getID(address common.Address) uint64 {
	if stats.dontLog {
		return 0
	}
	defer stats.idMappingsFile.Flush()
	if val, ok := idMapping[address]; ok {
		return val
	}
	defer func() { nextID++ }()
	fmt.Fprintf(stats.idMappingsFile, "%x\n", address)
	idMapping[address] = nextID
	return nextID
}

// LogCreate saves a Create contract on the stats
func (stats *Stats) LogCreate(_from, _to common.Address, _value *big.Int, _code *[]byte) {
	callStat := logStruct{from: stats.getID(_from), to: stats.getID(_to), value: _value.String(), code: _code}
	stats.create[callStat]++
}

// LogCall saves a Call on the stats
func (stats *Stats) LogCall(_from, _to common.Address, _value *big.Int) {
	callStat := logStruct{from: stats.getID(_from), to: stats.getID(_to), value: _value.String()}
	stats.call[callStat]++
}

// LogCallCode saves a CallCode on the stats
func (stats *Stats) LogCallCode(_from, _to common.Address, _value *big.Int) {
	callStat := logStruct{from: stats.getID(_from), to: stats.getID(_to), value: _value.String()}
	stats.callCode[callStat]++
}

// LogDelegateCall saves a DelegateCall on the stats
func (stats *Stats) LogDelegateCall(_from, _to common.Address) {
	callStat := logStruct{from: stats.getID(_from), to: stats.getID(_to)}
	stats.delegateCall[callStat]++
}

// LogStaticCall saves a StaticCall on the stats
func (stats *Stats) LogStaticCall(_from, _to common.Address) {
	callStat := logStruct{from: stats.getID(_from), to: stats.getID(_to)}
	stats.staticCall[callStat]++
}

// LogPreCompiledCode saves a call to a pre compiled contract on the stats
func (stats *Stats) LogPreCompiledCode(_from, _to common.Address) {
	callStat := logStruct{from: stats.getID(_from), to: stats.getID(_to)}
	stats.preCompiledCode[callStat]++
}

// LogOpSelfdestruct saves a call to a pre compiled contract on the stats
func (stats *Stats) LogOpSelfdestruct(_from, _to common.Address, _value *big.Int) {
	callStat := logStruct{from: stats.getID(_from), to: stats.getID(_to), value: _value.String()}
	stats.opSelfdestruct[callStat]++
}

// SetSize sets the transaction size
func (stats *Stats) SetSize(size common.StorageSize) {
	stats.size = size
}

// SetCoinbaseReward sets the SetCoinbaseReward
func (stats *Stats) SetCoinbaseReward(coinbaseReward []CoinbaseReward) {
	stats.coinbaseReward = coinbaseReward
}

// SetTxFailed sets transaction as success or fail
func (stats *Stats) SetTxFailed(failed bool) {
	stats.failed = failed
}

func (stats *Stats) writeCall(logEntry *map[logStruct]uint32, logType methodType) {
	if len(*logEntry) != 0 {
		fmt.Fprintf(stats.LogTransactionsFile, "%d %d\n", logType, len(*logEntry))
	} else {
		return
	}

	keys := make(logStructRange, 0)
	for k := range *logEntry {
		keys = append(keys, k)
	}
	sort.Sort(keys)

	// for call, times := range *logEntry {
	for _, call := range keys {
		fmt.Fprintf(stats.LogTransactionsFile, "%d %d", call.from, call.to)
		if hasValue(logType) {
			fmt.Fprintf(stats.LogTransactionsFile, " %s", call.value)
		}
		fmt.Fprintf(stats.LogTransactionsFile, " %d\n", (*logEntry)[call])
	}
}

func (stats *Stats) nonEmptyCalls() uint8 {
	var total uint8
	if len(stats.create) > 0 {
		total++
	}
	if len(stats.call) > 0 {
		total++
	}
	if len(stats.callCode) > 0 {
		total++
	}
	if len(stats.delegateCall) > 0 {
		total++
	}
	if len(stats.staticCall) > 0 {
		total++
	}
	if len(stats.preCompiledCode) > 0 {
		total++
	}
	if len(stats.opSelfdestruct) > 0 {
		total++
	}
	return total
}

// WriteCalls write transactions calls in file
func (stats *Stats) WriteCalls() {
	defer stats.LogTransactionsFile.Flush()
	// T is a new Transaction
	failed := 0
	if stats.failed {
		failed = 1
	}
	fmt.Fprintf(stats.LogTransactionsFile, "T %d %.0f %d\n", failed, stats.size, stats.nonEmptyCalls())
	stats.writeCall(&stats.create, createType)
	stats.writeCall(&stats.call, callType)
	stats.writeCall(&stats.callCode, callcodeType)
	stats.writeCall(&stats.delegateCall, delegateCallType)
	stats.writeCall(&stats.staticCall, staticCallType)
	stats.writeCall(&stats.preCompiledCode, preCompiledType)
	stats.writeCall(&stats.opSelfdestruct, opSelfdestructType)
}

// WriteCreatedContracts Write contracts code
func (stats *Stats) WriteCreatedContracts() {
	defer stats.LogCreatedContractsFile.Flush()
	for call := range stats.create {
		fmt.Fprintf(stats.LogCreatedContractsFile, "%d 0x%x\n", call.to, *call.code)
	}
}

// WriteBlockHeader writes the block header
func (stats *Stats) WriteBlockHeader(callPath *bufio.Writer, block *types.Block, nTransactions int) {
	defer callPath.Flush()
	fmt.Fprintf(callPath, "B %d %d %d %d %.0f %d", block.Number(), block.Time(),
		block.GasUsed(), block.GasLimit(), block.Size(), len(stats.coinbaseReward))
	for _, coinbaseReward := range stats.coinbaseReward {
		coinbase := stats.getID(coinbaseReward.Coinbase)
		fmt.Fprintf(callPath, " %d %s", coinbase, coinbaseReward.Reward)
	}
	fmt.Fprintf(callPath, " %d\n", nTransactions)
}

// WriteGenesis writes the genesis block
func (stats *Stats) WriteGenesis(genesis map[common.Address]*big.Int) {
	defer stats.LogBlocksFile.Flush()
	fmt.Fprintf(stats.LogBlocksFile, "G %d\n", len(genesis))

	keys := make([]string, 0)
	for k := range genesis {
		keys = append(keys, k.Hex())
	}
	sort.Strings(keys)
	for _, k := range keys {
		id := stats.getID(common.HexToAddress(k))
		fmt.Fprintf(stats.LogBlocksFile, "%d %d\n", id, genesis[common.HexToAddress(k)])
	}
}
