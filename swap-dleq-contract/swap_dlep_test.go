package swapdleq

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"os/exec"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"

	"github.com/noot/atomic-swap/common"
	"github.com/noot/atomic-swap/monero"
)

func reverse(s []byte) []byte {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

func setBigIntLE(s []byte) *big.Int {
	s = reverse(s)
	return big.NewInt(0).SetBytes(s)
}

func TestDeploySwapDLEQ(t *testing.T) {
	conn, err := ethclient.Dial("http://127.0.0.1:8545")
	require.NoError(t, err)

	pk_a, err := crypto.HexToECDSA(common.DefaultPrivKeyAlice)
	require.NoError(t, err)

	pk_b, err := crypto.HexToECDSA(common.DefaultPrivKeyBob)
	require.NoError(t, err)

	authAlice, err := bind.NewKeyedTransactorWithChainID(pk_a, big.NewInt(common.GanacheChainID))
	require.NoError(t, err)

	address, tx, swapContract, err := DeploySwapDLEQ(authAlice, conn, pk_a.X, pk_a.Y, pk_b.X, pk_b.Y)
	require.NoError(t, err)

	t.Log(address)
	t.Log(tx)
	t.Log(swapContract)
}

func TestSwap_Claim(t *testing.T) {
	if testing.Short() {
		t.Skip() // TODO: relies on ../target/debug/dleq-gen binary
	}

	// Alice generates key
	keyPairAlice, err := monero.GenerateKeys()
	require.NoError(t, err)
	// pubKeyAlice := keyPairAlice.PublicKeyPair().SpendKey().Bytes()
	pubKeyAliceX, pubKeyAliceY := monero.PublicSpendOnSecp256k1(keyPairAlice.SpendKeyBytes())
	fmt.Println("pubKeyAliceX: ", hex.EncodeToString(pubKeyAliceX.Bytes()), "pubKeyAliceY: ", hex.EncodeToString(pubKeyAliceY.Bytes()))

	// Alice generates DLEQ proof - this binary file is transmitted to Bob via some offchain protocol
	fmt.Println("kAlice: ", keyPairAlice.SpendKeyBytes())
	kAliceHex := hex.EncodeToString(keyPairAlice.SpendKeyBytes())
	fmt.Println("kAliceHex: ", kAliceHex)
	out, err := exec.Command("../target/debug/dleq-gen", kAliceHex, "../dleq-file-exchange/dleq_proof_alice.dat").Output()
	require.NoError(t, err)
	fmt.Printf("%s\n", out)

	// keyAlease, err := hex.DecodeString("e32ad36ce8e59156aa416da9c9f41a7abc59f6b5f1dd1c2079e8ff4e14503090")
	keyAlease, err := hex.DecodeString("a6e51afb9662bf2173d807ceaf88938d09a82d1ab2cea3eeb1706eeeb8b6ba03")
	require.NoError(t, err)

	pubKeyAleaseX, pubKeyAleaseY := monero.PublicSpendOnSecp256k1(keyAlease)

	fmt.Println("PubKey:", hex.EncodeToString(reverse(pubKeyAleaseX.Bytes())), hex.EncodeToString(reverse(pubKeyAleaseY.Bytes())))

	// Bob generates key
	keyPairBob, err := monero.GenerateKeys()
	require.NoError(t, err)
	secretBob := keyPairBob.SpendKeyBytes()
	pubKeyBobX, pubKeyBobY := monero.PublicSpendOnSecp256k1(secretBob)
	fmt.Println("pubKeyBobX: ", hex.EncodeToString(pubKeyBobX.Bytes()), "pubKeyBobY: ", hex.EncodeToString(pubKeyBobY.Bytes()))
	// pubKeyBob := keyPairBob.PublicKeyPair().SpendKey().Bytes()

	kBobHex := hex.EncodeToString(keyPairBob.SpendKeyBytes())
	fmt.Println("kBobHex: ", kBobHex)
	out, err = exec.Command("../target/debug/dleq-gen", kBobHex, "../dleq-file-exchange/dleq_proof_bob.dat").Output()
	require.NoError(t, err)
	fmt.Println("dleq-gen out: ", (string(out)))

	// exchange dleq-proofs

	// alice verifies bob's dleq
	out, err = exec.Command("../target/debug/dleq-verify", "../dleq-file-exchange/dleq_proof_bob.dat").Output()
	// if err != nil {
	// 	fmt.Println("proof verification failed:", err)
	// }
	require.NoError(t, err)
	fmt.Printf("%s\n", out)
	BobStrings := strings.Fields(string(out))
	fmt.Println("BobStrings: ", BobStrings)
	secp256k1BobX, err := hex.DecodeString(BobStrings[1])
	require.NoError(t, err)
	secp256k1BobY, err := hex.DecodeString(BobStrings[2])
	require.NoError(t, err)
	fmt.Println("BobParsed: ", secp256k1BobX, secp256k1BobY)

	// bob verifies alice's dleq
	out, err = exec.Command("../target/debug/dleq-verify", "../dleq-file-exchange/dleq_proof_alice.dat").Output()
	require.NoError(t, err)
	fmt.Printf("%s\n", out)
	AliceStrings := strings.Fields(string(out))
	fmt.Println("AliceStrings: ", AliceStrings)
	secp256k1AliceX, err := hex.DecodeString(AliceStrings[1])
	require.NoError(t, err)
	secp256k1AliceY, err := hex.DecodeString(AliceStrings[2])
	require.NoError(t, err)
	fmt.Println("AliceParsed: ", secp256k1AliceX, secp256k1AliceY)

	// setup
	conn, err := ethclient.Dial("ws://127.0.0.1:8545")
	require.NoError(t, err)

	pk_a, err := crypto.HexToECDSA(common.DefaultPrivKeyAlice)
	require.NoError(t, err)
	pk_b, err := crypto.HexToECDSA(common.DefaultPrivKeyBob)
	require.NoError(t, err)

	authAlice, err := bind.NewKeyedTransactorWithChainID(pk_a, big.NewInt(common.GanacheChainID))
	authAlice.Value = big.NewInt(10)
	require.NoError(t, err)
	authBob, err := bind.NewKeyedTransactorWithChainID(pk_b, big.NewInt(common.GanacheChainID))
	require.NoError(t, err)

	aliceBalanceBefore, err := conn.BalanceAt(context.Background(), authAlice.From, nil)
	require.NoError(t, err)
	fmt.Println("AliceBalanceBefore: ", aliceBalanceBefore)

	// check whether Bob had nothing before the Tx
	bobBalanceBefore, err := conn.BalanceAt(context.Background(), authBob.From, nil)
	require.NoError(t, err)
	fmt.Println("BobBalanceBefore: ", bobBalanceBefore)

	var pkAliceFixedX [32]byte
	// copy(pkAliceFixedX[:], reverse(pubKeyAliceX.Bytes()))
	copy(pkAliceFixedX[:], reverse(secp256k1AliceX))
	var pkAliceFixedY [32]byte
	// copy(pkAliceFixedY[:], reverse(pubKeyAliceY.Bytes()))
	copy(pkAliceFixedY[:], reverse(secp256k1AliceY))
	var pkBobFixedX [32]byte
	// copy(pkBobFixedX[:], reverse(pubKeyBobX.Bytes()))
	copy(pkBobFixedX[:], reverse(secp256k1BobX))
	var pkBobFixedY [32]byte
	// copy(pkBobFixedY[:], reverse(pubKeyBobY.Bytes()))
	copy(pkBobFixedY[:], reverse(secp256k1BobY))

	contractAddress, deployTx, swap, err := DeploySwapDLEQ(authAlice, conn, setBigIntLE(pkBobFixedX[:]), setBigIntLE(pkBobFixedY[:]), setBigIntLE(pkAliceFixedX[:]), setBigIntLE(pkAliceFixedY[:]))
	require.NoError(t, err)
	fmt.Println("Deploy Tx Gas Cost:", deployTx.Gas())

	aliceBalanceAfter, err := conn.BalanceAt(context.Background(), authAlice.From, nil)
	require.NoError(t, err)
	fmt.Println("AliceBalanceAfter: ", aliceBalanceAfter)

	contractBalance, err := conn.BalanceAt(context.Background(), contractAddress, nil)
	require.NoError(t, err)
	require.Equal(t, contractBalance, big.NewInt(10))

	txOpts := &bind.TransactOpts{
		From:   authAlice.From,
		Signer: authAlice.Signer,
	}

	txOptsBob := &bind.TransactOpts{
		From:   authBob.From,
		Signer: authBob.Signer,
	}

	// Bob tries to claim before Alice has called ready, should fail
	s := big.NewInt(0).SetBytes(reverse(secretBob))
	fmt.Println("Secret:", hex.EncodeToString(reverse(secretBob)))
	fmt.Println("PubKey:", hex.EncodeToString(reverse(pubKeyBobX.Bytes())), hex.EncodeToString(reverse(pubKeyBobY.Bytes())))
	_, err = swap.Claim(txOptsBob, s)
	require.Regexp(t, ".*'isReady == false' cannot claim yet!", err)

	// Alice calls set_ready on the contract
	setReadyTx, err := swap.SetReady(txOpts)
	fmt.Println("setReady Tx Gas Cost:", setReadyTx.Gas())
	require.NoError(t, err)

	// The main transaction that we're testing. Should work
	tx, err := swap.Claim(txOptsBob, s)
	require.NoError(t, err)

	// The Swap contract has self destructed: should have no balance AND no bytecode at the address
	contractBalance, err = conn.BalanceAt(context.Background(), contractAddress, nil)
	require.NoError(t, err)
	require.Equal(t, contractBalance.Uint64(), big.NewInt(0).Uint64())
	bytecode, err := conn.CodeAt(context.Background(), contractAddress, nil) // nil is latest block
	require.NoError(t, err)
	require.Empty(t, bytecode)

	fmt.Println("Tx details are:", tx.Gas())

	// check whether Bob's account balance has increased now
	// bobBalanceAfter, err := conn.BalanceAt(context.Background(), authBob.From, nil)
	// fmt.Println("BobBalanceBefore: ", bobBalanceAfter)
	// require.NoError(t, err)
	// require.Equal(t, big.NewInt(10), big.NewInt(0).Sub(bobBalanceAfter, bobBalanceBefore))
}

func TestSwap_Refund_Within_T0(t *testing.T) {
	if testing.Short() {
		t.Skip() // TODO: relies on ../target/debug/dleq-gen binary
	}

	// Alice generates key
	keyPairAlice, err := monero.GenerateKeys()
	secretAlice := keyPairAlice.SpendKeyBytes()
	require.NoError(t, err)
	// pubKeyAlice := keyPairAlice.PublicKeyPair().SpendKey().Bytes()
	pubKeyAliceX, pubKeyAliceY := monero.PublicSpendOnSecp256k1(keyPairAlice.SpendKeyBytes())
	fmt.Println("pubKeyAliceX: ", hex.EncodeToString(pubKeyAliceX.Bytes()), "pubKeyAliceY: ", hex.EncodeToString(pubKeyAliceY.Bytes()))

	// Alice generates DLEQ proof - this binary file is transmitted to Bob via some offchain protocol
	fmt.Println("kAlice: ", keyPairAlice.SpendKeyBytes())
	kAliceHex := hex.EncodeToString(keyPairAlice.SpendKeyBytes())
	fmt.Println("kAliceHex: ", kAliceHex)
	out, err := exec.Command("../target/debug/dleq-gen", kAliceHex, "../dleq-file-exchange/dleq_proof_alice.dat").Output()
	require.NoError(t, err)
	fmt.Printf("%s\n", out)

	// keyAlease, err := hex.DecodeString("e32ad36ce8e59156aa416da9c9f41a7abc59f6b5f1dd1c2079e8ff4e14503090")
	keyAlease, err := hex.DecodeString("a6e51afb9662bf2173d807ceaf88938d09a82d1ab2cea3eeb1706eeeb8b6ba03")
	require.NoError(t, err)

	pubKeyAleaseX, pubKeyAleaseY := monero.PublicSpendOnSecp256k1(keyAlease)

	fmt.Println("PubKey:", hex.EncodeToString(reverse(pubKeyAleaseX.Bytes())), hex.EncodeToString(reverse(pubKeyAleaseY.Bytes())))

	// Bob generates key
	keyPairBob, err := monero.GenerateKeys()
	require.NoError(t, err)
	secretBob := keyPairBob.SpendKeyBytes()
	pubKeyBobX, pubKeyBobY := monero.PublicSpendOnSecp256k1(secretBob)
	fmt.Println("pubKeyBobX: ", hex.EncodeToString(pubKeyBobX.Bytes()), "pubKeyBobY: ", hex.EncodeToString(pubKeyBobY.Bytes()))
	// pubKeyBob := keyPairBob.PublicKeyPair().SpendKey().Bytes()

	kBobHex := hex.EncodeToString(keyPairBob.SpendKeyBytes())
	fmt.Println("kBobHex: ", kBobHex)
	out, err = exec.Command("../target/debug/dleq-gen", kBobHex, "../dleq-file-exchange/dleq_proof_bob.dat").Output()
	require.NoError(t, err)
	fmt.Println("dleq-gen out: ", (string(out)))

	// exchange dleq-proofs

	// alice verifies bob's dleq
	out, err = exec.Command("../target/debug/dleq-verify", "../dleq-file-exchange/dleq_proof_bob.dat").Output()
	// if err != nil {
	// 	fmt.Println("proof verification failed:", err)
	// }
	require.NoError(t, err)
	fmt.Printf("%s\n", out)
	BobStrings := strings.Fields(string(out))
	fmt.Println("BobStrings: ", BobStrings)
	secp256k1BobX, err := hex.DecodeString(BobStrings[1])
	require.NoError(t, err)
	secp256k1BobY, err := hex.DecodeString(BobStrings[2])
	require.NoError(t, err)
	fmt.Println("BobParsed: ", secp256k1BobX, secp256k1BobY)

	// bob verifies alice's dleq
	out, err = exec.Command("../target/debug/dleq-verify", "../dleq-file-exchange/dleq_proof_alice.dat").Output()
	require.NoError(t, err)
	fmt.Printf("%s\n", out)
	AliceStrings := strings.Fields(string(out))
	fmt.Println("AliceStrings: ", AliceStrings)
	secp256k1AliceX, err := hex.DecodeString(AliceStrings[1])
	require.NoError(t, err)
	secp256k1AliceY, err := hex.DecodeString(AliceStrings[2])
	require.NoError(t, err)
	fmt.Println("AliceParsed: ", secp256k1AliceX, secp256k1AliceY)

	// setup
	conn, err := ethclient.Dial("ws://127.0.0.1:8545")
	require.NoError(t, err)

	pk_a, err := crypto.HexToECDSA(common.DefaultPrivKeyAlice)
	require.NoError(t, err)

	authAlice, err := bind.NewKeyedTransactorWithChainID(pk_a, big.NewInt(common.GanacheChainID))
	require.NoError(t, err)
	authAlice.Value = big.NewInt(10)

	aliceBalanceBefore, err := conn.BalanceAt(context.Background(), authAlice.From, nil)
	require.NoError(t, err)
	fmt.Println("AliceBalanceBefore: ", aliceBalanceBefore)

	var pkAliceFixedX [32]byte
	// copy(pkAliceFixedX[:], reverse(pubKeyAliceX.Bytes()))
	copy(pkAliceFixedX[:], reverse(secp256k1AliceX))
	var pkAliceFixedY [32]byte
	// copy(pkAliceFixedY[:], reverse(pubKeyAliceY.Bytes()))
	copy(pkAliceFixedY[:], reverse(secp256k1AliceY))
	var pkBobFixedX [32]byte
	// copy(pkBobFixedX[:], reverse(pubKeyBobX.Bytes()))
	copy(pkBobFixedX[:], reverse(secp256k1BobX))
	var pkBobFixedY [32]byte
	// copy(pkBobFixedY[:], reverse(pubKeyBobY.Bytes()))
	copy(pkBobFixedY[:], reverse(secp256k1BobY))
	contractAddress, deployTx, swap, err := DeploySwapDLEQ(authAlice, conn, setBigIntLE(pkBobFixedX[:]), setBigIntLE(pkBobFixedY[:]), setBigIntLE(pkAliceFixedX[:]), setBigIntLE(pkAliceFixedY[:]))
	require.NoError(t, err)
	fmt.Println("Deploy Tx Gas Cost:", deployTx.Gas())

	aliceBalanceAfter, err := conn.BalanceAt(context.Background(), authAlice.From, nil)
	require.NoError(t, err)
	fmt.Println("AliceBalanceAfter: ", aliceBalanceAfter)

	txOpts := &bind.TransactOpts{
		From:   authAlice.From,
		Signer: authAlice.Signer,
	}

	// Alice never calls set_ready on the contract, instead she just tries to Refund immidiately
	s := big.NewInt(0).SetBytes(reverse(secretAlice))
	_, err = swap.Refund(txOpts, s)
	require.NoError(t, err)

	// The Swap contract has self destructed: should have no balance AND no bytecode at the address
	contractBalance, err := conn.BalanceAt(context.Background(), contractAddress, nil)
	require.NoError(t, err)
	require.Equal(t, contractBalance.Uint64(), big.NewInt(0).Uint64())

	// bytecode, err := conn.CodeAt(context.Background(), contractAddress, nil) // nil is latest block
	// require.NoError(t, err)
	// require.Empty(t, bytecode)

	// require.Equal(t, big.NewInt(0).Sub(bobBalanceAfter, bobBalanceBefore), 10)
}

func TestSwap_Refund_After_T1(t *testing.T) {
	if testing.Short() {
		t.Skip() // TODO: relies on ../target/debug/dleq-gen binary
	}

	// Alice generates key
	keyPairAlice, err := monero.GenerateKeys()
	secretAlice := keyPairAlice.SpendKeyBytes()
	require.NoError(t, err)
	// pubKeyAlice := keyPairAlice.PublicKeyPair().SpendKey().Bytes()
	pubKeyAliceX, pubKeyAliceY := monero.PublicSpendOnSecp256k1(keyPairAlice.SpendKeyBytes())
	fmt.Println("pubKeyAliceX: ", hex.EncodeToString(pubKeyAliceX.Bytes()), "pubKeyAliceY: ", hex.EncodeToString(pubKeyAliceY.Bytes()))

	// Alice generates DLEQ proof - this binary file is transmitted to Bob via some offchain protocol
	fmt.Println("kAlice: ", keyPairAlice.SpendKeyBytes())
	kAliceHex := hex.EncodeToString(keyPairAlice.SpendKeyBytes())
	fmt.Println("kAliceHex: ", kAliceHex)
	out, err := exec.Command("../target/debug/dleq-gen", kAliceHex, "../dleq-file-exchange/dleq_proof_alice.dat").Output()
	require.NoError(t, err)
	fmt.Printf("%s\n", out)

	// keyAlease, err := hex.DecodeString("e32ad36ce8e59156aa416da9c9f41a7abc59f6b5f1dd1c2079e8ff4e14503090")
	keyAlease, err := hex.DecodeString("a6e51afb9662bf2173d807ceaf88938d09a82d1ab2cea3eeb1706eeeb8b6ba03")
	require.NoError(t, err)

	pubKeyAleaseX, pubKeyAleaseY := monero.PublicSpendOnSecp256k1(keyAlease)

	fmt.Println("PubKey:", hex.EncodeToString(reverse(pubKeyAleaseX.Bytes())), hex.EncodeToString(reverse(pubKeyAleaseY.Bytes())))

	// Bob generates key
	keyPairBob, err := monero.GenerateKeys()
	require.NoError(t, err)
	secretBob := keyPairBob.SpendKeyBytes()
	pubKeyBobX, pubKeyBobY := monero.PublicSpendOnSecp256k1(secretBob)
	fmt.Println("pubKeyBobX: ", hex.EncodeToString(pubKeyBobX.Bytes()), "pubKeyBobY: ", hex.EncodeToString(pubKeyBobY.Bytes()))
	// pubKeyBob := keyPairBob.PublicKeyPair().SpendKey().Bytes()

	kBobHex := hex.EncodeToString(keyPairBob.SpendKeyBytes())
	fmt.Println("kBobHex: ", kBobHex)
	out, err = exec.Command("../target/debug/dleq-gen", kBobHex, "../dleq-file-exchange/dleq_proof_bob.dat").Output()
	require.NoError(t, err)
	fmt.Println("dleq-gen out: ", (string(out)))

	// exchange dleq-proofs

	// alice verifies bob's dleq
	out, err = exec.Command("../target/debug/dleq-verify", "../dleq-file-exchange/dleq_proof_bob.dat").Output()
	// if err != nil {
	// 	fmt.Println("proof verification failed:", err)
	// }
	require.NoError(t, err)
	fmt.Printf("%s\n", out)
	BobStrings := strings.Fields(string(out))
	fmt.Println("BobStrings: ", BobStrings)
	secp256k1BobX, err := hex.DecodeString(BobStrings[1])
	require.NoError(t, err)
	secp256k1BobY, err := hex.DecodeString(BobStrings[2])
	require.NoError(t, err)
	fmt.Println("BobParsed: ", secp256k1BobX, secp256k1BobY)

	// bob verifies alice's dleq
	out, err = exec.Command("../target/debug/dleq-verify", "../dleq-file-exchange/dleq_proof_alice.dat").Output()
	require.NoError(t, err)
	fmt.Printf("%s\n", out)
	AliceStrings := strings.Fields(string(out))
	fmt.Println("AliceStrings: ", AliceStrings)
	secp256k1AliceX, err := hex.DecodeString(AliceStrings[1])
	require.NoError(t, err)
	secp256k1AliceY, err := hex.DecodeString(AliceStrings[2])
	require.NoError(t, err)
	fmt.Println("AliceParsed: ", secp256k1AliceX, secp256k1AliceY)

	// setup
	conn, err := ethclient.Dial("ws://127.0.0.1:8545")
	require.NoError(t, err)

	pk_a, err := crypto.HexToECDSA(common.DefaultPrivKeyAlice)
	require.NoError(t, err)

	authAlice, err := bind.NewKeyedTransactorWithChainID(pk_a, big.NewInt(common.GanacheChainID))
	require.NoError(t, err)
	authAlice.Value = big.NewInt(10)

	aliceBalanceBefore, err := conn.BalanceAt(context.Background(), authAlice.From, nil)
	require.NoError(t, err)
	fmt.Println("AliceBalanceBefore: ", aliceBalanceBefore)

	var pkAliceFixedX [32]byte
	// copy(pkAliceFixedX[:], reverse(pubKeyAliceX.Bytes()))
	copy(pkAliceFixedX[:], reverse(secp256k1AliceX))
	var pkAliceFixedY [32]byte
	// copy(pkAliceFixedY[:], reverse(pubKeyAliceY.Bytes()))
	copy(pkAliceFixedY[:], reverse(secp256k1AliceY))
	var pkBobFixedX [32]byte
	// copy(pkBobFixedX[:], reverse(pubKeyBobX.Bytes()))
	copy(pkBobFixedX[:], reverse(secp256k1BobX))
	var pkBobFixedY [32]byte
	// copy(pkBobFixedY[:], reverse(pubKeyBobY.Bytes()))
	copy(pkBobFixedY[:], reverse(secp256k1BobY))
	contractAddress, deployTx, swap, err := DeploySwapDLEQ(authAlice, conn, setBigIntLE(pkBobFixedX[:]), setBigIntLE(pkBobFixedY[:]), setBigIntLE(pkAliceFixedX[:]), setBigIntLE(pkAliceFixedY[:]))
	require.NoError(t, err)
	fmt.Println("Deploy Tx Gas Cost:", deployTx.Gas())

	aliceBalanceAfter, err := conn.BalanceAt(context.Background(), authAlice.From, nil)
	require.NoError(t, err)
	fmt.Println("AliceBalanceAfter: ", aliceBalanceAfter)
	txOpts := &bind.TransactOpts{
		From:   authAlice.From,
		Signer: authAlice.Signer,
	}

	// Alice calls set_ready on the contract, and immediately tries to Refund
	// After waiting T1, Alice should be able to refund now
	s := big.NewInt(0).SetBytes(reverse(secretAlice))
	_, err = swap.SetReady(txOpts)
	require.NoError(t, err)

	_, err = swap.Refund(txOpts, s)
	require.Regexp(t, ".*It's Bob's turn now, please wait!", err)

	// wait some, then try again
	var result int64
	rpcClient, err := rpc.Dial("http://127.0.0.1:8545")
	require.NoError(t, err)

	ret := rpcClient.Call(&result, "evm_increaseTime", 3600*25)
	require.NoError(t, ret)
	_, err = swap.Refund(txOpts, s)
	require.NoError(t, err)

	// The Swap contract has self destructed: should have no balance AND no bytecode at the address
	contractBalance, err := conn.BalanceAt(context.Background(), contractAddress, nil)
	require.NoError(t, err)
	require.Equal(t, contractBalance.Uint64(), big.NewInt(0).Uint64())

	bytecode, err := conn.CodeAt(context.Background(), contractAddress, nil) // nil is latest block
	require.NoError(t, err)
	require.Empty(t, bytecode)
}
