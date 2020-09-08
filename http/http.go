package http

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	goHttp "net/http"

	"github.com/go-chi/chi"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"

	"github.com/schmurfy/sniffit/index"
	"github.com/schmurfy/sniffit/store"
)

func Start(addr string, indexStore index.IndexInterface, dataStore store.StoreInterface) error {
	r := chi.NewRouter()

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

		ip := net.ParseIP(ipStr).To4()

		ids, err := indexStore.FindPackets(ip)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		findQuery, err := store.QueryFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Printf("query: %+v\n", findQuery)

		pkts, err := dataStore.FindPackets(ids, findQuery)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
