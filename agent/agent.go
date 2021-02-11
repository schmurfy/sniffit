package agent

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/pkg/errors"
	"github.com/rs/xid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "github.com/schmurfy/sniffit/generated_pb/proto"
)

// 2021/01/21 05:21:08 rpc error: code = Unavailable desc = transport is closing
const (
	_batch_size = 1000
)

var (
	_batch_timeout = 1 * time.Second
	_packetsCount  = label.Key("packets_count")
	_tracer        = otel.Tracer("github.com/schmurfy/sniffit/archivist")
)

type Agent struct {
	ifName string
	filter string
	name   string

	grpcConn   *grpc.ClientConn
	grpcClient pb.ArchivistClient

	// internals
	pcapHandle *pcap.Handle
}

func New(interfaceName string, filter string, archivistAddress string, agentName string) (*Agent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// start grpc client
	conn, err := grpc.DialContext(ctx, archivistAddress,
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect")
	}

	return &Agent{
		ifName:     interfaceName,
		filter:     filter,
		grpcConn:   conn,
		grpcClient: pb.NewArchivistClient(conn),
		name:       agentName,
	}, nil
}

func (agent *Agent) sendPackets(ctx context.Context, queue chan gopacket.Packet, errorsCh chan error) {

	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"agent-name", agent.name,
	))

	batch := NewBatchQueue(_batch_size, _batch_timeout, func(pkts []*pb.Packet) {
		ctx, span := _tracer.Start(ctx, "sendPackets:NewBatchQueue",
			trace.WithAttributes(
				_packetsCount.Int(len(pkts)),
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
				// fmt.Printf("retrying in %s...\n", d.String())
			})

			if err != nil {
				span.RecordError(err)
				errorsCh <- err
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

	h, err := pcap.OpenLive(agent.ifName, 1000, false, pcap.BlockForever)
	if err != nil {
		return errors.WithStack(err)
	}

	agent.pcapHandle = h

	err = h.SetBPFFilter(agent.filter)
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
