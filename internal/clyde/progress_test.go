package clyde

import (
	"strings"
	"sync"
	"testing"
	"time"
)

type progressRecorder struct {
	mu     sync.Mutex
	events []ProgressEvent
}

func (r *progressRecorder) Emit(event ProgressEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *progressRecorder) snapshot() []ProgressEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]ProgressEvent(nil), r.events...)
}

func TestProgressHeartbeatReportsOngoingWork(t *testing.T) {
	recorder := &progressRecorder{}
	stop := startProgressHeartbeat(recorder, "sync", "scanning", "scanning repository", 0, 0, "", time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	stop()

	events := recorder.snapshot()
	if len(events) < 2 {
		t.Fatalf("expected initial and heartbeat events, got %#v", events)
	}
	if !strings.HasPrefix(events[len(events)-1].Message, "still scanning repository") {
		t.Fatalf("unexpected heartbeat: %#v", events)
	}
}
