// Copyright 2019 The go-ethereum Authors
// (original work)
// Copyright 2024 The Erigon Authors
// (modifications)
// This file is part of Erigon.
//
// Erigon is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Erigon is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Erigon. If not, see <http://www.gnu.org/licenses/>.

package backends

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	ethereum "github.com/erigontech/erigon"
	"github.com/erigontech/erigon-db/rawdb"
	"github.com/erigontech/erigon-lib/abi"
	"github.com/erigontech/erigon-lib/chain"
	"github.com/erigontech/erigon-lib/chain/params"
	"github.com/erigontech/erigon-lib/common"
	"github.com/erigontech/erigon-lib/common/u256"
	"github.com/erigontech/erigon-lib/crypto"
	"github.com/erigontech/erigon-lib/kv"
	"github.com/erigontech/erigon-lib/types"
	"github.com/erigontech/erigon/execution/abi/bind"
)

func TestSimulatedBackend(t *testing.T) {
	var gasLimit uint64 = 8000029
	key, _ := crypto.GenerateKey() // nolint: gosec
	auth, _ := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))
	genAlloc := make(types.GenesisAlloc)
	genAlloc[auth.From] = types.GenesisAccount{Balance: big.NewInt(9223372036854775807)}

	sim := NewSimulatedBackend(t, genAlloc, gasLimit)

	// should return an error if the txn is not found
	txHash := common.HexToHash("2")
	_, isPending, err := sim.TransactionByHash(context.Background(), txHash)

	if isPending {
		t.Fatal("transaction should not be pending")
	}
	if err != ethereum.NotFound {
		t.Fatalf("err should be `ethereum.NotFound` but received %v", err)
	}

	// generate a transaction and confirm you can retrieve it
	code := `6060604052600a8060106000396000f360606040526008565b00`
	var gas uint64 = 3000000
	signer := types.MakeSigner(chain.TestChainConfig, 1, 0)
	var txn types.Transaction = types.NewContractCreation(0, u256.Num0, gas, u256.Num1, common.FromHex(code))
	txn, _ = types.SignTx(txn, *signer, key)

	err = sim.SendTransaction(context.Background(), txn)
	if err != nil {
		t.Fatalf("error sending transaction: %v", err)
	}

	txHash = txn.Hash()
	_, isPending, err = sim.TransactionByHash(context.Background(), txHash)
	if err != nil {
		t.Fatalf("error getting transaction with hash: %v", txHash.String())
	}
	if !isPending {
		t.Fatal("transaction should have pending status")
	}

	sim.Commit()
	_, isPending, err = sim.TransactionByHash(context.Background(), txHash)
	if err != nil {
		t.Fatalf("error getting transaction with hash: %v %v", txHash.String(), err.Error())
	}
	if isPending {
		t.Fatal("transaction should not have pending status")
	}
}

var testKey, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")

//	 the following is based on this contract:
//	 contract T {
//	 	event received(address sender, uint amount, bytes memo);
//	 	event receivedAddr(address sender);
//
//	 	function receive(bytes calldata memo) external payable returns (string memory res) {
//	 		emit received(msg.sender, msg.value, memo);
//	 		emit receivedAddr(msg.sender);
//			    return "hello world";
//	 	}
//	 }
const abiJSON = `[ { "constant": false, "inputs": [ { "name": "memo", "type": "bytes" } ], "name": "receive", "outputs": [ { "name": "res", "type": "string" } ], "payable": true, "stateMutability": "payable", "type": "function" }, { "anonymous": false, "inputs": [ { "indexed": false, "name": "sender", "type": "address" }, { "indexed": false, "name": "amount", "type": "uint256" }, { "indexed": false, "name": "memo", "type": "bytes" } ], "name": "received", "type": "event" }, { "anonymous": false, "inputs": [ { "indexed": false, "name": "sender", "type": "address" } ], "name": "receivedAddr", "type": "event" } ]`
const abiBin = `0x608060405234801561001057600080fd5b506102a0806100206000396000f3fe60806040526004361061003b576000357c010000000000000000000000000000000000000000000000000000000090048063a69b6ed014610040575b600080fd5b6100b76004803603602081101561005657600080fd5b810190808035906020019064010000000081111561007357600080fd5b82018360208201111561008557600080fd5b803590602001918460018302840111640100000000831117156100a757600080fd5b9091929391929390505050610132565b6040518080602001828103825283818151815260200191508051906020019080838360005b838110156100f75780820151818401526020810190506100dc565b50505050905090810190601f1680156101245780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b60607f75fd880d39c1daf53b6547ab6cb59451fc6452d27caa90e5b6649dd8293b9eed33348585604051808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001848152602001806020018281038252848482818152602001925080828437600081840152601f19601f8201169050808301925050509550505050505060405180910390a17f46923992397eac56cf13058aced2a1871933622717e27b24eabc13bf9dd329c833604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390a16040805190810160405280600b81526020017f68656c6c6f20776f726c6400000000000000000000000000000000000000000081525090509291505056fea165627a7a72305820ff0c57dad254cfeda48c9cfb47f1353a558bccb4d1bc31da1dae69315772d29e0029`
const deployedCode = `60806040526004361061003b576000357c010000000000000000000000000000000000000000000000000000000090048063a69b6ed014610040575b600080fd5b6100b76004803603602081101561005657600080fd5b810190808035906020019064010000000081111561007357600080fd5b82018360208201111561008557600080fd5b803590602001918460018302840111640100000000831117156100a757600080fd5b9091929391929390505050610132565b6040518080602001828103825283818151815260200191508051906020019080838360005b838110156100f75780820151818401526020810190506100dc565b50505050905090810190601f1680156101245780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b60607f75fd880d39c1daf53b6547ab6cb59451fc6452d27caa90e5b6649dd8293b9eed33348585604051808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001848152602001806020018281038252848482818152602001925080828437600081840152601f19601f8201169050808301925050509550505050505060405180910390a17f46923992397eac56cf13058aced2a1871933622717e27b24eabc13bf9dd329c833604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390a16040805190810160405280600b81526020017f68656c6c6f20776f726c6400000000000000000000000000000000000000000081525090509291505056fea165627a7a72305820ff0c57dad254cfeda48c9cfb47f1353a558bccb4d1bc31da1dae69315772d29e0029`

