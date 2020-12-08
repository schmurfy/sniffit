package stats

import (
	"sync"
	"time"
)

type Source struct {
	LastPacket time.Time `json:"last_packet"`
	Packets    int       `json:"packets"`

	updateMutex sync.Mutex
}

type Stats struct {
	Sources map[string]*Source `json:"sources"`
	Keys    int                `json:"keys"`

	insertMutex sync.Mutex
}

func NewStats() *Stats {
	return &Stats{
		Sources: map[string]*Source{},
	}
}

func (st *Stats) RegisterPacket(agent string, t time.Time, count int) {
	src, exists := st.Sources[agent]
	if !exists {
		src = &Source{}
		st.insertMutex.Lock()
		st.Sources[agent] = src
		st.insertMutex.Unlock()
	}

	src.updateMutex.Lock()
	src.LastPacket = t
	src.Packets += count
	src.updateMutex.Unlock()
}
