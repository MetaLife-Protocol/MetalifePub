package chain

import (
	"crypto/ecdsa"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Chain interface {
	GetChainName() string
	GetEventChan() <-chan Event
	StartEventListener() error
	StopEventListener()
	RegisterEventListenContract(contractAddresses ...common.Address) error
	UnRegisterEventListenContract(contractAddresses ...common.Address)
	DeployContract(opts *bind.TransactOpts, params ...string) (contractAddress common.Address, err error)
	SetLastBlockNumber(lastBlockNumber uint64)
	//GetContractProxy(contractAddress common.Address) ContractProxy
	GetConn() *ethclient.Client

	Transfer10ToAccount(key *ecdsa.PrivateKey, accountTo common.Address, amount *big.Int, nonce ...int) (err error) // for debug
}
