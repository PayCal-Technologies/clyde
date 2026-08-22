package clyde

import "testing"

func TestStatusStoreFormatsEmpty(t *testing.T) {
	store := NewStatusStore()
	if got := FormatStatus(store.Get("")); got != "No jobs." {
		t.Fatalf("unexpected status: %s", got)
	}
}

func TestStatusStoreRecordsEvent(t *testing.T) {
	store := NewStatusStore()
	store.Event(ProgressEvent{JobID: "sync", Phase: "uploading", Message: "uploading app.go", Done: 1, Total: 2})
	got := FormatStatus(store.Get(""))
	if got != "sync: uploading 1/2 - uploading app.go" {
		t.Fatalf("unexpected status: %s", got)
	}
}

func TestStatusStoreSnapshotsEvents(t *testing.T) {
	store := NewStatusStore()
	first := store.Event(ProgressEvent{JobID: "sync", Phase: "uploading", Message: "one", Done: 1, Total: 2})
	first.Events[0].Message = "mutated"

	got := store.Get("sync")["job"].(JobStatus)
	if got.Events[0].Message != "one" {
		t.Fatalf("store leaked mutable events: %#v", got.Events)
	}
}

func TestStatusURLFormatsIPv6(t *testing.T) {
	if got := StatusURL("::1", 5876); got != "http://[::1]:5876/rpc" {
		t.Fatalf("unexpected URL: %s", got)
	}
}
