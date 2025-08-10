package main

import (
	"sync"
)

// Direct import of https://github.com/pwaller/barrier/blob/master/barrier.go

// The zero of Barrier is a ready-to-use value

// The zero of Barrier is a ready-to-use value
type Barrier struct {
	channel            chan struct{}
	initOnce, fallOnce sync.Once

	m sync.Mutex // Protects "forwards" and "backwards"
	// List of barriers to forward to
	forwards map[*Barrier]struct{}
	// List of barriers that might hold a reference to this one.
	// When this barrier falls, those barriers should forget about us to avoid
	// unbounded memory growth.
	backwards map[*Barrier]struct{}

	// An optional hook, which if set, is called exactly once when the first
	// b.Fall() is invoked.
	FallHook func()
}

func (b *Barrier) init() {
	b.initOnce.Do(func() {
		b.channel = make(chan struct{})

		b.m.Lock()
		defer b.m.Unlock()
		b.forwards = map[*Barrier]struct{}{}
		b.backwards = map[*Barrier]struct{}{}
	})
}

// Forward will cause `f.Fall()` to be invoked if `b.Fall()` is invoked.
// The implementation ensures that any reference `b` holds to `f` is removed
// if `f` falls.
func (b *Barrier) Forward(f *Barrier) {
	b.init()

	func() {
		b.m.Lock()
		defer b.m.Unlock()

		select {
		case <-b.channel:
			// Barrier has already fallen, forward the signal immediately
			f.Fall()
			return
		default:
		}
		b.forwards[f] = struct{}{}
	}()

	// Ensure f is init'd and make sure it knows to notify `b` when it falls.
	f.init()
	f.m.Lock()
	defer f.m.Unlock()
	f.backwards[b] = struct{}{}
}

// `b.Fall()` can be called any number of times and causes the channel returned
// by `b.Barrier()` to become closed (permanently available for immediate reading)
func (b *Barrier) Fall() {
	b.init()

	b.fallOnce.Do(func() {
		b.m.Lock()
		if b.FallHook != nil {
			b.FallHook()
		}
		close(b.channel)
		b.m.Unlock()

		// When `b` is fired, all `f`s are fired
		for forward := range b.forwards {
			forward.Fall()
		}
		b.forwards = nil // lose any references to f

		// When `f` is fired, no `b` ever needs to know about us anymore.
		for backward := range b.backwards {
			func() {
				backward.m.Lock()
				defer backward.m.Unlock()
				delete(backward.forwards, b)
			}()
		}
	})
}

// When `b.Fall()` is called, the channel returned by Barrier() is closed
// (and becomes always readable)
func (b *Barrier) Barrier() <-chan struct{} {
	b.init()
	return b.channel
}
