// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/hashkey-chain/hashkey-chain/common"
	"github.com/hashkey-chain/hashkey-chain/crypto"
	"github.com/hashkey-chain/hashkey-chain/params"
)

var (
	ErrInvalidChainId = errors.New("invalid chain id for signer")
)

// sigCache is used to cache the derived sender and contains
// the signer used to derive it.
type sigCache struct {
	signer Signer
	from   common.Address
}

// MakeSigner returns a Signer based on the given chain config and block number.
func MakeSigner(config *params.ChainConfig, pip7 bool) Signer {
	var signer Signer
	if pip7 {
		signer = NewPIP7Signer(config.ChainID, config.PIP7ChainID)
	} else {
		signer = NewEIP155Signer(config.ChainID)
	}
	return signer
}

// SignTx signs the transaction using the given signer and private key
func SignTx(tx *Transaction, s Signer, prv *ecdsa.PrivateKey) (*Transaction, error) {
	h := s.Hash(tx, nil)
	sig, err := crypto.Sign(h[:], prv)
	if err != nil {
		return nil, err
	}
	return tx.WithSignature(s, sig)
}

// Sender returns the address derived from the signature (V, R, S) using secp256k1
// elliptic curve and an error if it failed deriving or upon an incorrect
// signature.
//
// Sender may cache the address, allowing it to be used regardless of
// signing method. The cache is invalidated if the cached signer does
// not match the signer used in the current call.
func Sender(signer Signer, tx *Transaction) (common.Address, error) {
	if sc := tx.from.Load(); sc != nil {
		sigCache := sc.(sigCache)
		// If the signer used to derive from in a previous
		// call is not the same as used current, invalidate
		// the cache.
		if sigCache.signer.Equal(signer) {
			return sigCache.from, nil
		}
	}

	addr, err := signer.Sender(tx)
	if err != nil {
		return common.Address{}, err
	}
	tx.from.Store(sigCache{signer: signer, from: addr})
	return addr, nil
}

// Signer encapsulates transaction signature handling. Note that this interface is not a
// stable API and may change at any time to accommodate new protocol rules.
type Signer interface {
	// Sender returns the sender address of the transaction.
	Sender(tx *Transaction) (common.Address, error)
	// SignatureValues returns the raw R, S, V values corresponding to the
	// given signature.
	SignatureValues(sig []byte) (r, s, v *big.Int, err error)
	// Hash returns the hash to be signed.
	Hash(tx *Transaction, chainId *big.Int) common.Hash
	// Equal returns true if the given signer is the same as the receiver.
	Equal(Signer) bool
	// Signature return the sig info of the transaction
	//	SignatureAndSender(tx *Transaction) (common.Address, []byte, error)
}

// EIP155Transaction implements Signer using the EIP155 rules.
type EIP155Signer struct {
	chainId, chainIdMul *big.Int
}

func NewEIP155Signer(chainId *big.Int) EIP155Signer {
	if chainId == nil {
		chainId = new(big.Int)
	}
	return EIP155Signer{
		chainId:    chainId,
		chainIdMul: new(big.Int).Mul(chainId, big.NewInt(2)),
	}
}

func (s EIP155Signer) Equal(s2 Signer) bool {
	eip155, ok := s2.(EIP155Signer)
	return ok && eip155.chainId.Cmp(s.chainId) == 0
}

var big8 = big.NewInt(8)

func (s EIP155Signer) Sender(tx *Transaction) (common.Address, error) {
	txChainId := tx.ChainId()
	if txChainId.Cmp(s.chainId) != 0 {
		return common.Address{}, ErrInvalidChainId
	}
	V := new(big.Int).Sub(tx.data.V, s.chainIdMul)
	V.Sub(V, big8)
	return recoverPlain(s.Hash(tx, txChainId), tx.data.R, tx.data.S, V, true)
}

// SignatureValues returns the raw R, S, V values corresponding to the
// given signature.This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func (s EIP155Signer) SignatureValues(sig []byte) (R, S, V *big.Int, err error) {
	if len(sig) != crypto.SignatureLength {
		panic(fmt.Sprintf("wrong size for signature: got %d, want 65", len(sig)))
	}
	R = new(big.Int).SetBytes(sig[:32])
	S = new(big.Int).SetBytes(sig[32:64])
	V = new(big.Int).SetBytes([]byte{sig[64] + 35})
	V.Add(V, s.chainIdMul)

	return R, S, V, nil
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (s EIP155Signer) Hash(tx *Transaction, chainId *big.Int) common.Hash {
	cid := chainId
	if chainId == nil {
		cid = s.chainId
	}
	return rlpHash([]interface{}{
		tx.data.AccountNonce,
		tx.data.Price,
		tx.data.GasLimit,
		tx.data.Recipient,
		tx.data.Amount,
		tx.data.Payload,
		cid, uint(0), uint(0),
	})
}

func (s EIP155Signer) SignatureAndSender(tx *Transaction) (common.Address, []byte, error) {

	if tx.ChainId().Cmp(s.chainId) != 0 {
		return common.Address{}, []byte{}, ErrInvalidChainId
	}
	V := new(big.Int).Sub(tx.data.V, s.chainIdMul)
	V.Sub(V, big8)
	return recoverPubKeyAndSender(s.Hash(tx, s.chainId), tx.data.R, tx.data.S, V, true)
}

