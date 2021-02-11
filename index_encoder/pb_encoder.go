package index_encoder

import (
	"github.com/pkg/errors"
	pb "github.com/schmurfy/sniffit/generated_pb/proto"
	"google.golang.org/protobuf/proto"
)

type ProtoEncoder struct {
}

func NewProto() (*ProtoEncoder, error) {
	return &ProtoEncoder{}, nil
}

type ProtoEncoderValue struct {
	list *pb.IndexArray
}

func (e *ProtoEncoder) NewEmpty() (ValueInterface, error) {
	return &ProtoEncoderValue{
		list: &pb.IndexArray{},
	}, nil
}

func (e *ProtoEncoder) NewFromData(data []byte) (ValueInterface, error) {
	lst := &pb.IndexArray{}

	err := errors.WithStack(proto.Unmarshal(data, lst))
	if err != nil {
		return nil, err
	}

	return &ProtoEncoderValue{
		list: lst,
	}, nil
}

func (e *ProtoEncoderValue) Add(ids ...string) error {
	e.list.Ids = append(e.list.Ids, ids...)
	return nil
}

func (e *ProtoEncoderValue) Serialize() ([]byte, error) {
	return proto.Marshal(e.list)
}

func (e *ProtoEncoderValue) GetIds() ([]string, error) {
	return e.list.Ids, nil
}