// expected return value contains "hello world"
var expectedReturn = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 32, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 11, 104, 101, 108, 108, 111, 32, 119, 111, 114, 108, 100, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

func simTestBackend(t *testing.T, testAddr common.Address) *SimulatedBackend {
	expectedBal := uint256.NewInt(10000000000)
	return NewSimulatedBackend(t,
		types.GenesisAlloc{
			testAddr: {Balance: expectedBal.ToBig()},
		}, 10000000,
	)
}

func TestNewSimulatedBackend(t *testing.T) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)
	expectedBal := uint256.NewInt(10000000000)
	sim := simTestBackend(t, testAddr)

	if sim.m.ChainConfig != chain.TestChainConfig {
		t.Errorf("expected sim config to equal chain.TestChainConfig, got %v", sim.m.ChainConfig)
	}

	if sim.m.ChainConfig != chain.TestChainConfig {
		t.Errorf("expected sim blockchain config to equal chain.TestChainConfig, got %v", sim.m.ChainConfig)
	}
	tx, err1 := sim.DB().BeginTemporalRo(context.Background())
	if err1 != nil {
		t.Errorf("TestNewSimulatedBackend create tx: %v", err1)
	}
	defer tx.Rollback()

	var num uint64
	if err := sim.m.DB.View(context.Background(), func(tx kv.Tx) error {
		num = rawdb.ReadCurrentHeader(tx).Number.Uint64()
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	statedb := sim.stateByBlockNumber(tx, new(big.Int).SetUint64(num+1))
	bal, err := statedb.GetBalance(testAddr)
	if err != nil {
		t.Fatal(err)
	}
	if !bal.Eq(expectedBal) {
		t.Errorf("expected balance for test address not received. expected: %v actual: %v", expectedBal, bal)
	}
}

func TestSimulatedBackend_AdjustTime(t *testing.T) {
	sim := NewSimulatedBackend(t,
		types.GenesisAlloc{}, 10000000,
	)

	prevTime := sim.pendingBlock.Time()
	if err := sim.AdjustTime(time.Second); err != nil {
		t.Error(err)
	}
	newTime := sim.pendingBlock.Time()

	if newTime-prevTime != uint64(time.Second.Seconds()) {
		t.Errorf("adjusted time not equal to a second. prev: %v, new: %v", prevTime, newTime)
	}
}

func TestNewSimulatedBackend_AdjustTimeFail(t *testing.T) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)
	sim := simTestBackend(t, testAddr)
	// Create txn and send
	amount, _ := uint256.FromBig(big.NewInt(1000))
	gasPrice, _ := uint256.FromBig(big.NewInt(1))
	signer := types.MakeSigner(chain.TestChainConfig, 1, 0)
	var txn types.Transaction = types.NewTransaction(0, testAddr, amount, params.TxGas, gasPrice, nil)
	signedTx, err := types.SignTx(txn, *signer, testKey)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}
	sim.SendTransaction(context.Background(), signedTx) //nolint:errcheck
	// AdjustTime should fail on non-empty block
	if err = sim.AdjustTime(time.Second); err == nil {
		t.Error("Expected adjust time to error on non-empty block")
	}
	sim.Commit()

	prevTime := sim.pendingBlock.Time()
	if err = sim.AdjustTime(time.Minute); err != nil {
		t.Error(err)
	}
	newTime := sim.pendingBlock.Time()
	if newTime-prevTime != uint64(time.Minute.Seconds()) {
		t.Errorf("adjusted time not equal to a minute. prev: %v, new: %v", prevTime, newTime)
	}
	// Put a transaction after adjusting time
	amount2, _ := uint256.FromBig(big.NewInt(1000))
	gasPrice2, _ := uint256.FromBig(big.NewInt(1))
	var tx2 types.Transaction = types.NewTransaction(1, testAddr, amount2, params.TxGas, gasPrice2, nil)
	signedTx2, err := types.SignTx(tx2, *signer, testKey)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}
	sim.SendTransaction(context.Background(), signedTx2) //nolint:errcheck
	sim.Commit()
	newTime = sim.pendingBlock.Time()
	if newTime-prevTime >= uint64(time.Minute.Seconds()) {
		t.Errorf("time adjusted, but shouldn't be: prev: %v, new: %v", prevTime, newTime)
	}
}

func TestSimulatedBackend_BalanceAt(t *testing.T) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)
	expectedBal := uint256.NewInt(10000000000)
	sim := simTestBackend(t, testAddr)
	bgCtx := context.Background()

	bal, err := sim.BalanceAt(bgCtx, testAddr, nil)
	if err != nil {
		t.Error(err)
	}

	if !bal.Eq(expectedBal) {
		t.Errorf("expected balance for test address not received. expected: %v actual: %v", expectedBal, bal)
	}
}

func TestSimulatedBackend_BlockByHash(t *testing.T) {
	sim := NewSimulatedBackend(t,
		types.GenesisAlloc{}, 10000000,
	)
	bgCtx := context.Background()

	block, err := sim.BlockByNumber(bgCtx, nil)
	if err != nil {
		t.Errorf("could not get recent block: %v", err)
	}
	blockByHash, err := sim.BlockByHash(bgCtx, block.Hash())
	if err != nil {
		t.Errorf("could not get recent block: %v", err)
	}

	if block.Hash() != blockByHash.Hash() {
		t.Errorf("did not get expected block")
	}
}

