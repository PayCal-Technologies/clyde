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
