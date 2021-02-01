package http

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	goHttp "net/http"

	"github.com/go-chi/chi"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/label"

	"github.com/schmurfy/sniffit/archivist"
	"github.com/schmurfy/sniffit/stats"
	"github.com/schmurfy/sniffit/store"
)

const (
	_tracer = "http"
)

func Start(addr string, arc *archivist.Archivist, indexStore store.IndexInterface, dataStore store.StoreInterface, st *stats.Stats) error {
	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {

	})

	r.Get("/stats", func(w http.ResponseWriter, r *http.Request) {
		rawIps, err := indexStore.AnyKeys()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		statsCopy := *st
		statsCopy.Keys = len(rawIps)

		encoder := json.NewEncoder(w)
		err = encoder.Encode(statsCopy)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	r.Get("/keys", func(w http.ResponseWriter, r *http.Request) {
		rawIps, err := indexStore.AnyKeys()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ips := make([]string, len(rawIps))
		for n, bb := range rawIps {
			ip := net.IP([]byte(bb))
			ips[n] = ip.String()
		}

		encoder := json.NewEncoder(w)
		err = encoder.Encode(ips)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	})

	r.Get("/download/{ip}", func(w http.ResponseWriter, r *http.Request) {
		ipStr := chi.URLParam(r, "ip")

		tracer := otel.Tracer(_tracer)
		ctx, span := tracer.Start(r.Context(), "download")
		defer span.End()

		span.SetAttributes(
			label.String("request.ip", ipStr),
		)

		ip := net.ParseIP(ipStr).To4()

		ids, err := indexStore.FindPacketsByAddress(ctx, ip)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			span.RecordError(err)
			return
		}

		findQuery, err := store.QueryFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			span.RecordError(err)
			return
		}

		pkts, err := dataStore.GetPackets(ctx, ids, findQuery)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			span.RecordError(err)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="data.pcap"`)

		pcapWriter := pcapgo.NewWriter(w)

		err = pcapWriter.WriteFileHeader(1000, layers.LinkTypeEthernet)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, pkt := range pkts {
			ci := gopacket.CaptureInfo{
				CaptureLength: int(pkt.CaptureLength),
				Length:        int(pkt.DataLength),
				Timestamp:     pkt.Timestamp,
			}

			err := pcapWriter.WritePacket(ci, pkt.Data)
			if err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

	})

	return goHttp.ListenAndServe(addr, r)
}
