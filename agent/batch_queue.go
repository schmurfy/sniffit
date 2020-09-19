package agent

import (
	"sync"
	"time"

	pb "github.com/schmurfy/sniffit/generated_pb/proto"
)

type BatchQueue struct {
	batch      []*pb.Packet
	next_index int
	capacity   int
	callback   func([]*pb.Packet)
	mutex      sync.Mutex
	timeout    time.Duration
	timer      *time.Timer
}

func NewBatchQueue(cap int, timeout time.Duration, f func([]*pb.Packet)) *BatchQueue {
	ret := &BatchQueue{
		batch:      make([]*pb.Packet, cap),
		capacity:   cap,
		callback:   f,
		next_index: 0,
		timeout:    timeout,
	}

	ret.timer = time.AfterFunc(timeout, func() {
		ret.mutex.Lock()
		defer ret.mutex.Unlock()

		ret.flushQueue()
	})

	return ret
}

// should be called with the lock acquired
func (bq *BatchQueue) flushQueue() {
	tmp := bq.batch[0:bq.next_index]
	if len(tmp) > 0 {
		bq.callback(tmp)
	}

	bq.next_index = 0
}

func (bq *BatchQueue) resetTimer() {
	bq.timer.Stop()
	bq.timer.Reset(bq.timeout)
}

func (bq *BatchQueue) Add(pkt *pb.Packet) {
	bq.mutex.Lock()
	defer bq.mutex.Unlock()

	bq.batch[bq.next_index] = pkt
	bq.next_index++

	if bq.next_index >= bq.capacity {
		bq.flushQueue()
	}

	bq.resetTimer()
}
