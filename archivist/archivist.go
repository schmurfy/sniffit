package archivist

import (
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
		pbPacket, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.SendPacketResp{})
		}
		if err != nil {
			return err
		}

		ar.lastPacket = time.Now()

		pkt := models.NewPacketFromProto(pbPacket)

		// store the packet data
		err = ar.dataStore.StorePacket(pkt)
		if err != nil {
			return err
		}

		// and the index if all went fine
		err = ar.indexStore.IndexPacket(pkt)
		if err != nil {
			// remove packet from store if the index was not saved
			ar.dataStore.DeletePacket(pkt)
			return err
		}
	}
}
