package models

import (
	"bytes"
	"encoding/gob"
	"net"
	"time"

	"github.com/pkg/errors"
	pb "github.com/schmurfy/sniffit/generated_pb/proto"
)

type Packet struct {
	Id            string
	Data          []byte
	Timestamp     time.Time
	CaptureLength uint16
	DataLength    uint16
	SrcIP         net.IP
	DstIP         net.IP
}

func NewPacketFromProto(pkt *pb.Packet) *Packet {
	return &Packet{
		Id:            pkt.Id,
		Data:          pkt.Data,
		CaptureLength: uint16(pkt.CaptureLength),
		DataLength:    uint16(pkt.DataLength),
		Timestamp:     time.Unix(pkt.Timestamp, pkt.TimestampNano),
	}
}

func UnserializePacket(data []byte) (*Packet, error) {
	var ret Packet

	rd := bytes.NewReader(data)
	decoder := gob.NewDecoder(rd)

	err := errors.WithStack(decoder.Decode(&ret))
	return &ret, err
}

func (pp *Packet) Serialize() ([]byte, error) {
	buffer := bytes.NewBufferString("")
	encoder := gob.NewEncoder(buffer)

	err := errors.WithStack(encoder.Encode(pp))
	return buffer.Bytes(), err
}
