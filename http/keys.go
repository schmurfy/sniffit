package http

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/schmurfy/sniffit/store"
)

type KeysList struct {
	IndexKeys []string `json:"index_keys"`
	DataKeys  []string `json:"data_keys"`
}

type GetKeysRequest struct {
	Path     struct{} `example:"/keys"`
	Query    struct{}
	Response KeysList

	IndexStore store.IndexInterface
	DataStore  store.DataInterface
}

func (r *GetKeysRequest) Handle(ctx context.Context, w http.ResponseWriter) {
	var err error
	var ret KeysList

	ctx, span := _tracer.Start(ctx, "GetKeys")
	defer func() {
		if err != nil {
			span.RecordError(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		span.End()
	}()

	var indexKeys []string
	indexKeys, err = r.IndexStore.IndexKeys(ctx)
	if err != nil {
		return
	}

	ret.IndexKeys = make([]string, len(indexKeys))
	for i, k := range indexKeys {
		var data []byte
		parts := strings.Split(k, "-")
		data, err = hex.DecodeString(parts[0])
		if err != nil {
			return
		}

		ret.IndexKeys[i] = net.IP(data).String()
	}

	ret.DataKeys, err = r.DataStore.DataKeys(ctx)
	if err != nil {
		return
	}

	w.Header().Set("Content-Type", "application/json")

	encoder := json.NewEncoder(w)
	err = encoder.Encode(&ret)
	if err != nil {
		return
	}
}
