package badger_store

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/schmurfy/sniffit/index_encoder"
	"github.com/schmurfy/sniffit/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/trace"
)

var (
	_tracer = otel.Tracer("badger_store")
)

// copy in shared
type key struct {
	name      string
	ids       []string
	timestamp time.Time
}

func (n *BadgerStore) buildKey(t time.Time, addr net.IP) *key {
	strTime := t.Format(n.timeFormat)

	tt, _ := time.Parse(n.timeFormat, strTime)

	return &key{
		name:      fmt.Sprintf("%s-%s", hex.EncodeToString(addr), strTime),
		timestamp: tt,
	}
}

func (n *BadgerStore) buildKeys(pkts []*models.Packet) map[string]*key {
	ret := make(map[string]*key, len(pkts))

	for _, pkt := range pkts {
		// extract packet data
		packet := gopacket.NewPacket(pkt.Data, layers.LayerTypeEthernet, gopacket.Default)
		ipLayer := packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4)

		for _, addr := range []net.IP{ipLayer.SrcIP, ipLayer.DstIP} {
			key := n.buildKey(pkt.Timestamp, addr)
			k, exists := ret[key.name]
			if !exists {
				ret[key.name] = key
				k = key
			}
			k.ids = append(k.ids, pkt.Id)
		}
	}

	return ret
}

func (n *BadgerStore) IndexPackets(ctx context.Context, pkts []*models.Packet) (err error) {
	ctx, span := _tracer.Start(ctx, "IndexPackets",
		trace.WithAttributes(
			label.Int("request.packets_count", len(pkts)),
		))
	defer func() {
		if err != nil {
			span.RecordError(err)
		}
		span.End()
	}()

	indexes := n.buildKeys(pkts)

	for key, k := range indexes {
		var list index_encoder.ValueInterface

		addr := []byte(key)

		err = n.db.Update(func(tx *badger.Txn) error {

			// load key if exists
			item, err := tx.Get(addr)
			if err != nil {
				if err == badger.ErrKeyNotFound {
					list, err = n.encoder.NewEmpty()
					if err != nil {
						return err
					}
				} else {
					return err
				}
			} else {
				// load existing data
				err = item.Value(func(data []byte) error {
					list, err = n.encoder.NewFromData(data)
					return err
				})

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

			// and save it back
			entry := badger.NewEntry(addr, newData)
			entry.ExpiresAt = uint64(k.timestamp.Add(n.ttl).Unix())

			ids, _ := list.GetIds()
			span.AddEvent("saved packet",
				trace.WithAttributes(
					label.String("ttl", n.ttl.String()),
					label.String("timestamp", k.timestamp.String()),
					label.String("expire_at", k.timestamp.Add(n.ttl).String()),
					label.String("packets", strings.Join(ids, ",")),
					label.String("key", key),
				),
			)

			// fmt.Printf("\nt: %s\nttl: %s\n", k.timestamp.String(), n.ttl.String())
			// fmt.Printf("expire: %s\n", k.timestamp.Add(n.ttl).String())

			err = tx.SetEntry(entry)
			// err = tx.PutWithTimestamp(_indexBucket, addr, newData, uint32(n.ttl.Seconds()), uint64(k.timestamp.Unix()))
			if err != nil {
				return err
			}

			return nil
		})

	}

	return
}

func (n *BadgerStore) IndexKeys(ctx context.Context) (ret []string, err error) {
	ret = []string{}

	err = n.db.View(func(tx *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10

		it := tx.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			err := item.Value(func(v []byte) error {
				ret = append(ret, string(k))
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return
}

func (n *BadgerStore) FindPacketsByAddress(ctx context.Context, ip net.IP) (ret []string, err error) {
	ctx, span := _tracer.Start(ctx, "FindPacketsByAddress")
	defer func() {
		if err != nil {
			span.RecordError(err)
		}
		span.End()
	}()

	err = n.db.View(func(tx *badger.Txn) error {
		it := tx.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte(hex.EncodeToString(ip))

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			// k := item.Key()
			err := item.Value(func(v []byte) error {

				list, err := n.encoder.NewFromData(v)
				if err != nil {
					return err
				}

				ids, err := list.GetIds()
				if err != nil {
					return err
				}

				ret = append(ret, ids...)

				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	return
}
