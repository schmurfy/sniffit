package http

import (
	"net/http"
	goHttp "net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"github.com/schmurfy/chipi"
	"go.opentelemetry.io/otel"

	"github.com/schmurfy/sniffit/archivist"
	"github.com/schmurfy/sniffit/config"
	"github.com/schmurfy/sniffit/stats"
	"github.com/schmurfy/sniffit/store"
)

var (
	_tracer = otel.Tracer("http")
)

func Start(addr string, arc *archivist.Archivist, indexStore store.IndexInterface, dataStore store.StoreInterface, st *stats.Stats, cfg *config.ArchivistConfig) error {
	r := chi.NewRouter()

	api, err := chipi.New(r, &openapi3.Info{
		Title:       "test api",
		Description: "a great api",
	})

	if err != nil {
		return errors.WithStack(err)
	}

	// api.AddServer(&openapi3.Server{
	// 	URL: "http://127.0.0.1:2121",
	// })

	r.Use(cors.AllowAll().Handler)
	r.Get("/doc.json", api.ServeSchema)
	r.Get("/doc", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(INDEX_DOC))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {

	})

	err = api.Get(r, "/stats", &GetStatsRequest{
		IndexStore: indexStore,
		DataStore:  dataStore,
		Stats:      st,
	})
	if err != nil {
		return errors.WithStack(err)
	}

	err = api.Get(r, "/download/{Address}", &DownloadRequest{
		Index:   indexStore,
		Store:   dataStore,
		snaplen: cfg.SnapLen,
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return goHttp.ListenAndServe(addr, r)
}
