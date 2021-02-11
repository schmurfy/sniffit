package badger_store

import (
	"context"

	"github.com/dgraph-io/badger/v3"
	"github.com/pkg/errors"
	"github.com/schmurfy/sniffit/models"
	"github.com/schmurfy/sniffit/store"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/trace"
)

// TODO: https://dgraph.io/docs/badger/faq/#my-writes-are-really-slow-why
func (n *BadgerStore) StorePackets(ctx context.Context, pkts []*models.Packet) (err error) {
	ctx, span := _tracer.Start(ctx, "StorePackets",
		trace.WithAttributes(
			label.Int("request.packets_count", len(pkts)),
		))
	defer func() {
		if err != nil {
			span.RecordError(err)
		}
		span.End()
	}()

	err = n.db.Update(func(tx *badger.Txn) error {
		var data []byte
		var err error

		for _, pkt := range pkts {

			data, err = pkt.Serialize()
			if err != nil {
				return errors.WithStack(err)
			}

			entry := badger.NewEntry([]byte(pkt.Id), data)
			entry.ExpiresAt = uint64(pkt.Timestamp.Add(n.ttl).Unix())
			err = errors.WithStack(tx.SetEntry(entry))
			if err != nil {
				return err
			}
		}

		return nil
	})

	return
}

func (n *BadgerStore) GetPackets(ctx context.Context, ids []string, q *store.FindQuery) (pkts []*models.Packet, err error) {
	ctx, span := _tracer.Start(ctx, "GetPackets",
		trace.WithAttributes(
			label.Array("request.ids", ids),
			label.Any("request.query", q),
		))
	defer func() {
		if err != nil {
			span.RecordError(err)
		}
		span.End()
	}()

	pkts = make([]*models.Packet, 0, len(ids))

	err = n.db.View(func(tx *badger.Txn) error {
		var err error
		var item *badger.Item

		for _, id := range ids {
			item, err = tx.Get([]byte(id))
			if err != nil {
				if err == badger.ErrKeyNotFound {
					continue
				}

				return err
			}

			err = item.Value(func(data []byte) error {
				pp, err := models.UnserializePacket(data)
				if err != nil {
					return errors.WithStack(err)
				}

				pkts = append(pkts, pp)
				return nil
			})
		}

		return nil
	})

	return
}

func (n *BadgerStore) DataKeys(ctx context.Context) (ret []string, err error) {
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
				return errors.WithStack(err)
			}
		}
		return nil
	})

	return
}
