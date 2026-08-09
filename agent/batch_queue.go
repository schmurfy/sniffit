package agent

import (
	"fmt"
	"sync"
	"time"

	pb "github.com/schmurfy/sniffit/generated_pb/proto"
)

type BatchQueue struct {
	batch      []*pb.Packet
	next_index int
	capacity   int
	callback   func([]*pb.Packet) error
	mutex      sync.Mutex
	timeout    time.Duration
	timer      *time.Timer
}

func NewBatchQueue(cap int, timeout time.Duration, f func([]*pb.Packet) error) *BatchQueue {
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

		if !ret.flushQueue() {
			// flush failed, re-arm the timer to retry later
			ret.timer.Reset(ret.timeout)
		}
	})

	return ret
}

// flushQueue flushes the current batch via the callback.
// Must be called with the mutex held.
// Returns true on success, false if the callback returned an error
// (in which case next_index is NOT reset, preserving the batch for retry).
func (bq *BatchQueue) flushQueue() bool {
	tmp := bq.batch[0:bq.next_index]
	if len(tmp) > 0 {
		if err := bq.callback(tmp); err != nil {
			return false
		}
	}

	bq.next_index = 0
	return true
}

func (bq *BatchQueue) Add(pkt *pb.Packet) {
	bq.mutex.Lock()
	defer bq.mutex.Unlock()

	if bq.next_index >= bq.capacity {
		// queue is full from a previously failed flush; retry now
		if !bq.flushQueue() {
			// still failing, tail-drop the new packet to avoid
			// blocking the capture pipeline indefinitely
			fmt.Printf("batch queue full and flush still failing, dropping packet\n")
			return
		}
	}

	bq.batch[bq.next_index] = pkt
	bq.next_index++

	if bq.next_index >= bq.capacity {
		bq.flushQueue()
	}

	bq.timer.Stop()
	bq.timer.Reset(bq.timeout)
}
