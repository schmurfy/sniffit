package store

import (
	"context"
	"net"

	"github.com/schmurfy/sniffit/models"
)

type Stats map[string]string

type IndexInterface interface {
	IndexPackets(context.Context, []*models.Packet) error
	IndexKeys(context.Context) ([]string, error)
	FindPacketsByAddress(context.Context, net.IP) ([]string, error)
	GetStats() (*Stats, error)
}

type DirectDataInterface interface {
	GetPacketsByAddress(context.Context, net.IP, *FindQuery) ([]*models.Packet, error)
}

type DataInterface interface {
	StorePackets(context.Context, []*models.Packet) error
	GetPackets(context.Context, []string, *FindQuery) ([]*models.Packet, error)
	DataKeys(context.Context) ([]string, error)
	GetStats() (*Stats, error)
}

type StoreInterface interface {
	IndexInterface
	DataInterface
}
