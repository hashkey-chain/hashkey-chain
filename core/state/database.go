// Copyright 2017 The go-ethereum Authors
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

package state

import (
	"errors"
	"fmt"

	"github.com/hashkey-chain/hashkey-chain/core/rawdb"

	"github.com/VictoriaMetrics/fastcache"

	lru "github.com/hashicorp/golang-lru"

	"github.com/hashkey-chain/hashkey-chain/common"
	"github.com/hashkey-chain/hashkey-chain/ethdb"
	"github.com/hashkey-chain/hashkey-chain/trie"
)

const (
	// Number of codehash->size associations to keep.
	codeSizeCacheSize = 100000

	// Cache size granted for caching clean code.
	codeCacheSize = 64 * 1024 * 1024
)

// Database wraps access to tries and contract code.
type Database interface {
	// OpenTrie opens the main account trie.
	OpenTrie(root common.Hash) (Trie, error)

	// OpenStorageTrie opens the storage trie of an account.
	OpenStorageTrie(addrHash, root common.Hash) (Trie, error)

	// CopyTrie returns an independent copy of the given trie.
	CopyTrie(Trie) Trie

	NewTrie(t Trie) Trie
	// ContractCode retrieves a particular contract's code.
	ContractCode(addrHash, codeHash common.Hash) ([]byte, error)

	// ContractCodeSize retrieves a particular contracts code's size.
	ContractCodeSize(addrHash, codeHash common.Hash) (int, error)

	// TrieDB retrieves the low level trie database used for data storage.
	TrieDB() *trie.Database
}

// Trie is a Ethereum Merkle Trie.
type Trie interface {
	TryGet(key []byte) ([]byte, error)
	TryUpdate(key, value []byte) error
	TryDelete(key []byte) error
	Commit(onleaf trie.LeafCallback) (common.Hash, error)
	Hash() common.Hash
	NodeIterator(startKey []byte) trie.NodeIterator
	GetKey([]byte) []byte // TODO(fjl): remove this when SecureTrie is removed
	Prove(key []byte, fromLevel uint, proofDb ethdb.KeyValueWriter) error
}

// NewDatabase creates a backing store for state. The returned database is safe for
// concurrent use and retains cached trie nodes in memory. The pool is an optional
// intermediate trie-node memory pool between the low level storage layer and the
// high level trie abstraction.
func NewDatabase(db ethdb.Database) Database {
	return NewDatabaseWithConfig(db, nil)
}

func NewDatabaseWithConfig(db ethdb.Database, config *trie.Config) Database {
	csc, _ := lru.New(codeSizeCacheSize)
	return &cachingDB{
		db:            trie.NewDatabaseWithConfig(db, config),
		codeSizeCache: csc,
		codeCache:     fastcache.New(codeCacheSize),
	}
}

type cachingDB struct {
	db            *trie.Database
	codeSizeCache *lru.Cache
	codeCache     *fastcache.Cache
}

//OpenTrie opens the main account trie.
func (db *cachingDB) OpenTrie(root common.Hash) (Trie, error) {
	return trie.NewSecure(root, db.db)
}

// OpenStorageTrie opens the storage trie of an account.
func (db *cachingDB) OpenStorageTrie(addrHash, root common.Hash) (Trie, error) {
	return trie.NewSecure(root, db.db)
}

// CopyTrie returns an independent copy of the given trie.
func (db *cachingDB) CopyTrie(t Trie) Trie {
	switch t := t.(type) {
	case *trie.SecureTrie:
		return t.Copy()
	default:
		panic(fmt.Errorf("unknown trie type %T", t))
	}
}

func (db *cachingDB) NewTrie(t Trie) Trie {
	switch t := t.(type) {
	case *trie.SecureTrie:
		return t.New()
	default:
		panic(fmt.Errorf("unknown trie type %T", t))
	}
}

// ContractCode retrieves a particular contract's code.
func (db *cachingDB) ContractCode(addrHash, codeHash common.Hash) ([]byte, error) {
	if code := db.codeCache.Get(nil, codeHash.Bytes()); len(code) > 0 {
		return code, nil
	}
	code := rawdb.ReadCode(db.db.DiskDB(), codeHash)
	if len(code) > 0 {
		db.codeCache.Set(codeHash.Bytes(), code)
		db.codeSizeCache.Add(codeHash, len(code))
		return code, nil
	}
	return nil, errors.New("not found")
}

// ContractCodeWithPrefix retrieves a particular contract's code. If the
// code can't be found in the cache, then check the existence with **new**
// db scheme.
func (db *cachingDB) ContractCodeWithPrefix(addrHash, codeHash common.Hash) ([]byte, error) {
	if code := db.codeCache.Get(nil, codeHash.Bytes()); len(code) > 0 {
		return code, nil
	}
	code := rawdb.ReadCodeWithPrefix(db.db.DiskDB(), codeHash)
	if len(code) > 0 {
		db.codeCache.Set(codeHash.Bytes(), code)
		db.codeSizeCache.Add(codeHash, len(code))
		return code, nil
	}
	return nil, errors.New("not found")
}

// ContractCodeSize retrieves a particular contracts code's size.
func (db *cachingDB) ContractCodeSize(addrHash, codeHash common.Hash) (int, error) {
	if cached, ok := db.codeSizeCache.Get(codeHash); ok {
		return cached.(int), nil
	}
	code, err := db.ContractCode(addrHash, codeHash)
	return len(code), err
}

// TrieDB retrieves any intermediate trie-node caching layer.
func (db *cachingDB) TrieDB() *trie.Database {
	return db.db
}