func TestSimulatedBackend_BlockByNumber(t *testing.T) {
	sim := NewSimulatedBackend(t,
		types.GenesisAlloc{}, 10000000,
	)
	bgCtx := context.Background()

	block, err := sim.BlockByNumber(bgCtx, nil)
	if err != nil {
		t.Errorf("could not get recent block: %v", err)
	}
	if block.NumberU64() != 0 {
		t.Errorf("did not get most recent block, instead got block number %v", block.NumberU64())
	}

	// create one block
	sim.Commit()

	block, err = sim.BlockByNumber(bgCtx, nil)
	if err != nil {
		t.Errorf("could not get recent block: %v", err)
	}
	if block.NumberU64() != 1 {
		t.Errorf("did not get most recent block, instead got block number %v", block.NumberU64())
	}

	blockByNumber, err := sim.BlockByNumber(bgCtx, big.NewInt(1))
	if err != nil {
		t.Errorf("could not get block by number: %v", err)
	}
	if blockByNumber.Hash() != block.Hash() {
		t.Errorf("did not get the same block with height of 1 as before")
	}
}

func TestSimulatedBackend_NonceAt(t *testing.T) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)

	sim := simTestBackend(t, testAddr)
	bgCtx := context.Background()

	nonce, err := sim.NonceAt(bgCtx, testAddr, big.NewInt(0))
	if err != nil {
		t.Errorf("could not get nonce for test addr: %v", err)
	}

	if nonce != uint64(0) {
		t.Errorf("received incorrect nonce. expected 0, got %v", nonce)
	}

	// create a signed transaction to send
	signer := types.MakeSigner(chain.TestChainConfig, 1, 0)
	var txn types.Transaction = types.NewTransaction(nonce, testAddr, uint256.NewInt(1000), params.TxGas, uint256.NewInt(1), nil)
	signedTx, err := types.SignTx(txn, *signer, testKey)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}

	// send txn to simulated backend
	err = sim.SendTransaction(bgCtx, signedTx)
	if err != nil {
		t.Errorf("could not add txn to pending block: %v", err)
	}
	sim.Commit()

	newNonce, err := sim.NonceAt(bgCtx, testAddr, big.NewInt(1))
	if err != nil {
		t.Errorf("could not get nonce for test addr: %v", err)
	}

	if newNonce != nonce+uint64(1) {
		t.Errorf("received incorrect nonce. expected 1, got %v", nonce)
	}
	// create some more blocks
	sim.Commit()
	// Check that we can get data for an older block/state
	newNonce, err = sim.NonceAt(bgCtx, testAddr, big.NewInt(1))
	if err != nil {
		t.Fatalf("could not get nonce for test addr: %v", err)
	}
	if newNonce != nonce+uint64(1) {
		t.Fatalf("received incorrect nonce. expected 1, got %v", nonce)
	}
}

func TestSimulatedBackend_SendTransaction(t *testing.T) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)

	sim := simTestBackend(t, testAddr)
	bgCtx := context.Background()

	// create a signed transaction to send
	signer := types.MakeSigner(chain.TestChainConfig, 1, 0)
	var txn types.Transaction = types.NewTransaction(uint64(0), testAddr, uint256.NewInt(1000), params.TxGas, uint256.NewInt(1), nil)
	signedTx, err := types.SignTx(txn, *signer, testKey)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}

	// send txn to simulated backend
	err = sim.SendTransaction(bgCtx, signedTx)
	if err != nil {
		t.Errorf("could not add txn to pending block: %v", err)
	}
	sim.Commit()

	block, err := sim.BlockByNumber(bgCtx, big.NewInt(1))
	if err != nil {
		t.Errorf("could not get block at height 1: %v", err)
	}

	if signedTx.Hash() != block.Transactions()[0].Hash() {
		t.Errorf("did not commit sent transaction. expected hash %v got hash %v", block.Transactions()[0].Hash(), signedTx.Hash())
	}
}

func TestSimulatedBackend_TransactionByHash(t *testing.T) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)

	sim := NewSimulatedBackend(t,
		types.GenesisAlloc{
			testAddr: {Balance: big.NewInt(10000000000)},
		}, 10000000)
	bgCtx := context.Background()

	// create a signed transaction to send
	signer := types.MakeSigner(chain.TestChainConfig, 1, 0)
	var txn types.Transaction = types.NewTransaction(uint64(0), testAddr, uint256.NewInt(1000), params.TxGas, uint256.NewInt(1), nil)
	signedTx, err := types.SignTx(txn, *signer, testKey)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}

	// send txn to simulated backend
	err = sim.SendTransaction(bgCtx, signedTx)
	if err != nil {
		t.Errorf("could not add txn to pending block: %v", err)
	}

	// ensure txn is committed pending
	receivedTx, pending, err := sim.TransactionByHash(bgCtx, signedTx.Hash())
	if err != nil {
		t.Errorf("could not get transaction by hash %v: %v", signedTx.Hash(), err)
	}
	if !pending {
		t.Errorf("expected transaction to be in pending state")
	}
	if receivedTx.Hash() != signedTx.Hash() {
		t.Errorf("did not received committed transaction. expected hash %v got hash %v", signedTx.Hash(), receivedTx.Hash())
	}

	sim.Commit()

	// ensure txn is not and committed pending
	receivedTx, pending, err = sim.TransactionByHash(bgCtx, signedTx.Hash())
	if err != nil {
		t.Errorf("could not get transaction by hash %v: %v", signedTx.Hash(), err)
	}
	if pending {
		t.Errorf("expected transaction to not be in pending state")
	}
	if receivedTx.Hash() != signedTx.Hash() {
		t.Errorf("did not received committed transaction. expected hash %v got hash %v", signedTx.Hash(), receivedTx.Hash())
	}
}

