package http

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

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

const (
	ISO8601 = "2006-01-02T15:04:05-0700"
)

type DownloadRequest struct {
	response.ErrorEncoder

	Path struct {
		Address string
	} `example:"/download/1.2.3.4"`

	Query struct {
		From  *string `example:"2025-11-09T11:00:00+01:00"`
		To    *string `example:"2019-09-07T15:50:00+01:00"`
		Count *int
	}

	response.BytesEncoder
	Response []byte

	Index store.IndexInterface
	Store store.StoreInterface

	snaplen int32
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

	query := &store.FindQuery{}

	if r.Query.From != nil {
		query.From, err = time.Parse(time.RFC3339, *r.Query.From)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	if r.Query.To != nil {
		query.To, err = time.Parse(time.RFC3339, *r.Query.To)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	if r.Query.Count != nil {
		query.MaxCount = *r.Query.Count
	}

	ip := net.ParseIP(r.Path.Address).To4()

	var pkts []*models.Packet
	directData, ok := r.Store.(store.DirectDataInterface)
	if ok {
		pkts, err = directData.GetPacketsByAddress(ctx, ip, query)
		if err != nil {
			return errors.WithStack(err)
		}
	} else {

		ids, err := r.Index.FindPacketsByAddress(ctx, ip)
		if err != nil {
			return errors.WithStack(err)
		}

		pkts, err = r.Store.GetPackets(ctx, ids, query)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	span.SetAttributes(
		attribute.Int("response.packets_count", len(pkts)),
	)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `inline; filename=data.pcap`)

	buff := bytes.NewBufferString("")
	pcapWriter := pcapgo.NewWriter(buff)

	err = pcapWriter.WriteFileHeader(65535, layers.LinkTypeEthernet)
	if err != nil {
		return errors.WithStack(err)
	}

	fmt.Printf("Packets: %d\n", len(pkts))

	for _, pkt := range pkts {
		ci := gopacket.CaptureInfo{
			CaptureLength: len(pkt.Data),
			Length:        len(pkt.Data),
			Timestamp:     pkt.Timestamp,
		}

		err = pcapWriter.WritePacket(ci, pkt.Data)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	r.Response = buff.Bytes()

	return nil
}
