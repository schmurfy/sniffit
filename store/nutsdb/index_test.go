package nuts

import (
	"testing"
	"time"

	"github.com/franela/goblin"

	"github.com/schmurfy/sniffit/index_encoder"
	"github.com/schmurfy/sniffit/store"
)

func TestNuts(t *testing.T) {
	g := goblin.Goblin(t)
	g.Describe("nutsdb", func() {
		store.TestIndex(g, func(path string, encoder index_encoder.Interface) (store.StoreInterface, error) {
			opts := NutsDefaultOptions
			opts.Path = path
			opts.Encoder = encoder
			opts.TTL = 7 * 24 * time.Hour
			return New(&opts)
		})
	})
}