func TestSimulatedBackend_EstimateGas(t *testing.T) {
	/*
		pragma solidity ^0.6.4;
		contract GasEstimation {
		    function PureRevert() public { revert(); }
		    function Revert() public { revert("revert reason");}
		    function OOG() public { for (uint i = 0; ; i++) {}}
		    function Assert() public { assert(false);}
		    function Valid() public {}
		}*/
	const contractAbi = "[{\"inputs\":[],\"name\":\"Assert\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"OOG\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"PureRevert\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"Revert\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"Valid\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"
	const contractBin = "0x60806040523480156100115760006000fd5b50610017565b61016e806100266000396000f3fe60806040523480156100115760006000fd5b506004361061005c5760003560e01c806350f6fe3414610062578063aa8b1d301461006c578063b9b046f914610076578063d8b9839114610080578063e09fface1461008a5761005c565b60006000fd5b61006a610094565b005b6100746100ad565b005b61007e6100b5565b005b6100886100c2565b005b610092610135565b005b6000600090505b5b808060010191505061009b565b505b565b60006000fd5b565b600015156100bf57fe5b5b565b6040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252600d8152602001807f72657665727420726561736f6e0000000000000000000000000000000000000081526020015060200191505060405180910390fd5b565b5b56fea2646970667358221220345bbcbb1a5ecf22b53a78eaebf95f8ee0eceff6d10d4b9643495084d2ec934a64736f6c63430006040033"

	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)
	opts, _ := bind.NewKeyedTransactorWithChainID(key, big.NewInt(1337))

	sim := NewSimulatedBackend(t, types.GenesisAlloc{addr: {Balance: big.NewInt(common.Ether)}}, 10000000)

	parsed, _ := abi.JSON(strings.NewReader(contractAbi))
	contractAddr, _, _, _ := bind.DeployContract(opts, parsed, common.FromHex(contractBin), sim)
	sim.Commit()

	var cases = []struct {
		name        string
		message     ethereum.CallMsg
		expect      uint64
		expectError error
		expectData  interface{}
	}{
		{"plain transfer(valid)", ethereum.CallMsg{
			From:     addr,
			To:       &addr,
			Gas:      0,
			GasPrice: u256.Num0,
			Value:    u256.Num1,
			Data:     nil,
		}, params.TxGas, nil, nil},

		{"plain transfer(invalid)", ethereum.CallMsg{
			From:     addr,
			To:       &contractAddr,
			Gas:      0,
			GasPrice: u256.Num0,
			Value:    u256.Num1,
			Data:     nil,
		}, 0, errors.New("execution reverted"), nil},

		{"Revert", ethereum.CallMsg{
			From:     addr,
			To:       &contractAddr,
			Gas:      0,
			GasPrice: u256.Num0,
			Value:    nil,
			Data:     common.Hex2Bytes("d8b98391"),
		}, 0, errors.New("execution reverted: revert reason"), "0x08c379a00000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000d72657665727420726561736f6e00000000000000000000000000000000000000"},

		{"PureRevert", ethereum.CallMsg{
			From:     addr,
			To:       &contractAddr,
			Gas:      0,
			GasPrice: u256.Num0,
			Value:    nil,
			Data:     common.Hex2Bytes("aa8b1d30"),
		}, 0, errors.New("execution reverted"), nil},

		{"OOG", ethereum.CallMsg{
			From:     addr,
			To:       &contractAddr,
			Gas:      100000,
			GasPrice: u256.Num0,
			Value:    nil,
			Data:     common.Hex2Bytes("50f6fe34"),
		}, 0, errors.New("gas required exceeds allowance (100000)"), nil},

		{"Assert", ethereum.CallMsg{
			From:     addr,
			To:       &contractAddr,
			Gas:      100000,
			GasPrice: u256.Num0,
			Value:    nil,
			Data:     common.Hex2Bytes("b9b046f9"),
		}, 0, errors.New("invalid opcode: INVALID"), nil},

		{"Valid", ethereum.CallMsg{
			From:     addr,
			To:       &contractAddr,
			Gas:      100000,
			GasPrice: u256.Num0,
			Value:    nil,
			Data:     common.Hex2Bytes("e09fface"),
		}, 21275, nil, nil},
	}
	for _, c := range cases {
		got, err := sim.EstimateGas(context.Background(), c.message)
		if c.expectError != nil {
			if err == nil {
				t.Fatalf("Expect error, got nil")
			}
			if c.expectError.Error() != err.Error() {
				t.Fatalf("Expect error, want %v, got %v", c.expectError, err)
			}
			if c.expectData != nil {
				if err, ok := err.(*revertError); !ok {
					t.Fatalf("Expect revert error, got %T", err)
				} else if !reflect.DeepEqual(err.ErrorData(), c.expectData) {
					t.Fatalf("Error data mismatch, want %v, got %v", c.expectData, err.ErrorData())
				}
			}
			continue
		}
		if got != c.expect {
			t.Fatalf("Gas estimation mismatch, want %d, got %d", c.expect, got)
		}
	}
}

func TestSimulatedBackend_EstimateGasWithPrice(t *testing.T) {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)

	sim := NewSimulatedBackend(t, types.GenesisAlloc{addr: {Balance: big.NewInt(common.Ether*2 + 2e17)}}, 10000000)

	recipient := common.HexToAddress("deadbeef")
	var cases = []struct {
		name        string
		message     ethereum.CallMsg
		expect      uint64
		expectError error
	}{
		{"EstimateWithoutPrice", ethereum.CallMsg{
			From:     addr,
			To:       &recipient,
			Gas:      0,
			GasPrice: uint256.NewInt(0),
			Value:    uint256.NewInt(1000),
			Data:     nil,
		}, 21000, nil},

		{"EstimateWithPrice", ethereum.CallMsg{
			From:     addr,
			To:       &recipient,
			Gas:      0,
			GasPrice: uint256.NewInt(1000),
			Value:    uint256.NewInt(1000),
			Data:     nil,
		}, 21000, nil},

		{"EstimateWithVeryHighPrice", ethereum.CallMsg{
			From:     addr,
			To:       &recipient,
			Gas:      0,
			GasPrice: uint256.NewInt(1e14), // gascost = 2.1ether
			Value:    uint256.NewInt(1e17), // the remaining balance for fee is 2.1ether
			Data:     nil,
		}, 21000, nil},

		{"EstimateWithSuperhighPrice", ethereum.CallMsg{
			From:     addr,
			To:       &recipient,
			Gas:      0,
			GasPrice: uint256.NewInt(2e14), // gascost = 4.2ether
			Value:    uint256.NewInt(1000),
			Data:     nil,
		}, 21000, errors.New("gas required exceeds allowance (10999)")}, // 10999=(2.2ether-1000wei)/(2e14)
	}
	for _, c := range cases {
		got, err := sim.EstimateGas(context.Background(), c.message)
		if c.expectError != nil {
			if err == nil {
				t.Fatalf("Expect error, got nil")
			}
			if c.expectError.Error() != err.Error() {
				t.Fatalf("Expect error, want %v, got %v", c.expectError, err)
			}
			continue
		}
		if got != c.expect {
			t.Fatalf("Gas estimation mismatch, want %d, got %d", c.expect, got)
		}
	}
}

