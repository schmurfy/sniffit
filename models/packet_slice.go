package models

type PacketSlice []*Packet

func (p PacketSlice) Len() int {
	return len(p)
}

func (p PacketSlice) Less(i, j int) bool {
	return p[i].Timestamp.Before(p[j].Timestamp)
}

func (p PacketSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
