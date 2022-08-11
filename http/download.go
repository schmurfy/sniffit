package http

import (
	"context"
	"net"
	"net/http"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/pkg/errors"
	"github.com/schmurfy/chipi/response"
	"github.com/schmurfy/sniffit/models"
	"github.com/schmurfy/sniffit/store"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type DownloadRequest struct {
	response.ErrorEncoder

	Path struct {
		Address string
	} `example:"/download/1.2.3.4"`

	Query struct {
		// From  *string
		// To    *string
		// Count *int
	}

	response.JsonEncoder
	Response []byte

	Index store.IndexInterface
	Store store.StoreInterface
}

func (r *DownloadRequest) Handle(ctx context.Context, w http.ResponseWriter) error {
	var err error

	ctx, span := _tracer.Start(ctx, "DownloadRequest", trace.WithAttributes(
		attribute.String("request.Address", r.Path.Address),
	))
	defer func() {
		if err != nil {
			span.RecordError(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		span.End()
	}()

	ip := net.ParseIP(r.Path.Address).To4()

	ids, err := r.Index.FindPacketsByAddress(ctx, ip)
	if err != nil {
		return errors.WithStack(err)
	}

	query := &store.FindQuery{}

	// findQuery, err := store.QueryFromRequest(r)
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }
	var pkts []*models.Packet
	pkts, err = r.Store.GetPackets(ctx, ids, query)
	if err != nil {
		return errors.WithStack(err)
	}

	span.SetAttributes(
		attribute.Int("response.packets_count", len(pkts)),
	)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `inline; filename=data.pcap`)

	pcapWriter := pcapgo.NewWriter(w)

	err = pcapWriter.WriteFileHeader(1000, layers.LinkTypeEthernet)
	if err != nil {
		return errors.WithStack(err)
	}

	for _, pkt := range pkts {
		ci := gopacket.CaptureInfo{
			CaptureLength: int(pkt.CaptureLength),
			Length:        int(pkt.DataLength),
			Timestamp:     pkt.Timestamp,
		}

		err = pcapWriter.WritePacket(ci, pkt.Data)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}
