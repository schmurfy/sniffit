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
	"github.com/schmurfy/sniffit/index"
	"github.com/schmurfy/sniffit/models"
	"github.com/schmurfy/sniffit/stats"
	"github.com/schmurfy/sniffit/store"
)

type Archivist struct {
	dataStore   store.StoreInterface
	indexStore  index.IndexInterface
	stats       *stats.Stats
	lastCleanup time.Time
	retention   time.Duration
}

func New(store store.StoreInterface, idx index.IndexInterface, st *stats.Stats, cfg *config.ArchivistConfig) (*Archivist, error) {
	retention, err := time.ParseDuration(cfg.DataRetention)
	if err != nil {
		return nil, err
	}

	return &Archivist{
		dataStore:  store,
		indexStore: idx,
		stats:      st,
		retention:  retention,
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

	// schedule cleanup
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for {
			t := <-ticker.C

			// only run cleanup at night
			now := time.Now()
			if now.Hour() == 2 {
				fmt.Printf("Running cleanup at %s", t.Format(time.RFC3339))
				err = ar.Cleanup(context.Background(), 5000)
				if err != nil {

				}
			}
		}
	}()

	return s.Serve(lis)
}

func (ar *Archivist) Cleanup(ctx context.Context, maxCount int) error {
	tracer := otel.Tracer("")

	ctx, span := tracer.Start(ctx, "Cleanup")
	defer span.End()

	t := time.Now().Add(-ar.retention)

	// find all matching packets
	packets, err := ar.dataStore.FindPacketsBefore(ctx, t, maxCount)
	if err != nil {
		return err
	}

	// start by removing them from the index
	err = ar.indexStore.DeletePackets(ctx, packets)
	if err != nil {
		return err
	}

	// and then remove them from the store
	return ar.dataStore.DeletePackets(ctx, packets)
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

	ar.stats.RegisterPacket(agentName, lastTime)

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
