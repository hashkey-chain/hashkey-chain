// Copyright 2019 The go-ethereum Authors
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

package graphql

import (
	"encoding/json"
	"net/http"

	"github.com/graph-gophers/graphql-go"

	graphqlEth "github.com/AlayaNetwork/graphql-go"

	json2 "github.com/hashkey-chain/hashkey-chain/common/json"
	"github.com/hashkey-chain/hashkey-chain/internal/ethapi"
	"github.com/hashkey-chain/hashkey-chain/node"
)

type handler struct {
	Schema    *graphql.Schema
	SchemaEth *graphqlEth.Schema
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var params struct {
		Query         string                 `json:"query"`
		OperationName string                 `json:"operationName"`
		Variables     map[string]interface{} `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.URL.Path == "/graphql" || r.URL.Path == "/graphql/" {
		response := h.SchemaEth.Exec(r.Context(), params.Query, params.OperationName, params.Variables)
		responseJSON, err := json2.Marshal(response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(response.Errors) > 0 {
			w.WriteHeader(http.StatusBadRequest)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(responseJSON)
	} else {
		response := h.Schema.Exec(r.Context(), params.Query, params.OperationName, params.Variables)
		responseJSON, err := json.Marshal(response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(response.Errors) > 0 {
			w.WriteHeader(http.StatusBadRequest)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(responseJSON)
	}
}

// New constructs a new GraphQL service instance.
func New(stack *node.Node, backend ethapi.Backend, cors, vhosts []string) error {
	if backend == nil {
		panic("missing backend")
	}
	// check if http server with given endpoint exists and enable graphQL on it
	return newHandler(stack, backend, cors, vhosts)
}

// newHandler returns a new `http.Handler` that will answer GraphQL queries.
// It additionally exports an interactive query browser on the / endpoint.
func newHandler(stack *node.Node, backend ethapi.Backend, cors, vhosts []string) error {
	q := Resolver{backend}

	s, err := graphql.ParseSchema(schema, &q)
	if err != nil {
		return err
	}

	sEth, err := graphqlEth.ParseSchema(schema, &q)
	if err != nil {
		return err
	}

	h := handler{Schema: s, SchemaEth: sEth}
	handler := node.NewHTTPHandlerStack(h, cors, vhosts)

	stack.RegisterHandler("GraphQL UI", "/graphql/ui", GraphiQL{})
	stack.RegisterHandler("GraphQL UI", "/hskchain/graphql/ui", GraphiQL{}) // for PlatON

	stack.RegisterHandler("GraphQL", "/graphql", handler)
	stack.RegisterHandler("GraphQL", "/graphql/", handler)

	// for PlatON
	stack.RegisterHandler("GraphQL", "/hskchain/graphql", handler)
	stack.RegisterHandler("GraphQL", "/hskchain/graphql/", handler)

	return nil
}
