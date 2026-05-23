// Package progress tracks long-running task progress and broadcasts state
// changes to subscribers (e.g. SSE clients).
package progress

import "sync"

type State struct {
	Total  int  `json:"total"`
	Done   int  `json:"done"`
	Active bool `json:"active"`
}

type Tracker struct {
	mu    sync.Mutex
	state State
	subs  map[chan State]struct{}
}

func New() *Tracker {
	return &Tracker{subs: map[chan State]struct{}{}}
}

// Start resets the tracker for a new run of total items.
func (t *Tracker) Start(total int) {
	t.mu.Lock()
	t.state = State{Total: total, Done: 0, Active: total > 0}
	t.mu.Unlock()
	t.broadcast()
}

// Inc records one completed item.
func (t *Tracker) Inc() {
	t.mu.Lock()
	t.state.Done++
	if t.state.Done >= t.state.Total {
		t.state.Active = false
	}
	t.mu.Unlock()
	t.broadcast()
}

// Finish marks the run inactive regardless of counts.
func (t *Tracker) Finish() {
	t.mu.Lock()
	t.state.Active = false
	t.mu.Unlock()
	t.broadcast()
}

func (t *Tracker) State() State {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

// Subscribe returns a channel that receives state snapshots. The current
// state is sent immediately. Callers must call Unsubscribe when done.
func (t *Tracker) Subscribe() chan State {
	ch := make(chan State, 4)
	t.mu.Lock()
	t.subs[ch] = struct{}{}
	s := t.state
	t.mu.Unlock()
	ch <- s
	return ch
}

func (t *Tracker) Unsubscribe(ch chan State) {
	t.mu.Lock()
	if _, ok := t.subs[ch]; ok {
		delete(t.subs, ch)
		close(ch)
	}
	t.mu.Unlock()
}

func (t *Tracker) broadcast() {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.state
	for ch := range t.subs {
		select {
		case ch <- s:
		default:
		}
	}
}
