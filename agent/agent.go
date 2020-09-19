package agent

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/rs/xid"
	"google.golang.org/grpc"

	pb "github.com/schmurfy/sniffit/generated_pb/proto"
)

const (
	_batch_size = 1000
)

var (
	_batch_timeout = 10 * time.Second
)

type Agent struct {
	ifName string
	filter string

	grpcConn   *grpc.ClientConn
	grpcClient pb.ArchivistClient

	// internals
	pcapHandle *pcap.Handle
}

func New(interfaceName string, filter string, archivistAddress string) (*Agent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// start grpc client
	conn, err := grpc.DialContext(ctx, archivistAddress,
		grpc.WithInsecure(),
		grpc.WithBlock(),
	)
	if err != nil {
		fmt.Printf("failed to connect to %s\n", archivistAddress)
		return nil, err
	}

	return &Agent{
		ifName:     interfaceName,
		filter:     filter,
		grpcConn:   conn,
		grpcClient: pb.NewArchivistClient(conn),
	}, nil
}

func (agent *Agent) sendPackets(ctx context.Context, queue chan gopacket.Packet, errors chan error) {
	var err error

retry:
	stream, err := agent.grpcClient.SendPacket(ctx)
	if err != nil {
		time.Sleep(1000 * time.Millisecond)
		goto retry
	}

	batch := NewBatchQueue(_batch_size, _batch_timeout, func(pkts []*pb.Packet) {
		err = stream.Send(&pb.PacketBatch{Packets: pkts})
	})

	for pkt := range queue {
		md := pkt.Metadata()

		// err := stream.Send(&pb.Packet{
		batch.Add(&pb.Packet{
			Id:            xid.New().String(),
			Data:          pkt.Data(),
			Timestamp:     md.Timestamp.Unix(),
			CaptureLength: int64(md.CaptureInfo.CaptureLength),
			DataLength:    int64(md.CaptureInfo.Length),
		})

		if err != nil {
			if err == io.EOF {
				fmt.Printf("send failure, retrying...\n")
				time.Sleep(1000 * time.Millisecond)
				queue <- pkt
				goto retry
			} else {
				errors <- err
				return
			}
		}
	}
}

func (agent *Agent) Start() error {
	ctx := context.Background()
	errQueue := make(chan error)

	h, err := pcap.OpenLive(agent.ifName, 1000, false, pcap.BlockForever)
	if err != nil {
		return err
	}

	agent.pcapHandle = h

	err = h.SetBPFFilter(agent.filter)
	if err != nil {
		return err
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
