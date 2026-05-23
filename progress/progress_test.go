package progress

import (
	"testing"
	"time"
)

func TestLateSubscriberReceivesCurrentState(t *testing.T) {
	tr := New()
	tr.Start(10)
	tr.Inc()
	tr.Inc()
	tr.Inc()

	ch := tr.Subscribe()
	defer tr.Unsubscribe(ch)

	select {
	case s := <-ch:
		if s.Total != 10 || s.Done != 3 || !s.Active {
			t.Errorf("expected Total=10 Done=3 Active=true, got %+v", s)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive initial state")
	}
}

func TestInactiveWhenComplete(t *testing.T) {
	tr := New()
	tr.Start(2)
	tr.Inc()
	tr.Inc()
	if s := tr.State(); s.Active {
		t.Errorf("expected inactive after all items done, got %+v", s)
	}
}

func TestStartWithZeroTotalIsInactive(t *testing.T) {
	tr := New()
	tr.Start(0)
	if s := tr.State(); s.Active {
		t.Errorf("expected inactive for zero-total run, got %+v", s)
	}
}

func TestBroadcastDoesNotBlockOnSlowSubscriber(t *testing.T) {
	tr := New()
	tr.Start(1000)
	ch := tr.Subscribe() // never read
	defer tr.Unsubscribe(ch)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			tr.Inc()
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast blocked on slow subscriber")
	}
}
