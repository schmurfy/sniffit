package archivist

import (
	"fmt"
	"io"
	"net"
	"time"

	"google.golang.org/grpc"

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

	s := grpc.NewServer()
	pb.RegisterArchivistServer(s, ar)

	return s.Serve(lis)
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

		pkts := make([]*models.Packet, len(pbPacketBatch.Packets))
		fmt.Printf("received %d packets\n", len(pkts))

		var lastTime time.Time

		for n, pbPacket := range pbPacketBatch.Packets {
			pkts[n] = models.NewPacketFromProto(pbPacket)
			if lastTime.Before(pkts[n].Timestamp) {
				lastTime = pkts[n].Timestamp
			}
		}

		ar.lastPacket = lastTime

		// store the packet data
		err = ar.dataStore.StorePackets(pkts)
		if err != nil {
			return err
		}

		// and the index if all went fine
		errs, hasErrors := ar.indexStore.IndexPackets(pkts)
		if hasErrors {
			var n int
			var returnedError error

			// remove packet with errors
			for n, err = range errs {
				if err != nil {
					// remove packet from store if the index was not saved
					_ = ar.dataStore.DeletePacket(pkts[n])
					returnedError = err
				}
			}

			return returnedError
		}
	}
}
