package index

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/franela/goblin"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/schmurfy/sniffit/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	_indexPath = "/tmp/ghtjdk0.idx"
)

func buildPacket(ipSource, ipDest net.IP) []byte {
	mac, err := net.ParseMAC("02:00:5e:10:00:00")
	if err != nil {
		panic(err)
	}

	eth := &layers.Ethernet{
		EthernetType: layers.EthernetTypeIPv4,
		SrcMAC:       mac,
		DstMAC:       mac,
	}

	ip := &layers.IPv4{
		SrcIP: ipSource,
		DstIP: ipDest,
	}
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	err = gopacket.SerializeLayers(buf, opts,
		eth,
		ip,
	)

	return buf.Bytes()
}

func TestBBoltIndex(t *testing.T) {
	g := goblin.Goblin(t)

	g.Describe("BBolt Index", func() {
		var index *BboltIndex
		var ctx context.Context

		g.BeforeEach(func() {
			var err error

			os.Remove(_indexPath)
			index, err = NewBboltIndex(_indexPath)
			require.Nil(g, err)

			ctx = context.Background()
		})

		g.It("should remove packets from index", func() {
			addr1 := net.ParseIP("172.16.0.1").To4()
			addr2 := net.ParseIP("172.16.0.2").To4()
			addr3 := net.ParseIP("1.2.3.4").To4()

			p1 := &models.Packet{Id: "p1", Data: buildPacket(addr1, addr3)}
			p2 := &models.Packet{Id: "p2", Data: buildPacket(addr1, addr3)}
			p3 := &models.Packet{Id: "p3", Data: buildPacket(addr2, addr3)}

			// add packets
			err := index.IndexPackets(ctx, []*models.Packet{
				p1,
				p2,
				p3,
			})
			require.Nil(g, err)

			// and remove some
			err = index.DeletePackets(ctx, []*models.Packet{
				p1,
				p3,
			})
			require.Nil(g, err)

			// and check what we have
			ids, err := index.FindPackets(ctx, addr1)
			require.Nil(g, err)
			assert.Len(g, ids, 1)
		})
	})
}
