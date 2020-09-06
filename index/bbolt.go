package index

import (
	"errors"
	"fmt"
	"net"

	"github.com/golang/protobuf/proto"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	pb "github.com/schmurfy/sniffit/generated_pb/proto"
	"github.com/schmurfy/sniffit/models"
	bolt "go.etcd.io/bbolt"
)

var (
	_ipSourceBucketKey      = []byte("ip_source")
	_ipDestinationBucketKey = []byte("ip_destination")
	_ipAnyBucketKey         = []byte("ip_any")

	_buckets = [][]byte{_ipSourceBucketKey, _ipDestinationBucketKey, _ipAnyBucketKey}
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

func addIpIndex(b *bolt.Bucket, addr net.IP, pkt *models.Packet) error {
	var lst pb.IndexArray

	// load existing list if it exists
	data := b.Get(addr)
	if data != nil {
		err := proto.Unmarshal(data, &lst)
		if err != nil {
			return err
		}
	}

	// now add the new entry
	lst.Ids = append(lst.Ids, pkt.Id)

	// and serialize it back
	newData, err := proto.Marshal(&lst)
	if err != nil {
		return err
	}

	return b.Put(addr, newData)
}

func (i *BboltIndex) AnyKeys() ([]string, error) {
	ret := []string{}

	err := i.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(_ipAnyBucketKey)

		return b.ForEach(func(k []byte, v []byte) error {
			fmt.Printf("key[%s] %d bytes\n", net.IP(k).String(), len(v))
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

func (i *BboltIndex) IndexPacket(pkt *models.Packet) error {

	// extract packet data
	packet := gopacket.NewPacket(pkt.Data, layers.LayerTypeEthernet, gopacket.Default)
	ipLayer := packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4)

	// create index for source and destination ip
	return i.db.Batch(func(tx *bolt.Tx) error {
		srcBucket := tx.Bucket(_ipSourceBucketKey)
		dstBucket := tx.Bucket(_ipDestinationBucketKey)
		anyBucket := tx.Bucket(_ipAnyBucketKey)

		err := addIpIndex(srcBucket, ipLayer.SrcIP, pkt)
		if err != nil {
			return err
		}

		err = addIpIndex(dstBucket, ipLayer.DstIP, pkt)
		if err != nil {
			return err
		}

		err = addIpIndex(anyBucket, ipLayer.SrcIP, pkt)
		if err != nil {
			return err
		}

		err = addIpIndex(anyBucket, ipLayer.DstIP, pkt)
		if err != nil {
			return err
		}

		return nil
	})
}

func (i *BboltIndex) Close() {
	if i.db != nil {
		i.db.Close()
	}
}
