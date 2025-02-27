// Copyright 2021 The PlatON Network Authors
// This file is part of the PlatON-Go library.
//
// The PlatON-Go library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The PlatON-Go library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the PlatON-Go library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"math/big"
	"testing"
	"time"

	"github.com/hashkey-chain/hashkey-chain/common"
	"github.com/hashkey-chain/hashkey-chain/core/cbfttypes"
	"github.com/hashkey-chain/hashkey-chain/core/snapshotdb"
	"github.com/hashkey-chain/hashkey-chain/core/types"
	"github.com/hashkey-chain/hashkey-chain/event"
	"github.com/hashkey-chain/hashkey-chain/trie"
)

func TestBlockChainReactorClose(t *testing.T) {
	t.Run("close after commit", func(t *testing.T) {
		eventmux := new(event.TypeMux)
		reacter := NewBlockChainReactor(eventmux, big.NewInt(100))
		reacter.Start(common.PPOS_VALIDATOR_MODE)
		var parenthash common.Hash
		cbftress := make(chan cbfttypes.CbftResult, 5)
		go func() {
			for i := 1; i < 11; i++ {
				header := new(types.Header)
				header.Number = big.NewInt(int64(i))
				header.Time = uint64(i)
				header.ParentHash = parenthash
				block := types.NewBlock(header, nil, nil, new(trie.Trie))
				snapshotdb.Instance().NewBlock(header.Number, header.ParentHash, block.Hash())
				parenthash = block.Hash()
				cbftress <- cbfttypes.CbftResult{Block: block}
			}
			close(cbftress)
		}()

		for value := range cbftress {
			eventmux.Post(value)
		}

		reacter.Close()

		time.Sleep(time.Second)
		snapshotdb.Instance().Clear()
	})
}
