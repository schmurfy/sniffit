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
	"github.com/pkg/errors"
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

func (n *BadgerStore) buildKey(addr net.IP, packetId string) []byte {
	ret := fmt.Sprintf("%s-%s", hex.EncodeToString(addr), packetId)
	return []byte(ret)
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

	wb := n.db.NewWriteBatch()
	defer wb.Cancel()

	for _, pkt := range pkts {
		packet := gopacket.NewPacket(pkt.Data, layers.LayerTypeEthernet, gopacket.Default)
		ipLayer := packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4)

		for _, addr := range []net.IP{ipLayer.SrcIP, ipLayer.DstIP} {
			// addr := []byte(pkt.)
			key := n.buildKey(addr, pkt.Id)
			entry := badger.NewEntry(key, []byte{})
			entry.ExpiresAt = uint64(pkt.Timestamp.Add(n.ttl).Unix())

			err = errors.WithStack(wb.SetEntry(entry))
			if err != nil {
				return
			}
		}
	}

	err = errors.WithStack(wb.Flush())
	return
}

func (n *BadgerStore) IndexKeys(ctx context.Context) (ret []string, err error) {
	n.cachedIndexKeysMutex.Lock()
	defer n.cachedIndexKeysMutex.Unlock()

	if time.Now().Sub(n.lastIndexKeysScan) < n.cachedIndexKeysInterval {
		// return cached version
		return n.cachedIndexKeys, nil
	}

	// otherwise query the keys
	mret := map[string]interface{}{}

	err = n.db.View(func(tx *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10

		it := tx.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			parts := strings.Split(string(k), "-")
			if len(parts) >= 2 {
				mret[parts[0]] = nil
			}

			if err != nil {
				return errors.WithStack(err)
			}
		}
		return nil
	})

	n.lastIndexKeysScan = time.Now()
	n.cachedIndexKeys = ret

	ret = make([]string, 0, len(mret))

	for k := range mret {
		ret = append(ret, k)
	}

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
			k := item.Key()
			parts := strings.Split(string(k), "-")
			if len(parts) >= 2 {
				ret = append(ret, parts[1])
			}
		}

		return nil
	})

	return
}
