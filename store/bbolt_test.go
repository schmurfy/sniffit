package store

import (
	"context"
	"testing"
	"time"

	"github.com/franela/goblin"
	"github.com/schmurfy/sniffit/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBBolt(t *testing.T) {
	g := goblin.Goblin(t)

	g.Describe("BBolt", func() {
		var store *BboltStore
		var ctx context.Context
		var now time.Time

		g.BeforeEach(func() {
			var err error

			store, err = NewBboltStore("/tmp/store")
			require.Nil(g, err)

			ctx = context.Background()
			now = time.Now()
		})

		g.Describe("with data", func() {
			g.BeforeEach(func() {
				err := store.StorePackets(ctx, []*models.Packet{
					{Id: "p1", Timestamp: now.Add(-2 * time.Hour)},
					{Id: "p2", Timestamp: now.Add(-3 * time.Hour)},
					{Id: "p3", Timestamp: now.Add(-1 * time.Minute)},
					{Id: "p4", Timestamp: now.Add(time.Hour)},
				})

				require.Nil(g, err)
			})

			g.It("should return old packets", func() {
				packets, err := store.FindPacketsBefore(now.Add(-time.Hour))
				require.Nil(g, err)
				assert.Len(g, packets, 2)

				assert.Equal(g, "p1", packets[0].Id)
				assert.Equal(g, "p2", packets[1].Id)
			})

		})
	})
}
