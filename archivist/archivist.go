package archivist

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/schmurfy/sniffit/config"
	pb "github.com/schmurfy/sniffit/generated_pb/proto"
	"github.com/schmurfy/sniffit/models"
	"github.com/schmurfy/sniffit/stats"
	"github.com/schmurfy/sniffit/store"
)

var (
	_tracer = otel.Tracer("github.com/schmurfy/sniffit/archivist")
)

type Archivist struct {
	dataStore  store.StoreInterface
	indexStore store.IndexInterface
	stats      *stats.Stats
	retention  time.Duration
}

func New(store store.StoreInterface, idx store.IndexInterface, st *stats.Stats, cfg *config.ArchivistConfig) (*Archivist, error) {
	return &Archivist{
		dataStore:  store,
		indexStore: idx,
		stats:      st,
		retention:  cfg.DataRetention,
	}, nil
}

func (ar *Archivist) Start(address string) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return errors.WithStack(err)
	}

	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	pb.RegisterArchivistServer(s, ar)

	return s.Serve(lis)
}

func (ar *Archivist) handleReceivePackets(ctx context.Context, pbPacketBatch *pb.PacketBatch) (err error) {
	ctx, span := _tracer.Start(ctx, "handleReceivePackets",
		trace.WithAttributes(
			attribute.Int("events-count", len(pbPacketBatch.Packets)),
		))
	defer func() {
		if err != nil {
			span.RecordError(err)
		}
		span.End()
	}()

	md, _ := metadata.FromIncomingContext(ctx)
	agentName := md["agent-name"][0]

	span.SetAttributes(
		attribute.String("agent-name", agentName),
	)

	pkts := make([]*models.Packet, len(pbPacketBatch.Packets))
	fmt.Printf("received %d packets from %s\n", len(pkts), agentName)

	var lastTime time.Time

	for n, pbPacket := range pbPacketBatch.Packets {
		pkts[n] = models.NewPacketFromProto(pbPacket)
		if lastTime.Before(pkts[n].Timestamp) {
			lastTime = pkts[n].Timestamp
		}
	}

	ar.stats.RegisterPacket(agentName, lastTime, len(pkts))

	metrics.GetOrCreateCounter(`packets_received`).AddInt64(int64(len(pkts)))

	// store the packet data
	err = errors.WithStack(ar.dataStore.StorePackets(ctx, pkts))
	if err != nil {
		return
	}

	// and the index if all went fine
	err = errors.WithStack(ar.indexStore.IndexPackets(ctx, pkts))
	if err != nil {
		return
	}

	return nil
}

func (ar *Archivist) SendPacket(ctx context.Context, batch *pb.PacketBatch) (*pb.SendPacketResp, error) {
	return &pb.SendPacketResp{}, ar.handleReceivePackets(ctx, batch)
}
