package archivist

import (
	"context"
	"fmt"
	"net"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/label"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/schmurfy/sniffit/config"
	pb "github.com/schmurfy/sniffit/generated_pb/proto"
	"github.com/schmurfy/sniffit/models"
	"github.com/schmurfy/sniffit/stats"
	"github.com/schmurfy/sniffit/store"
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
		return err
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
	)
	pb.RegisterArchivistServer(s, ar)

	return s.Serve(lis)
}

func (ar *Archivist) handleReceivePackets(ctx context.Context, pbPacketBatch *pb.PacketBatch) error {
	tr := otel.Tracer("")

	globalCtx, globalSpan := tr.Start(ctx, "ReceivedPacket")
	defer globalSpan.End()

	globalSpan.SetAttributes(label.KeyValue{
		Key:   "events-count",
		Value: label.IntValue(len(pbPacketBatch.Packets)),
	})

	md, _ := metadata.FromIncomingContext(ctx)
	agentName := md["agent-name"][0]

	globalSpan.SetAttributes(
		label.KeyValue{Key: "agent-name", Value: label.StringValue(agentName)},
	)

	pkts := make([]*models.Packet, len(pbPacketBatch.Packets))
	fmt.Printf("received %d packets from %s\n", len(pkts), agentName)

	var lastTime time.Time

	_, span := tr.Start(globalCtx, "ConvertPackets")
	for n, pbPacket := range pbPacketBatch.Packets {
		pkts[n] = models.NewPacketFromProto(pbPacket)
		if lastTime.Before(pkts[n].Timestamp) {
			lastTime = pkts[n].Timestamp
		}
	}
	span.End()

	ar.stats.RegisterPacket(agentName, lastTime, len(pkts))

	// store the packet data
	err := ar.dataStore.StorePackets(globalCtx, pkts)
	if err != nil {
		return err
	}

	// and the index if all went fine
	err = ar.indexStore.IndexPackets(globalCtx, pkts)
	if err != nil {
		return err
	}

	return nil
}

func (ar *Archivist) SendPacket(ctx context.Context, batch *pb.PacketBatch) (*pb.SendPacketResp, error) {
	return &pb.SendPacketResp{}, ar.handleReceivePackets(ctx, batch)
}
