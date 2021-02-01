// +build test

package store

import (
	"context"
	"net"
	"os"
	"time"

	"github.com/franela/goblin"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/schmurfy/sniffit/index_encoder"
	"github.com/schmurfy/sniffit/models"
)

const (
	_indexPath = "/tmp/ghtjdk1.idx"
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

const (
	_day  = 24 * time.Hour
	_week = 7 * _day
)

type initFunc func(string, index_encoder.Interface) (StoreInterface, error)

func TestIndex(g *goblin.G, f initFunc) {
	g.Describe("generic", func() {
		var store StoreInterface
		var ctx context.Context

		var p1, p2, p3 *models.Packet
		var exp1, exp2 *models.Packet

		now := time.Now()

		addr1 := net.ParseIP("172.16.0.1").To4()
		addr2 := net.ParseIP("172.16.0.2").To4()
		addr3 := net.ParseIP("1.2.3.4").To4()

		g.BeforeEach(func() {
			var err error

			p1 = &models.Packet{Id: "p1", Data: buildPacket(addr1, addr3),
				Timestamp: now.Add(-2 * _day)}
			p2 = &models.Packet{Id: "p2", Data: buildPacket(addr1, addr3),
				Timestamp: now.Add(-1 * _day)}
			p3 = &models.Packet{Id: "p3", Data: buildPacket(addr2, addr3),
				Timestamp: now}

			exp1 = &models.Packet{Id: "exp1", Data: buildPacket(addr1, addr2),
				Timestamp: now.Add(-2 * _week)}
			exp2 = &models.Packet{Id: "exp2", Data: buildPacket(addr2, addr3),
				Timestamp: now.Add(-3 * _week)}

			encoder, err := index_encoder.NewProto()
			require.Nil(g, err)

			os.RemoveAll(_indexPath)
			store, err = f(_indexPath, encoder)
			require.Nil(g, err)

			ctx = context.Background()
		})

		g.Describe("index", func() {
			g.It("should find indexed packets", func() {
				// index packets
				err := store.IndexPackets(ctx, []*models.Packet{
					p1,
					p2,
					p3,
				})
				require.Nil(g, err)

				// and check what we have
				ids, err := store.FindPacketsByAddress(ctx, addr1)
				require.Nil(g, err)
				assert.Len(g, ids, 2)
			})

			g.It("should expire packets from index", func() {
				// add packets
				err := store.IndexPackets(ctx, []*models.Packet{
					p1, p2, p3,
					exp1, exp2,
				})
				require.Nil(g, err)

				// and check what we have
				ids, err := store.FindPacketsByAddress(ctx, addr2)
				require.Nil(g, err)
				assert.Equal(g, ids, []string{"p3"})
			})
		})

		g.Describe("data", func() {

			g.BeforeEach(func() {
				err := store.StorePackets(ctx, []*models.Packet{
					p1, p2, p3,
					exp1, exp2,
				})

				require.Nil(g, err)
			})

			g.It("should return stored packets", func() {
				packets, err := store.GetPackets(ctx, []string{"p1", "p3"}, &FindQuery{})
				require.Nil(g, err)
				assert.Len(g, packets, 2)
			})

			g.It("should not return expired packets", func() {
				packets, err := store.GetPackets(ctx, []string{"p1", "exp1", "exp2"}, &FindQuery{})
				require.Nil(g, err)
				assert.Len(g, packets, 1)
			})

		})

	})
}
