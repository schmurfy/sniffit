package index

import (
	"context"
	"net"

	"github.com/schmurfy/sniffit/models"
)

type IndexInterface interface {
	IndexPackets(ctx context.Context, pkt []*models.Packet) error
	AnyKeys() ([]string, error)
	FindPackets(ip net.IP) ([]string, error)
	Close()
}
