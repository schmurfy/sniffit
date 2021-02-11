package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
	"github.com/schmurfy/sniffit/stats"
	"github.com/schmurfy/sniffit/store"
)

type GetStatsRequest struct {
	Path  struct{} `example:"/stats"`
	Query struct{}

	Response stats.Stats

	IndexStore store.IndexInterface
	DataStore  store.DataInterface
	Stats      *stats.Stats
}

func (r *GetStatsRequest) Handle(ctx context.Context, w http.ResponseWriter) {
	var err error
	var rawIps []string

	ctx, span := _tracer.Start(ctx, "GetStats")
	defer func() {
		if err != nil {
			span.RecordError(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		span.End()
	}()

	rawIps, err = r.IndexStore.IndexKeys(ctx)
	if err != nil {
		err = errors.WithStack(err)
		return
	}

	statsCopy := *r.Stats
	statsCopy.Keys = len(rawIps)

	encoder := json.NewEncoder(w)
	err = errors.WithStack(encoder.Encode(statsCopy))
}