func TestSimulatedBackend_HeaderByHash(t *testing.T) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)

	sim := simTestBackend(t, testAddr)
	bgCtx := context.Background()

	header, err := sim.HeaderByNumber(bgCtx, nil)
	if err != nil {
		t.Errorf("could not get recent block: %v", err)
	}
	headerByHash, err := sim.HeaderByHash(bgCtx, header.Hash())
	if err != nil {
		t.Errorf("could not get recent block: %v", err)
	}

	if header.Hash() != headerByHash.Hash() {
		t.Errorf("did not get expected block")
	}
}

func TestSimulatedBackend_HeaderByNumber(t *testing.T) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)

	sim := simTestBackend(t, testAddr)
	bgCtx := context.Background()

	latestBlockHeader, err := sim.HeaderByNumber(bgCtx, nil)
	if err != nil {
		t.Errorf("could not get header for tip of chain: %v", err)
	}
	if latestBlockHeader == nil {
		t.Errorf("received a nil block header")
	} else if latestBlockHeader.Number.Uint64() != uint64(0) {
		t.Errorf("expected block header number 0, instead got %v", latestBlockHeader.Number.Uint64())
	}

	sim.Commit()

	latestBlockHeader, err = sim.HeaderByNumber(bgCtx, nil)
	if err != nil {
		t.Errorf("could not get header for blockheight of 1: %v", err)
	}

	blockHeader, err := sim.HeaderByNumber(bgCtx, big.NewInt(1))
	if err != nil {
		t.Errorf("could not get header for blockheight of 1: %v", err)
	}

	if blockHeader.Hash() != latestBlockHeader.Hash() {
		t.Errorf("block header and latest block header are not the same")
	}
	if blockHeader.Number.Int64() != int64(1) {
		t.Errorf("did not get blockheader for block 1. instead got block %v", blockHeader.Number.Int64())
	}

	block, err := sim.BlockByNumber(bgCtx, big.NewInt(1))
	if err != nil {
		t.Errorf("could not get block for blockheight of 1: %v", err)
	}

	if block.Hash() != blockHeader.Hash() {
		t.Errorf("block hash and block header hash do not match. expected %v, got %v", block.Hash(), blockHeader.Hash())
	}
}

func TestSimulatedBackend_TransactionCount(t *testing.T) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)

	sim := simTestBackend(t, testAddr)
	bgCtx := context.Background()
	currentBlock, err := sim.BlockByNumber(bgCtx, nil)
	if err != nil || currentBlock == nil {
		t.Error("could not get current block")
	}

	count, err := sim.TransactionCount(bgCtx, currentBlock.Hash())
	if err != nil {
		t.Error("could not get current block's transaction count")
	}

	if count != 0 {
		t.Errorf("expected transaction count of %v does not match actual count of %v", 0, count)
	}

	// create a signed transaction to send
	signer := types.MakeSigner(chain.TestChainConfig, 1, 0)
	var txn types.Transaction = types.NewTransaction(uint64(0), testAddr, uint256.NewInt(1000), params.TxGas, uint256.NewInt(1), nil)
	signedTx, err := types.SignTx(txn, *signer, testKey)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}

	// send txn to simulated backend
	err = sim.SendTransaction(bgCtx, signedTx)
	if err != nil {
		t.Errorf("could not add txn to pending block: %v", err)
	}

	sim.Commit()

	lastBlock, err := sim.BlockByNumber(bgCtx, nil)
	if err != nil {
		t.Errorf("could not get header for tip of chain: %v", err)
	}

	count, err = sim.TransactionCount(bgCtx, lastBlock.Hash())
	if err != nil {
		t.Error("could not get current block's transaction count")
	}

	if count != 1 {
		t.Errorf("expected transaction count of %v does not match actual count of %v", 1, count)
	}
}

func TestSimulatedBackend_TransactionInBlock(t *testing.T) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)

	sim := simTestBackend(t, testAddr)
	bgCtx := context.Background()

	transaction, err := sim.TransactionInBlock(bgCtx, sim.pendingBlock.Hash(), uint(0))
	if err == nil && err != errTransactionDoesNotExist {
		t.Errorf("expected a transaction does not exist error to be received but received %v", err)
	}
	if transaction != nil {
		t.Errorf("expected transaction to be nil but received %v", transaction)
	}

	// expect pending nonce to be 0 since account has not been used
	pendingNonce, err := sim.PendingNonceAt(bgCtx, testAddr)
	if err != nil {
		t.Errorf("did not get the pending nonce: %v", err)
	}

	if pendingNonce != uint64(0) {
		t.Errorf("expected pending nonce of 0 got %v", pendingNonce)
	}

	// create a signed transaction to send
	signer := types.MakeSigner(chain.TestChainConfig, 1, 0)
	var txn types.Transaction = types.NewTransaction(uint64(0), testAddr, uint256.NewInt(1000), params.TxGas, uint256.NewInt(1), nil)
	signedTx, err := types.SignTx(txn, *signer, testKey)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}

	// send txn to simulated backend
	err = sim.SendTransaction(bgCtx, signedTx)
	if err != nil {
		t.Errorf("could not add txn to pending block: %v", err)
	}

	sim.Commit()

	lastBlock, err := sim.BlockByNumber(bgCtx, nil)
	if err != nil {
		t.Errorf("could not get header for tip of chain: %v", err)
	}

	transaction, err = sim.TransactionInBlock(bgCtx, lastBlock.Hash(), uint(1))
	if err == nil && err != errTransactionDoesNotExist {
		t.Errorf("expected a transaction does not exist error to be received but received %v", err)
	}
	if transaction != nil {
		t.Errorf("expected transaction to be nil but received %v", transaction)
	}

	transaction, err = sim.TransactionInBlock(bgCtx, lastBlock.Hash(), uint(0))
	if err != nil {
		t.Errorf("could not get transaction in the lastest block with hash %v: %v", lastBlock.Hash().String(), err)
	}

	if signedTx.Hash().String() != transaction.Hash().String() {
		t.Errorf("received transaction that did not match the sent transaction. expected hash %v, got hash %v", signedTx.Hash().String(), transaction.Hash().String())
	}
}

