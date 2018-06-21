package main

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
)

// GetGenesis Gets genesis map
func GetGenesis(genesis *core.Genesis) map[common.Address]*big.Int {
	genesisMap := make(map[common.Address]*big.Int)
	for key, value := range genesis.Alloc {
		genesisMap[key] = value.Balance
	}
	return genesisMap
}