type PIP7Signer struct {
	EIP155Signer
	chainId, chainIdMul         *big.Int
	PIP7ChainId, PIP7ChainIdMul *big.Int
}

func NewPIP7Signer(chainId *big.Int, pip7ChainId *big.Int) PIP7Signer {
	if chainId == nil {
		chainId = new(big.Int)
	}
	// https://github.com/PlatONnetwork/PIPs/blob/master/PIPs/PIP-7.md
	return PIP7Signer{
		chainId:        chainId,
		chainIdMul:     new(big.Int).Mul(chainId, big.NewInt(2)),
		PIP7ChainId:    pip7ChainId,
		PIP7ChainIdMul: new(big.Int).Mul(pip7ChainId, big.NewInt(2)),
	}
}

func (s PIP7Signer) Equal(s2 Signer) bool {
	pip7, ok := s2.(PIP7Signer)
	return ok && pip7.chainId.Cmp(s.chainId) == 0 && pip7.PIP7ChainId.Cmp(s.PIP7ChainId) == 0
}

func (s PIP7Signer) Sender(tx *Transaction) (common.Address, error) {
	txChainId := tx.ChainId()
	if txChainId.Cmp(s.chainId) != 0 && txChainId.Cmp(s.PIP7ChainId) != 0 {
		return common.Address{}, ErrInvalidChainId
	}
	h := s.Hash(tx, txChainId)

	//ChainIdMul
	V := new(big.Int).Sub(tx.data.V, txChainId.Mul(txChainId, big.NewInt(2)))
	V.Sub(V, big8)

	return recoverPlain(h, tx.data.R, tx.data.S, V, true)
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (s PIP7Signer) Hash(tx *Transaction, chainId *big.Int) common.Hash {
	cid := chainId
	if chainId == nil {
		cid = s.chainId
	}
	return rlpHash([]interface{}{
		tx.data.AccountNonce,
		tx.data.Price,
		tx.data.GasLimit,
		tx.data.Recipient,
		tx.data.Amount,
		tx.data.Payload,
		cid, uint(0), uint(0),
	})
}

func (s PIP7Signer) SignatureAndSender(tx *Transaction) (common.Address, []byte, error) {
	txChainId := tx.ChainId()
	if txChainId.Cmp(s.chainId) != 0 && txChainId.Cmp(s.PIP7ChainId) != 0 {
		return common.Address{}, []byte{}, ErrInvalidChainId
	}
	//ChainIdMul
	V := new(big.Int).Sub(tx.data.V, txChainId.Mul(txChainId, big.NewInt(2)))
	V.Sub(V, big8)

	return recoverPubKeyAndSender(s.Hash(tx, txChainId), tx.data.R, tx.data.S, V, true)
}

// SignatureValues returns the raw R, S, V values corresponding to the
// given signature.This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func (s PIP7Signer) SignatureValues(sig []byte) (R, S, V *big.Int, err error) {
	if len(sig) != 65 {
		panic(fmt.Sprintf("wrong size for signature: got %d, want 65", len(sig)))
	}
	R = new(big.Int).SetBytes(sig[:32])
	S = new(big.Int).SetBytes(sig[32:64])
	V = new(big.Int).SetBytes([]byte{sig[64] + 35})
	V.Add(V, s.chainIdMul)

	return R, S, V, nil
}

func recoverPlain(sighash common.Hash, R, S, Vb *big.Int, homestead bool) (common.Address, error) {
	if Vb.BitLen() > 8 {
		return common.Address{}, ErrInvalidSig
	}
	V := byte(Vb.Uint64() - 27)
	if !crypto.ValidateSignatureValues(V, R, S, homestead) {
		return common.Address{}, ErrInvalidSig
	}
	// encode the signature in uncompressed format
	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, crypto.SignatureLength)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V
	// recover the public key from the signature
	pub, err := crypto.Ecrecover(sighash[:], sig)
	if err != nil {
		return common.Address{}, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return common.Address{}, errors.New("invalid public key")
	}
	var addr common.Address
	copy(addr[:], crypto.Keccak256(pub[1:])[12:])
	return addr, nil
}

func recoverPubKeyAndSender(sighash common.Hash, R, S, Vb *big.Int, homestead bool) (common.Address, []byte, error) {
	if Vb.BitLen() > 8 {
		return common.Address{}, []byte{}, ErrInvalidSig
	}
	V := byte(Vb.Uint64() - 27)
	if !crypto.ValidateSignatureValues(V, R, S, homestead) {
		return common.Address{}, []byte{}, ErrInvalidSig
	}
	// encode the snature in uncompressed format
	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, 65)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V
	// recover the public key from the snature
	pub, err := crypto.Ecrecover(sighash[:], sig)
	if err != nil {
		return common.Address{}, []byte{}, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return common.Address{}, []byte{}, errors.New("invalid public key")
	}
	pubValid := crypto.Keccak256(pub[1:])
	var addr common.Address
	copy(addr[:], pubValid[12:])
	return addr, pub[1:], nil
}

// deriveChainId derives the chain id from the given v parameter
func deriveChainId(v *big.Int) *big.Int {
	if v == nil {
		return nil
	}
	if v.BitLen() <= 64 {
		v := v.Uint64()
		if v == 27 || v == 28 {
			return new(big.Int)
		}
	}
	if v.Cmp(big.NewInt(35)) >= 0 {
		v = new(big.Int).Sub(v, big.NewInt(35))
		return v.Div(v, big.NewInt(2))
	}
	return nil
}
