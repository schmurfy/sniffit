package archivist

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	grpcotel "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc"
	"go.opentelemetry.io/otel/api/global"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "github.com/schmurfy/sniffit/generated_pb/proto"
	"github.com/schmurfy/sniffit/index"
	"github.com/schmurfy/sniffit/models"
	"github.com/schmurfy/sniffit/store"
)

type Archivist struct {
	dataStore  store.StoreInterface
	indexStore index.IndexInterface
	lastPacket time.Time
}

func New(st store.StoreInterface, idx index.IndexInterface) *Archivist {
	return &Archivist{
		dataStore:  st,
		indexStore: idx,
	}
}

func (ar *Archivist) LastReceivedPacket() time.Time {
	return ar.lastPacket
}

func (ar *Archivist) Start(address string) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(grpcotel.UnaryServerInterceptor(global.Tracer("grpc"))),
		grpc.StreamInterceptor(grpcotel.StreamServerInterceptor(global.Tracer("grpc"))),
	)
	pb.RegisterArchivistServer(s, ar)

	return s.Serve(lis)
}

func (ar *Archivist) handleReceivePackets(ctx context.Context, pbPacketBatch *pb.PacketBatch) error {
	tr := global.Tracer("archivist")

	globalCtx, globalSpan := tr.Start(ctx, "ReceivedPacket")
	defer globalSpan.End()

	md, _ := metadata.FromIncomingContext(ctx)
	agentName := md["agent-name"][0]

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

	ar.lastPacket = lastTime

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

func (ar *Archivist) SendPacket(stream pb.Archivist_SendPacketServer) error {
	for {
		pbPacketBatch, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.SendPacketResp{})
		}
		if err != nil {
			return err
		}

		err = ar.handleReceivePackets(stream.Context(), pbPacketBatch)
		if err != nil {
			return err
		}

	}
}
