package index

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/schmurfy/sniffit/index_encoder"
	"github.com/schmurfy/sniffit/models"
	"github.com/xujiajun/nutsdb"
	"go.opentelemetry.io/otel"
)

var (
	_nutsTracer        = otel.Tracer("index:nuts")
	NutsDefaultOptions = NutsIndexOptions{
		TimeFormat:  "2006:01:02",
		CurrentTime: time.Now,
	}
)

const (
	_nutsBucket = "index"
)

type NutsIndex struct {
	db          *nutsdb.DB
	encoder     index_encoder.Interface
	ttl         time.Duration
	timeFormat  string
	currentTime func() time.Time
}

type NutsIndexOptions struct {
	Path        string
	Encoder     index_encoder.Interface
	TimeFormat  string
	CurrentTime func() time.Time
}

func NewNutsDb(o *NutsIndexOptions) (*NutsIndex, error) {
	opts := nutsdb.DefaultOptions
	opts.Dir = o.Path

	db, err := nutsdb.Open(opts)
	if err != nil {
		return nil, err
	}

	return &NutsIndex{
		db:          db,
		encoder:     o.Encoder,
		timeFormat:  o.TimeFormat,
		currentTime: o.CurrentTime,
	}, nil
}

func (n *NutsIndex) buildKey(t time.Time, addr net.IP) *key {
	strTime := t.Format(n.timeFormat)

	tt, _ := time.Parse(n.timeFormat, strTime)

	return &key{
		name:      fmt.Sprintf("%s-%s", string(addr), strTime),
		timestamp: tt,
	}
}

func (n *NutsIndex) buildKeys(pkts []*models.Packet) map[string]*key {
	ret := make(map[string]*key, len(pkts))

	for _, pkt := range pkts {
		// extract packet data
		packet := gopacket.NewPacket(pkt.Data, layers.LayerTypeEthernet, gopacket.Default)
		ipLayer := packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4)

		for _, addr := range []net.IP{ipLayer.SrcIP, ipLayer.DstIP} {
			key := n.buildKey(pkt.Timestamp, addr)
			k := ret[key.name]
			if k == nil {
				ret[key.name] = key
				k = key
			}
			k.ids = append(key.ids, pkt.Id)
		}
	}

	return ret
}

type key struct {
	name      string
	ids       []string
	timestamp time.Time
}

// func (n *NutsIndex) buildIdListWithTime(pkts []*models.Packet) (map[string][]string, error) {
// 	ret := make(map[string][]string)

// 	n.forEachAddress(pkts, func(k *key) {
// 		ids, exists := ret[k.key]
// 		if exists {
// 			ids = append(ids, pkt.Id)
// 		} else {
// 			ids = []string{pkt.Id}
// 		}
// 		ret[k.key] = ids
// 	})

// 	return ret, nil
// }

func (n *NutsIndex) IndexPackets(ctx context.Context, pkts []*models.Packet) error {
	indexes := n.buildKeys(pkts)

	for key, k := range indexes {
		var list index_encoder.ValueInterface

		addr := []byte(key)

		expireTime := n.currentTime().Add(n.ttl).Unix()
		ttl := uint32(expireTime - k.timestamp.Unix())

		err := n.db.Update(func(tx *nutsdb.Tx) error {
			data, err := tx.Get(_nutsBucket, addr)
			if err != nil {
				if strings.HasPrefix(err.Error(), "not found bucket") || err == nutsdb.ErrKeyNotFound {
					list, err = n.encoder.NewEmpty()
					if err != nil {
						return err
					}
				} else {
					return err
				}
			} else {
				// load existing data
				list, err = n.encoder.NewFromData(data.Value)
				if err != nil {
					return err
				}
			}

			// now add the new entry
			err = list.Add(k.ids...)
			if err != nil {
				return err
			}

			// and serialize it back
			newData, err := list.Serialize()
			if err != nil {
				return err
			}
			err = tx.Put(_nutsBucket, addr, newData, ttl)
			if err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			return err
		}

		// err = n.db.View(func(tx *nutsdb.Tx) error {
		// 	entries, err := tx.GetAll(_nutsBucket)
		// 	// entries, err := tx.RangeScan(_nutsBucket, []byte("2020:01:01-"), []byte("2020:01:01-\xFF\xFF\xFF\xFF"))
		// 	if err != nil {
		// 		return err
		// 	}

		// 	for _, e := range entries {
		// 		fmt.Printf("  - %s\n", e.Key)
		// 	}

		// 	return nil
		// })

		// if err != nil {
		// 	return err
		// }
	}

	return nil
}

func (n *NutsIndex) AnyKeys() ([]string, error) {
	return []string{}, nil
}

func (n *NutsIndex) FindPackets(ctx context.Context, ip net.IP) (ret []string, err error) {
	err = n.db.View(func(tx *nutsdb.Tx) error {

		entries, err := tx.RangeScan(_nutsBucket,
			ip,
			append([]byte(ip), []byte("-9999:01:01")...),
		)

		if err != nil {
			return err
		}

		for _, data := range entries {
			list, err := n.encoder.NewFromData(data.Value)
			if err != nil {
				return err
			}

			ids, err := list.GetIds()
			if err != nil {
				return err
			}

			ret = append(ret, ids...)
		}

		return nil
	})

	return
}

func (n *NutsIndex) DeletePackets(ctx context.Context, pkts []*models.Packet) error {
	return nil
}
