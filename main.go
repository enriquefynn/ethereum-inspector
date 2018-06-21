package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/enriquefynn/ethereum-inspector/instrumenting"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	pb "gopkg.in/cheggaaa/pb.v1"
)

var (
	cache, _ = strconv.Atoi(os.Args[3])
	ethDb, _ = ethdb.NewLDBDatabase(os.Args[1], cache, 256)
	// executeDb, _ = ethdb.NewMemDatabase()
	executeDb, err = ethdb.NewLDBDatabase(os.Args[2], cache, 256)
	engine         = ethash.NewFullFaker()

	logContractFile, _   = os.OpenFile("/tmp/contracts.txt", os.O_APPEND|os.O_WRONLY, 0600)
	logBlocks, _         = os.OpenFile("/tmp/blocks.txt", os.O_APPEND|os.O_WRONLY, 0600)
	logTransactions, _   = os.OpenFile("/tmp/transactions.txt", os.O_APPEND|os.O_WRONLY, 0600)
	idMappingFile, _     = os.OpenFile("/tmp/id_mapping.txt", os.O_APPEND|os.O_WRONLY, 0600)
	logBlocksBufio       = bufio.NewWriter(logBlocks)
	logTransactionsBufio = bufio.NewWriter(logTransactions)
	logContractsBufio    = bufio.NewWriter(logContractFile)
	idMappingsBufio      = bufio.NewReadWriter(bufio.NewReader(idMappingFile), bufio.NewWriter(idMappingFile))
	config               = params.MainnetChainConfig
	vmConfig             = vm.Config{
		LogBlocksFile:           logBlocksBufio,
		LogTransactionsFile:     logTransactionsBufio,
		LogCreatedContractsFile: logContractsBufio,
		IDMappingsFile:          idMappingsBufio,
	}
	stats = instrumenting.NewStats(logBlocksBufio, logTransactionsBufio, logContractsBufio, idMappingsBufio)

	cacheConfig = core.CacheConfig{
		Disabled:      true,
		TrieNodeLimit: 256 * 1024 * 1024,
		TrieTimeLimit: 5 * time.Minute,
	}
)

func main() {
	var blkCache uint64
	blkCache = 1024

	blockchain, err := core.NewBlockChain(ethDb, &cacheConfig, config, engine, vmConfig)
	genesis := core.DefaultGenesisBlock()
	// genesis.MustCommit(executeDb)

	executeBlockchain, errEx := core.NewBlockChain(executeDb, &cacheConfig, genesis.Config, engine, vmConfig)

	//executeBlockchain.Reset()
	if err != nil {
		panic("Blockchain failed to init")
	}
	if errEx != nil {
		// genesis.MustCommit(executeDb)
		executeBlockchain, errEx = core.NewBlockChain(executeDb, &cacheConfig, genesis.Config, engine, vmConfig)
		// executeBlockchain.Reset()
		if errEx != nil {
			panic("Execute blockchain failed to init")
		}
	}
	logProcessor := core.NewLogProcessor(config, executeBlockchain, engine)
	executeBlockchain.SetProcessor(logProcessor)

	lastBlockNumber := blockchain.CurrentBlock().Number()
	lastExecuteBlockNumber := executeBlockchain.CurrentBlock().Number().Uint64()
	log.Printf("Got %v blocks", lastBlockNumber)
	log.Printf("Execute Blockchain is at block %v", executeBlockchain.CurrentBlock().Number())
	// executeBlockchain.SetHead(0)

	progressBar := pb.StartNew(int(lastBlockNumber.Int64()))
	genesisMap := GetGenesis(genesis)

	stats.WriteGenesis(genesisMap)

	// start := time.Now()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	calledInterrupt := false
	go func() {
		for _ = range c {
			calledInterrupt = true
		}
	}()

	progressBar.Set(int(lastExecuteBlockNumber))
	// var blockInterval uint64 = 1000
	var elapsed time.Duration
	statsFile, _ := os.Create("/tmp/executing_stats.txt")
	blocks := make(types.Blocks, blkCache)
	var blkN uint64
	var nBlock uint64 = 1
	for nBlock != lastBlockNumber.Uint64() {
		// for nBlock := lastExecuteBlockNumber; nBlock < lastBlockNumber.Uint64(); nBlock++ {

		if calledInterrupt {
			fmt.Printf("Interrupted at block %d\n", nBlock)
			os.Exit(0)
		}
		// for ; nBlock < 50000; nBlock++ {
		for blkN = 0; blkN < blkCache; blkN++ {
			progressBar.Increment()
			blocks[blkN] = blockchain.GetBlockByNumber(nBlock)
			nBlock++
			if nBlock > lastBlockNumber.Uint64() {
				break
			}
		}
		// block = blockchain.GetBlockByNumber(nBlock + 1)
		// if nBlock >= lastBlockLog {
		// lastBlockLog += blockInterval

		start := time.Now()
		n, err := executeBlockchain.InsertRawChain(blocks)
		if err != nil {
			fmt.Printf("Error %v: %v\n", err, n)
		}
		elapsed = time.Since(start)
		fmt.Fprintf(statsFile, "%d %d\n", blocks[blkCache-1].Time(), elapsed.Nanoseconds())
		// }
		// executeBlockchain.InsertBlockAndLog(block)
	}

	// elapsed = time.Since(start)
	// log.Printf("Elapsed time %v", elapsed.Seconds())
	// log.Printf("Execute Blockchain is at block %v", executeBlockchain.CurrentBlock().Number())
}