func TestSimulatedBackend_PendingNonceAt(t *testing.T) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)

	sim := simTestBackend(t, testAddr)
	bgCtx := context.Background()

	// expect pending nonce to be 0 since account has not been used
	pendingNonce, err := sim.PendingNonceAt(bgCtx, testAddr)
	if err != nil {
		t.Errorf("did not get the pending nonce: %v", err)
	}

	if pendingNonce != uint64(0) {
		t.Errorf("expected pending nonce of 0 got %v", pendingNonce)
	}

	// create a signed transaction to send
	signer := types.MakeSigner(chain.TestChainConfig, 1, 0)
	var txn types.Transaction = types.NewTransaction(uint64(0), testAddr, uint256.NewInt(1000), params.TxGas, uint256.NewInt(1), nil)
	signedTx, err := types.SignTx(txn, *signer, testKey)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}

	// send txn to simulated backend
	err = sim.SendTransaction(bgCtx, signedTx)
	if err != nil {
		t.Errorf("could not add txn to pending block: %v", err)
	}

	// expect pending nonce to be 1 since account has submitted one transaction
	pendingNonce, err = sim.PendingNonceAt(bgCtx, testAddr)
	if err != nil {
		t.Errorf("did not get the pending nonce: %v", err)
	}

	if pendingNonce != uint64(1) {
		t.Errorf("expected pending nonce of 1 got %v", pendingNonce)
	}

	// make a new transaction with a nonce of 1
	txn = types.NewTransaction(uint64(1), testAddr, uint256.NewInt(1000), params.TxGas, uint256.NewInt(1), nil)
	signedTx, err = types.SignTx(txn, *signer, testKey)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}
	err = sim.SendTransaction(bgCtx, signedTx)
	if err != nil {
		t.Errorf("could not send tx: %v", err)
	}

	// expect pending nonce to be 2 since account now has two transactions
	pendingNonce, err = sim.PendingNonceAt(bgCtx, testAddr)
	if err != nil {
		t.Errorf("did not get the pending nonce: %v", err)
	}

	if pendingNonce != uint64(2) {
		t.Errorf("expected pending nonce of 2 got %v", pendingNonce)
	}
}

func TestSimulatedBackend_TransactionReceipt(t *testing.T) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)

	sim := simTestBackend(t, testAddr)
	bgCtx := context.Background()

	// create a signed transaction to send
	signer := types.MakeSigner(chain.TestChainConfig, 1, 0)
	var txn types.Transaction = types.NewTransaction(uint64(0), testAddr, uint256.NewInt(1000), params.TxGas, uint256.NewInt(1), nil)
	signedTx, err := types.SignTx(txn, *signer, testKey)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}

	// send txn to simulated backend
	err = sim.SendTransaction(bgCtx, signedTx)
	if err != nil {
		t.Errorf("could not add txn to pending block: %v", err)
	}
	sim.Commit()

	receipt, err := sim.TransactionReceipt(bgCtx, signedTx.Hash())
	if err != nil {
		t.Errorf("could not get transaction receipt: %v", err)
	}

	if receipt.ContractAddress != testAddr && receipt.TxHash != signedTx.Hash() {
		t.Errorf("received receipt is not correct: %v", receipt)
	}
}

func TestSimulatedBackend_SuggestGasPrice(t *testing.T) {
	sim := NewSimulatedBackend(t,
		types.GenesisAlloc{},
		10000000,
	)
	bgCtx := context.Background()
	gasPrice, err := sim.SuggestGasPrice(bgCtx)
	if err != nil {
		t.Errorf("could not get gas price: %v", err)
	}
	if gasPrice.Uint64() != uint64(1) {
		t.Errorf("gas price was not expected value of 1. actual: %v", gasPrice.Uint64())
	}
}

func TestSimulatedBackend_PendingCodeAt(t *testing.T) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)
	sim := simTestBackend(t, testAddr)
	bgCtx := context.Background()
	code, err := sim.CodeAt(bgCtx, testAddr, nil)
	if err != nil {
		t.Errorf("could not get code at test addr: %v", err)
	}
	if len(code) != 0 {
		t.Errorf("got code for account that does not have contract code")
	}

	parsed, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Errorf("could not get code at test addr: %v", err)
	}
	auth, _ := bind.NewKeyedTransactorWithChainID(testKey, big.NewInt(1337))
	contractAddr, txn, contract, err := bind.DeployContract(auth, parsed, common.FromHex(abiBin), sim)
	if err != nil {
		t.Errorf("could not deploy contract: %v tx: %v contract: %v", err, txn, contract)
	}

	code, err = sim.PendingCodeAt(bgCtx, contractAddr)
	if err != nil {
		t.Errorf("could not get code at test addr: %v", err)
	}
	if len(code) == 0 {
		t.Errorf("did not get code for account that has contract code")
	}
	// ensure code received equals code deployed
	if !bytes.Equal(code, common.FromHex(deployedCode)) {
		t.Errorf("code received did not match expected deployed code:\n expected %v\n actual %v", common.FromHex(deployedCode), code)
	}
}

