package index

import (
	"context"
	"errors"
	"net"

	"github.com/golang/protobuf/proto"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	pb "github.com/schmurfy/sniffit/generated_pb/proto"
	"github.com/schmurfy/sniffit/models"
	bolt "go.etcd.io/bbolt"
	"go.opentelemetry.io/otel/api/global"
)

var (
	_ipAnyBucketKey = []byte("ip_any")

	_buckets = [][]byte{_ipAnyBucketKey}
)

type BboltIndex struct {
	db *bolt.DB
}

func NewBboltIndex(path string) (*BboltIndex, error) {
	var ret BboltIndex

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

func (i *BboltIndex) FindPackets(ip net.IP) ([]string, error) {
	var ret []string

	err := i.db.View(func(tx *bolt.Tx) error {
		var lst pb.IndexArray

		anyBucket := tx.Bucket(_ipAnyBucketKey)
		data := anyBucket.Get(ip)
		if data == nil {
			return errors.New("address unknown")
		}

		err := proto.Unmarshal(data, &lst)
		if err != nil {
			return err
		}

		ret = lst.Ids

		return nil
	})

	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (i *BboltIndex) IndexPackets(ctx context.Context, pkts []*models.Packet) error {
	tr := global.Tracer("BboltIndex")
	ctx, span := tr.Start(ctx, "IndexPackets")
	defer span.End()

	// prepare data before making the database update
	indexes := make(map[string][]string, 0)

	for _, pkt := range pkts {
		// extract packet data
		packet := gopacket.NewPacket(pkt.Data, layers.LayerTypeEthernet, gopacket.Default)
		ipLayer := packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4)

		// index source
		ids, exists := indexes[string(ipLayer.SrcIP)]
		if exists {
			ids = append(ids, pkt.Id)
		} else {
			ids = []string{pkt.Id}
		}
		indexes[string(ipLayer.SrcIP)] = ids

		// indx destination
		ids, exists = indexes[string(ipLayer.DstIP)]
		if exists {
			ids = append(ids, pkt.Id)
		} else {
			ids = []string{pkt.Id}
		}
		indexes[string(ipLayer.DstIP)] = ids

	}

	return i.db.Batch(func(tx *bolt.Tx) error {
		var lst pb.IndexArray

		anyBucket := tx.Bucket(_ipAnyBucketKey)

		for key, ids := range indexes {
			addr := []byte(key)
			// load existing list if it exists
			data := anyBucket.Get([]byte(addr))
			if data != nil {
				err := proto.Unmarshal(data, &lst)
				if err != nil {
					return err
				}
			}

			// now add the new entry
			lst.Ids = append(lst.Ids, ids...)

			// and serialize it back
			newData, err := proto.Marshal(&lst)
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

func (i *BboltIndex) Close() {
	if i.db != nil {
		i.db.Close()
	}
}
