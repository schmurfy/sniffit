package index

import (
	"context"
	"net"

	"github.com/schmurfy/sniffit/models"
)

type IndexInterface interface {
	IndexPackets(ctx context.Context, pkt []*models.Packet) error
	AnyKeys() ([]string, error)
	FindPackets(ctx context.Context, ip net.IP) ([]string, error)
	DeletePackets(ctx context.Context, pkts []*models.Packet) error
}
