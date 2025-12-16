package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/cenkalti/backoff/v4"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/pkg/errors"
	"github.com/rs/xid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "github.com/schmurfy/sniffit/generated_pb/proto"
)

var (
	_batch_timeout = 1 * time.Second
	_tracer        = otel.Tracer("github.com/schmurfy/sniffit/archivist")
)

type Agent struct {
	ifName string
	filter string
	name   string

	grpcConn   *grpc.ClientConn
	grpcClient pb.ArchivistClient

	// internals
	pcapHandle  *pcap.Handle
	snaplen     int32
	idGenerator *snowflake.Node
	batchSize   int
}

func New(interfaceName string, filter string, archivistAddress string, agentName string, batchSize int) (*Agent, error) {
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// start grpc client
	conn, err := grpc.NewClient(archivistAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect")
	}

	node, err := snowflake.NewNode(1)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect")
	}

	return &Agent{
		ifName:      interfaceName,
		filter:      filter,
		grpcConn:    conn,
		grpcClient:  pb.NewArchivistClient(conn),
		name:        agentName,
		idGenerator: node,
		batchSize:   batchSize,
	}, nil
}

func (agent *Agent) sendPackets(ctx context.Context, queue chan gopacket.Packet, errorsCh chan error) {

	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"agent-name", agent.name,
	))

	batch := NewBatchQueue(agent.batchSize, _batch_timeout, func(pkts []*pb.Packet) {
		ctx, span := _tracer.Start(ctx, "sendPackets:NewBatchQueue",
			trace.WithAttributes(
				attribute.Int("packets_count", len(pkts)),
			))
		defer span.End()

		for {
			operation := func() error {
				_, err := agent.grpcClient.SendPacket(ctx, &pb.PacketBatch{Packets: pkts})
				return errors.WithStack(err)
			}

			retryBackoff := backoff.NewExponentialBackOff()

			err := backoff.RetryNotify(operation, retryBackoff, func(err error, d time.Duration) {
				span.RecordError(err)
				fmt.Printf("retrying (%d packets) in %s (err: %s)...\n", len(pkts), d.String(), err.Error())
			})

			if err != nil {
				fmt.Printf("error: %+v\n", err)
				span.RecordError(err)
				// errorsCh <- err
			} else {
				break
			}
		}

	})

	for pkt := range queue {
		md := pkt.Metadata()

		batch.Add(&pb.Packet{
			Id:            xid.New().String(),
			Data:          pkt.Data(),
			TimestampNano: md.Timestamp.UnixNano(),
			CaptureLength: int64(md.CaptureInfo.CaptureLength),
			DataLength:    int64(md.CaptureInfo.Length),
		})
	}
}

func (agent *Agent) Start() error {
	ctx := context.Background()
	errQueue := make(chan error)

	inactive, err := pcap.NewInactiveHandle(agent.ifName)
	if err != nil {
		return errors.WithStack(err)
	}

	err = inactive.SetSnapLen(int(agent.snaplen))
	if err != nil {
		return errors.WithStack(err)
	}

	err = inactive.SetBufferSize(int(agent.snaplen) * 1000)
	if err != nil {
		return errors.WithStack(err)
	}

	err = inactive.SetTimeout(10 * time.Second)
	if err != nil {
		return errors.WithStack(err)
	}

	h, err := inactive.Activate()
	if err != nil {
		return errors.WithStack(err)
	}

	pktSource := gopacket.NewPacketSource(h, h.LinkType())

	go agent.sendPackets(ctx, pktSource.Packets(), errQueue)
	return <-errQueue
}

func (agent *Agent) Close() {
	if agent.grpcConn != nil {
		agent.grpcConn.Close()
	}
}