func TestSimulatedBackend_CodeAt(t *testing.T) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)
	sim := simTestBackend(t, testAddr)
	bgCtx := context.Background()
	code, err := sim.CodeAt(bgCtx, testAddr, nil)
	if err != nil {
		t.Errorf("could not get code at test addr: %v", err)
	}
	if len(code) != 0 {
		t.Errorf("got code for account that does not have contract code")
	}

	parsed, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Errorf("could not get code at test addr: %v", err)
	}
	auth, _ := bind.NewKeyedTransactorWithChainID(testKey, big.NewInt(1337))
	contractAddr, txn, contract, err := bind.DeployContract(auth, parsed, common.FromHex(abiBin), sim)
	if err != nil {
		t.Errorf("could not deploy contract: %v tx: %v contract: %v", err, txn, contract)
	}

	sim.Commit()
	code, err = sim.CodeAt(bgCtx, contractAddr, nil)
	if err != nil {
		t.Errorf("could not get code at test addr: %v", err)
	}
	if len(code) == 0 {
		t.Errorf("did not get code for account that has contract code")
	}
	// ensure code received equals code deployed
	if !bytes.Equal(code, common.FromHex(deployedCode)) {
		t.Errorf("code received did not match expected deployed code:\n expected %v\n actual %v", common.FromHex(deployedCode), code)
	}
}

// When receive("X") is called with sender 0x00... and value 1, it produces this txn receipt:
//
//	receipt{status=1 cgas=23949 bloom=00000000004000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000040200000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000 logs=[log: b6818c8064f645cd82d99b59a1a267d6d61117ef [75fd880d39c1daf53b6547ab6cb59451fc6452d27caa90e5b6649dd8293b9eed] 000000000000000000000000376c47978271565f56deb45495afa69e59c16ab200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000158 9ae378b6d4409eada347a5dc0c180f186cb62dc68fcc0f043425eb917335aa28 0 95d429d309bb9d753954195fe2d69bd140b4ae731b9b5b605c34323de162cf00 0]}
func TestSimulatedBackend_PendingAndCallContract(t *testing.T) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)
	sim := simTestBackend(t, testAddr)
	bgCtx := context.Background()

	parsed, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Errorf("could not get code at test addr: %v", err)
	}
	contractAuth, _ := bind.NewKeyedTransactorWithChainID(testKey, big.NewInt(1337))
	addr, _, _, err := bind.DeployContract(contractAuth, parsed, common.FromHex(abiBin), sim)
	if err != nil {
		t.Errorf("could not deploy contract: %v", err)
	}

	input, err := parsed.Pack("receive", []byte("X"))
	if err != nil {
		t.Errorf("could not pack receive function on contract: %v", err)
	}

	// make sure you can call the contract in pending state
	res, err := sim.PendingCallContract(bgCtx, ethereum.CallMsg{
		From: testAddr,
		To:   &addr,
		Data: input,
	})
	if err != nil {
		t.Errorf("could not call receive method on contract: %v", err)
	}
	if len(res) == 0 {
		t.Errorf("result of contract call was empty: %v", res)
	}

	// while comparing against the byte array is more exact, also compare against the human readable string for readability
	if !bytes.Equal(res, expectedReturn) || !strings.Contains(string(res), "hello world") {
		t.Errorf("response from calling contract was expected to be 'hello world' instead received %v", string(res))
	}

	sim.Commit()

	// make sure you can call the contract
	res, err = sim.CallContract(bgCtx, ethereum.CallMsg{
		From: testAddr,
		To:   &addr,
		Data: input,
	}, nil)
	if err != nil {
		t.Errorf("could not call receive method on contract: %v", err)
	}
	if len(res) == 0 {
		t.Errorf("result of contract call was empty: %v", res)
	}

	if !bytes.Equal(res, expectedReturn) || !strings.Contains(string(res), "hello world") {
		t.Errorf("response from calling contract was expected to be 'hello world' instead received %v", string(res))
	}
}

