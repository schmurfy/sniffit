package index

import (
	"context"
	"errors"
	"net"

	"github.com/golang/protobuf/proto"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	pb "github.com/schmurfy/sniffit/generated_pb/proto"
	"github.com/schmurfy/sniffit/index_encoder"
	"github.com/schmurfy/sniffit/models"
	bolt "go.etcd.io/bbolt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/label"
)

var (
	_boltTracer = otel.Tracer("index:bbolt")
)

var (
	_ipAnyBucketKey = []byte("ip_any")

	_buckets = [][]byte{_ipAnyBucketKey}
)

type BboltIndex struct {
	db      *bolt.DB
	encoder index_encoder.Interface
}

func NewBboltIndex(path string, encoder index_encoder.Interface) (*BboltIndex, error) {
	ret := BboltIndex{
		encoder: encoder,
	}

	var err error

	ret.db, err = bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}

	// create buckets
	err = ret.db.Update(func(tx *bolt.Tx) error {
		for _, bucketName := range _buckets {
			_, err = tx.CreateBucketIfNotExists(bucketName)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &ret, nil
}

func (i *BboltIndex) AnyKeys() ([]string, error) {
	ret := []string{}

	err := i.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(_ipAnyBucketKey)

		return b.ForEach(func(k []byte, v []byte) error {
			ret = append(ret, string(k))

			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// func (i *BboltIndex) FindPacketsBefore(t time.Time) ([]string, error) {
// 	var ret []string

// 	err := i.db.View(func(tx *bolt.Tx) error {
// 		var lst pb.IndexArray

// 		anyBucket := tx.Bucket(_ipAnyBucketKey)

// 		anyBucket.ForEach(func(k []byte, v []byte) error {
// 			var lst pb.IndexArray

// 			err := proto.Unmarshal(v, &lst)
// 			if err != nil {
// 				return err
// 			}

// 			return nil
// 		})

// 	})

// 	return ret, nil
// }

func (i *BboltIndex) FindPackets(ctx context.Context, ip net.IP) (ret []string, err error) {
	// ctx, span := _boltTracer.Start(ctx, "FindPackets", trace.WithAttributes(
	// 	label.KeyValue{Key: "ip", Value: label.StringValue(ip.String())},
	// ))
	// defer func() {
	// 	if err != nil {
	// 		span.RecordError(err)
	// 	}
	// 	span.End()
	// }()

	err = i.db.View(func(tx *bolt.Tx) error {
		var err error

		anyBucket := tx.Bucket(_ipAnyBucketKey)
		data := anyBucket.Get(ip)
		if data == nil {
			return errors.New("address unknown")
		}

		list, err := i.encoder.NewFromData(data)
		if err != nil {
			return err
		}

		ret, err = list.GetIds()
		if err != nil {
			return err
		}

		return nil
	})

	return
}

func buildIdList(pkts []*models.Packet) (map[string][]string, error) {
	ret := make(map[string][]string)

	for _, pkt := range pkts {
		// extract packet data
		packet := gopacket.NewPacket(pkt.Data, layers.LayerTypeEthernet, gopacket.Default)
		ipLayer := packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4)

		// index source
		ids, exists := ret[string(ipLayer.SrcIP)]
		if exists {
			ids = append(ids, pkt.Id)
		} else {
			ids = []string{pkt.Id}
		}
		ret[string(ipLayer.SrcIP)] = ids

		// index destination
		ids, exists = ret[string(ipLayer.DstIP)]
		if exists {
			ids = append(ids, pkt.Id)
		} else {
			ids = []string{pkt.Id}
		}
		ret[string(ipLayer.DstIP)] = ids

	}

	return ret, nil
}

func (i *BboltIndex) IndexPackets(ctx context.Context, pkts []*models.Packet) error {
	_, span := _boltTracer.Start(ctx, "IndexPackets")
	span.SetAttributes(
		label.KeyValue{Key: "packets_count", Value: label.IntValue(len(pkts))},
	)
	defer span.End()

	// prepare data before making the database update
	indexes, err := buildIdList(pkts)
	if err != nil {
		return err
	}

	return i.db.Batch(func(tx *bolt.Tx) error {
		anyBucket := tx.Bucket(_ipAnyBucketKey)

		for key, ids := range indexes {
			var list index_encoder.ValueInterface

			addr := []byte(key)
			// load existing list if it exists
			data := anyBucket.Get([]byte(addr))
			if data != nil {
				list, err = i.encoder.NewFromData(data)
			} else {
				list, err = i.encoder.NewEmpty()
			}

			if err != nil {
				return err
			}

			// now add the new entry
			list.Add(ids...)

			// and serialize it back
			newData, err := list.Serialize()
			if err != nil {
				return err
			}

			err = anyBucket.Put(addr, newData)
			if err != nil {
				return err
			}

		}

		return nil
	})
}

func includeString(arr []string, el string) bool {
	for _, key := range arr {
		if key == el {
			return true
		}
	}

	return false
}

func (i *BboltIndex) DeletePackets(ctx context.Context, pkts []*models.Packet) error {
	ctx, span := _boltTracer.Start(ctx, "DeletePackets")
	defer span.End()

	span.SetAttributes(
		label.Int("request.packets_count", len(pkts)),
	)

	// first we need to build lists of ips and ids
	deletionQueue, err := buildIdList(pkts)
	if err != nil {
		return err
	}

	err = i.db.Update(func(tx *bolt.Tx) error {
		for key, ids := range deletionQueue {
			var lst pb.IndexArray

			anyBucket := tx.Bucket(_ipAnyBucketKey)

			data := anyBucket.Get([]byte(key))
			if data == nil {
				return errors.New("address unknown")
			}

			err := proto.Unmarshal(data, &lst)
			if err != nil {
				return err
			}

			// we have the list, remove unwanted ids
			newList := make([]string, 0, len(lst.Ids))

			for _, id := range lst.Ids {
				if !includeString(ids, id) {
					newList = append(newList, id)
				}
			}

			// and write it back
			lst.Ids = newList
			newData, err := proto.Marshal(&lst)
			if err != nil {
				return err
			}

			err = anyBucket.Put([]byte(key), newData)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		span.RecordError(err)
	}

	return nil
}

func (i *BboltIndex) Close() {
	if i.db != nil {
		i.db.Close()
	}
}
