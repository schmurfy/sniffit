package http

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/schmurfy/chipi/response"
	"github.com/schmurfy/sniffit/store"
)

type KeysList struct {
	IndexKeys []string `json:"index_keys"`
	DataKeys  []string `json:"data_keys,omitempty"`
}

type GetKeysRequest struct {
	response.ErrorEncoder

	Path  struct{} `example:"/keys"`
	Query struct {
		WithData bool `description:"include data keys" example:"false"`
	}

	response.JsonEncoder
	Response KeysList

	IndexStore store.IndexInterface
	DataStore  store.DataInterface
}

func (r *GetKeysRequest) Handle(ctx context.Context, w http.ResponseWriter) error {
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
		return errors.WithStack(err)
	}

	ret.IndexKeys = make([]string, len(indexKeys))
	for i, k := range indexKeys {
		var data []byte
		parts := strings.Split(k, "-")
		data, err = hex.DecodeString(parts[0])
		if err != nil {
			return errors.WithStack(err)
		}

		ret.IndexKeys[i] = net.IP(data).String()
	}

	if r.Query.WithData {
		ret.DataKeys, err = r.DataStore.DataKeys(ctx)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	w.Header().Set("Content-Type", "application/json")

	encoder := json.NewEncoder(w)
	err = encoder.Encode(&ret)
	return errors.WithStack(err)
}
