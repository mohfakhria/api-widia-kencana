package queue

import (
	"context"
	"errors"
	"sync"
)

// ErrClosed is returned by Pop and TryPop after Close has been called and all
// remaining items have been drained.
var ErrClosed = errors.New("mpsc: queue closed")

// Queue is an unbounded MPSC queue. The zero value is not usable; create one
// with New. It must not be copied after first use.
type Queue[T any] struct {
	mu     sync.Mutex
	write  []T  // incoming items, guarded by mu
	closed bool // guarded by mu

	// signal wakes the consumer when items arrive or the queue closes.
	// Capacity 1 so producers never block on notification.
	signal chan struct{}

	// read is the consumer-side buffer. Only the single consumer touches
	// it, so no lock is needed here.
	read []T
	rpos int
}

// New creates an empty queue.
func New[T any]() *Queue[T] {
	return &Queue[T]{signal: make(chan struct{}, 1)}
}

// Push appends v to the queue. It never blocks and is safe for concurrent use
// by multiple producers. It returns false if the queue has been closed, in
// which case v is dropped.
func (q *Queue[T]) Push(v T) bool {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return false
	}
	q.write = append(q.write, v)
	q.mu.Unlock()

	q.notify()
	return true
}

// Pop removes and returns the oldest item. It blocks until an item is
// available, ctx is done, or the queue is closed and fully drained.
// Only one goroutine may call Pop/TryPop.
func (q *Queue[T]) Pop(ctx context.Context) (T, error) {
	for {
		if v, ok := q.take(); ok {
			return v, nil
		}

		closed, empty := q.refill()
		if !empty {
			continue
		}
		if closed {
			var zero T
			return zero, ErrClosed
		}

		select {
		case <-q.signal:
		case <-ctx.Done():
			var zero T
			return zero, ctx.Err()
		}
	}
}

// TryPop removes and returns the oldest item without blocking. It returns
// (v, true, nil) on success, (zero, false, nil) when the queue is momentarily
// empty, and (zero, false, ErrClosed) when the queue is closed and drained.
func (q *Queue[T]) TryPop() (v T, ok bool, err error) {
	if v, ok := q.take(); ok {
		return v, true, nil
	}
	closed, empty := q.refill()
	if !empty {
		v, _ := q.take()
		return v, true, nil
	}
	if closed {
		return v, false, ErrClosed
	}
	return v, false, nil
}

// Close marks the queue closed. Subsequent Push calls return false. The
// consumer can still drain items already queued; once empty, Pop and TryPop
// return ErrClosed. Close is idempotent and safe to call from any goroutine.
func (q *Queue[T]) Close() {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()

	q.notify()
}

// Len reports the number of items currently queued (producer side plus the
// consumer's undrained buffer). It is an instantaneous snapshot and useful
// mainly for monitoring.
func (q *Queue[T]) Len() int {
	q.mu.Lock()
	n := len(q.write)
	q.mu.Unlock()
	return n + len(q.read) - q.rpos
}

// take pops from the consumer-local buffer. Consumer-only; no lock.
func (q *Queue[T]) take() (T, bool) {
	if q.rpos >= len(q.read) {
		return *new(T), false
	}
	v := q.read[q.rpos]
	var zero T
	q.read[q.rpos] = zero // drop the reference so GC can reclaim it
	q.rpos++
	if q.rpos == len(q.read) {
		q.read = q.read[:0]
		q.rpos = 0
	}
	return v, true
}

// refill swaps the producer buffer into the consumer buffer. It reports the
// closed flag and whether the queue is still empty after the swap.
func (q *Queue[T]) refill() (closed, empty bool) {
	q.mu.Lock()
	closed = q.closed
	if len(q.write) > 0 {
		// Swap buffers: producers continue appending into the (now
		// empty) previous read slice, reusing its capacity.
		q.write, q.read = q.read[:0], q.write
		q.rpos = 0
	}
	q.mu.Unlock()
	return closed, q.rpos >= len(q.read)
}

// notify wakes the consumer without ever blocking the caller.
func (q *Queue[T]) notify() {
	select {
	case q.signal <- struct{}{}:
	default:
	}
}
