package models

import (
	"bytes"
	"encoding/gob"
	"time"

	pb "github.com/schmurfy/sniffit/generated_pb/proto"
)

type Packet struct {
	Id            string
	Data          []byte
	Timestamp     time.Time
	CaptureLength int64
	DataLength    int64
}

func NewPacketFromProto(pkt *pb.Packet) *Packet {
	return &Packet{
		Id:            pkt.Id,
		Data:          pkt.Data,
		CaptureLength: pkt.CaptureLength,
		DataLength:    pkt.DataLength,
		Timestamp:     time.Unix(pkt.Timestamp, 0),
	}
}

func UnserializePacket(data []byte) (*Packet, error) {
	var ret Packet

	rd := bytes.NewReader(data)
	decoder := gob.NewDecoder(rd)

	err := decoder.Decode(&ret)
	return &ret, err
}

func (pp *Packet) Serialize() ([]byte, error) {
	buffer := bytes.NewBufferString("")
	encoder := gob.NewEncoder(buffer)

	err := encoder.Encode(pp)
	return buffer.Bytes(), err
}
