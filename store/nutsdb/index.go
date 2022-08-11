package nuts

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/schmurfy/sniffit/index_encoder"
	"github.com/schmurfy/sniffit/models"
	"github.com/xujiajun/nutsdb"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var (
	_tracer            = otel.Tracer("index:nuts")
	NutsDefaultOptions = NutsStoreOptions{
		TimeFormat:  "2006:01:02",
		CurrentTime: time.Now,
	}
)

const (
	_indexBucket = "index"
)

func (n *NutsStore) buildKey(t time.Time, addr net.IP) *key {
	strTime := t.Format(n.timeFormat)

	tt, _ := time.Parse(n.timeFormat, strTime)

	return &key{
		name:      fmt.Sprintf("%s-%s", hex.EncodeToString(addr), strTime),
		timestamp: tt,
	}
}

func (n *NutsStore) buildKeys(pkts []*models.Packet) map[string]*key {
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

// func (n *NutsStore) buildIdListWithTime(pkts []*models.Packet) (map[string][]string, error) {
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

func (n *NutsStore) IndexPackets(ctx context.Context, pkts []*models.Packet) (err error) {
	ctx, span := _tracer.Start(ctx, "IndexPackets",
		trace.WithAttributes(
			attribute.Int("request.packets_count", len(pkts)),
		))
	defer func() {
		if err != nil {
			span.RecordError(err)
		}
		span.End()
	}()

	indexes := n.buildKeys(pkts)

	// err = n.db.Update(func(tx *nutsdb.Tx) error {
	// 	var err error
	// 	err = tx.PutWithTimestamp(_indexBucket, []byte("TOTO"), []byte("TITI"), uint32(n.ttl.Seconds()), uint64(time.Now().Unix()))
	// 	if err != nil {
	// 		return err
	// 	}

	// 	return nil
	// })
	// if err != nil {
	// 	return
	// }

	for key, k := range indexes {
		var list index_encoder.ValueInterface

		addr := []byte(key)

		err = n.db.Update(func(tx *nutsdb.Tx) error {
			data, err := tx.Get(_indexBucket, addr)
			if err != nil {
				if (err == nutsdb.ErrKeyNotFound) || (err == nutsdb.ErrNotFoundKey) {
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

			ids, _ := list.GetIds()
			span.AddEvent("saved packet",
				trace.WithAttributes(
					attribute.String("ttl", n.ttl.String()),
					attribute.String("timestamp", k.timestamp.String()),
					attribute.Array("packets", ids),
					attribute.String("key", key),
					attribute.Int("newDataSize", len(newData)),
				),
			)

			err = tx.PutWithTimestamp(_indexBucket, addr, newData, uint32(n.ttl.Seconds()), uint64(k.timestamp.Unix()))
			if err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			return
		}

		// err = n.db.View(func(tx *nutsdb.Tx) error {
		// 	entries, err := tx.GetAll(_nutsBucket)
		// 	if err != nil {
		// 		return err
		// 	}

		// 	for _, e := range entries {
		// 		fmt.Printf("  - %s\n", e.Key)
		// 	}

		// 	return nil
		// })

	}

	return
}

func (n *NutsStore) IndexKeys(ctx context.Context) (ret []string, err error) {
	ctx, span := _tracer.Start(ctx, "IndexKeys")
	defer func() {
		if err != nil {
			span.RecordError(err)
		}
		span.End()
	}()

	ret, err = n.listKeys(_indexBucket)
	return
}

func (n *NutsStore) FindPacketsByAddress(ctx context.Context, ip net.IP) (ret []string, err error) {
	ctx, span := _tracer.Start(ctx, "FindPacketsByAddress")
	defer func() {
		if err != nil {
			span.RecordError(err)
		}
		span.End()
	}()

	err = n.db.View(func(tx *nutsdb.Tx) error {

		entries, _, err := tx.PrefixScan(_indexBucket, ip, 0, 20000)

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