// This test is based on the following contract:
/*
contract Reverter {
    function revertString() public pure{
        require(false, "some error");
    }
    function revertNoString() public pure {
        require(false, "");
    }
    function revertASM() public pure {
        assembly {
            revert(0x0, 0x0)
        }
    }
    function noRevert() public pure {
        assembly {
            // Assembles something that looks like require(false, "some error") but is not reverted
            mstore(0x0, 0x08c379a000000000000000000000000000000000000000000000000000000000)
            mstore(0x4, 0x0000000000000000000000000000000000000000000000000000000000000020)
            mstore(0x24, 0x000000000000000000000000000000000000000000000000000000000000000a)
            mstore(0x44, 0x736f6d65206572726f7200000000000000000000000000000000000000000000)
            return(0x0, 0x64)
        }
    }
}*/
func TestSimulatedBackend_CallContractRevert(t *testing.T) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)
	sim := simTestBackend(t, testAddr)
	bgCtx := context.Background()

	reverterABI := `[{"inputs": [],"name": "noRevert","outputs": [],"stateMutability": "pure","type": "function"},{"inputs": [],"name": "revertASM","outputs": [],"stateMutability": "pure","type": "function"},{"inputs": [],"name": "revertNoString","outputs": [],"stateMutability": "pure","type": "function"},{"inputs": [],"name": "revertString","outputs": [],"stateMutability": "pure","type": "function"}]`
	reverterBin := "608060405234801561001057600080fd5b506101d3806100206000396000f3fe608060405234801561001057600080fd5b506004361061004c5760003560e01c80634b409e01146100515780639b340e361461005b5780639bd6103714610065578063b7246fc11461006f575b600080fd5b610059610079565b005b6100636100ca565b005b61006d6100cf565b005b610077610145565b005b60006100c8576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526000815260200160200191505060405180910390fd5b565b600080fd5b6000610143576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252600a8152602001807f736f6d65206572726f720000000000000000000000000000000000000000000081525060200191505060405180910390fd5b565b7f08c379a0000000000000000000000000000000000000000000000000000000006000526020600452600a6024527f736f6d65206572726f720000000000000000000000000000000000000000000060445260646000f3fea2646970667358221220cdd8af0609ec4996b7360c7c780bad5c735740c64b1fffc3445aa12d37f07cb164736f6c63430006070033"

	parsed, err := abi.JSON(strings.NewReader(reverterABI))
	if err != nil {
		t.Errorf("could not get code at test addr: %v", err)
	}
	contractAuth, _ := bind.NewKeyedTransactorWithChainID(testKey, big.NewInt(1337))
	addr, _, _, err := bind.DeployContract(contractAuth, parsed, common.FromHex(reverterBin), sim)
	if err != nil {
		t.Errorf("could not deploy contract: %v", err)
	}

	inputs := make(map[string]interface{}, 3)
	inputs["revertASM"] = nil
	inputs["revertNoString"] = ""
	inputs["revertString"] = "some error"

	call := make([]func([]byte) ([]byte, error), 2)
	call[0] = func(input []byte) ([]byte, error) {
		return sim.PendingCallContract(bgCtx, ethereum.CallMsg{
			From: testAddr,
			To:   &addr,
			Data: input,
		})
	}
	call[1] = func(input []byte) ([]byte, error) {
		return sim.CallContract(bgCtx, ethereum.CallMsg{
			From: testAddr,
			To:   &addr,
			Data: input,
		}, nil)
	}

	// Run pending calls then commit
	for _, cl := range call {
		for key, val := range inputs {
			input, err := parsed.Pack(key)
			if err != nil {
				t.Errorf("could not pack %v function on contract: %v", key, err)
			}

			res, err := cl(input)
			if err == nil {
				t.Errorf("call to %v was not reverted", key)
			}
			if res != nil {
				t.Errorf("result from %v was not nil: %v", key, res)
			}
			if val != nil {
				rerr, ok := err.(*revertError)
				if !ok {
					t.Errorf("expect revert error")
				}
				if rerr.Error() != "execution reverted: "+val.(string) {
					t.Errorf("error was malformed: got %v want %v", rerr.Error(), val)
				}
			} else {
				// revert(0x0,0x0)
				if err.Error() != "execution reverted" {
					t.Errorf("error was malformed: got %v want %v", err, "execution reverted")
				}
			}
		}
		input, err := parsed.Pack("noRevert")
		if err != nil {
			t.Errorf("could not pack noRevert function on contract: %v", err)
		}
		res, err := cl(input)
		if err != nil {
			t.Error("call to noRevert was reverted")
		}
		if res == nil {
			t.Errorf("result from noRevert was nil")
		}
		sim.Commit()
	}
}

func TestNewSimulatedBackend_AdjustTimeFailWithPostValidationSkip(t *testing.T) {
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)
	sim := simTestBackend(t, testAddr)
	// Create txn and send
	amount, _ := uint256.FromBig(big.NewInt(1000))
	gasPrice, _ := uint256.FromBig(big.NewInt(1))
	signer := types.MakeSigner(chain.TestChainConfig, 1, 0)
	var txn types.Transaction = types.NewTransaction(0, testAddr, amount, params.TxGas, gasPrice, nil)
	signedTx, err := types.SignTx(txn, *signer, testKey)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}
	sim.SendTransaction(context.Background(), signedTx) //nolint:errcheck
	// AdjustTime should fail on non-empty block
	if err = sim.AdjustTime(time.Second); err == nil {
		t.Error("Expected adjust time to error on non-empty block")
	}
	sim.Commit()

	prevTime := sim.pendingBlock.Time()
	if err = sim.AdjustTime(time.Minute); err != nil {
		t.Error(err)
	}
	newTime := sim.pendingBlock.Time()
	if newTime-prevTime != uint64(time.Minute.Seconds()) {
		t.Errorf("adjusted time not equal to a minute. prev: %v, new: %v", prevTime, newTime)
	}
	// Put a transaction after adjusting time
	amount2, _ := uint256.FromBig(big.NewInt(1000))
	gasPrice2, _ := uint256.FromBig(big.NewInt(1))
	var tx2 types.Transaction = types.NewTransaction(1, testAddr, amount2, params.TxGas, gasPrice2, nil)
	signedTx2, err := types.SignTx(tx2, *signer, testKey)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}
	sim.SendTransaction(context.Background(), signedTx2) //nolint:errcheck
	sim.Commit()
	newTime = sim.pendingBlock.Time()
	if newTime-prevTime >= uint64(time.Minute.Seconds()) {
		t.Errorf("time adjusted, but shouldn't be: prev: %v, new: %v", prevTime, newTime)
	}
	txdb, err := sim.DB().BeginRw(sim.m.Ctx)
	require.NoError(t, err)
	defer txdb.Rollback()
	// Set this artifically to make sure the we do skip post validation
	var k [8]byte
	var v [8]byte
	binary.BigEndian.PutUint64(k[:], 1)
	binary.BigEndian.PutUint64(v[:], 0)
	require.NoError(t, txdb.Put(kv.MaxTxNum, k[:], v[:]))
	require.NoError(t, txdb.Commit())
	// Put a transaction after adjusting time
	amount3, _ := uint256.FromBig(big.NewInt(1000))
	gasPrice3, _ := uint256.FromBig(big.NewInt(1))
	var tx3 types.Transaction = types.NewTransaction(2, testAddr, amount3, params.TxGas, gasPrice3, nil)
	signedTx3, err := types.SignTx(tx3, *signer, testKey)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}
	sim.SendTransaction(context.Background(), signedTx3) //nolint:errcheck
	sim.Commit()
	newTime = sim.pendingBlock.Time()
	if newTime-prevTime >= uint64(time.Minute.Seconds()) {
		t.Errorf("time adjusted, but shouldn't be: prev: %v, new: %v", prevTime, newTime)
	}
}
