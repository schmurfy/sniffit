package badger_store

import (
	"net"
	"testing"
	"time"

	"github.com/franela/goblin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/schmurfy/sniffit/index_encoder"
	"github.com/schmurfy/sniffit/models"
	"github.com/schmurfy/sniffit/store"
)

func TestBadger(t *testing.T) {
	g := goblin.Goblin(t)
	g.Describe("badger", func() {

		g.Describe("buildKeys", func() {
			var st *BadgerStore
			var pkts []*models.Packet

			g.BeforeEach(func() {
				var err error

				opts := DefaultOptions
				opts.Path = "/tmp/toto"
				st, err = New(&opts)
				require.NoError(g, err)

				src := net.ParseIP("192.168.0.1")
				dst := net.ParseIP("192.168.0.2")
				now := time.Now()

				pkts = []*models.Packet{
					{Id: "p1", Data: store.BuildPacket(src, dst), Timestamp: now},
					{Id: "p2", Data: store.BuildPacket(src, dst), Timestamp: now},
					{Id: "p3", Data: store.BuildPacket(src, dst), Timestamp: now},
					{Id: "p4", Data: store.BuildPacket(src, dst), Timestamp: now},
					{Id: "p5", Data: store.BuildPacket(src, dst), Timestamp: now},
				}
			})

			g.It("should add every packets in index", func() {
				keys := st.buildKeys(pkts)
				assert.Len(g, keys, 2)

				for _, v := range keys {
					// assert.Equal(g, "toto", k)
					assert.Len(g, v.ids, 5)
				}

			})
		})

		store.TestIndex(g, func(path string, encoder index_encoder.Interface) (store.StoreInterface, error) {
			opts := DefaultOptions
			opts.Path = path
			opts.Encoder = encoder
			opts.TTL = 7 * 24 * time.Hour
			return New(&opts)
		})
	})
}
