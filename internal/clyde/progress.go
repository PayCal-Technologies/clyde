package clyde

import (
	"fmt"
	"io"
	"time"
)

type ProgressEvent struct {
	JobID     string  `json:"job_id"`
	Phase     string  `json:"phase"`
	Message   string  `json:"message"`
	Done      int     `json:"done"`
	Total     int     `json:"total"`
	RelPath   string  `json:"rel_path,omitempty"`
	Error     string  `json:"error,omitempty"`
	Timestamp float64 `json:"timestamp"`
}

func NewProgressEvent(jobID, phase, message string, done, total int) ProgressEvent {
	return ProgressEvent{
		JobID:     jobID,
		Phase:     phase,
		Message:   message,
		Done:      done,
		Total:     total,
		Timestamp: float64(time.Now().UnixNano()) / 1e9,
	}
}

type ProgressSink interface {
	Emit(ProgressEvent)
}

type ConsoleSink struct {
	Writer io.Writer
}

func (s ConsoleSink) Emit(event ProgressEvent) {
	if s.Writer == nil {
		return
	}
	progress := fmt.Sprintf("%d", event.Done)
	if event.Total > 0 {
		progress = fmt.Sprintf("%d/%d", event.Done, event.Total)
	}
	detail := ""
	if event.RelPath != "" {
		detail = " - " + event.RelPath
	}
	errText := ""
	if event.Error != "" {
		errText = " (" + event.Error + ")"
	}
	fmt.Fprintf(s.Writer, "[%s] %s %s %s: %s%s%s\n",
		time.Unix(0, int64(event.Timestamp*1e9)).Format("15:04:05"),
		event.JobID,
		event.Phase,
		progress,
		event.Message,
		detail,
		errText,
	)
}

type TeeSink []ProgressSink

func (s TeeSink) Emit(event ProgressEvent) {
	for _, sink := range s {
		sink.Emit(event)
	}
}

type HTTPSink struct {
	URL string
}

func (s HTTPSink) Emit(event ProgressEvent) {
	_ = rpcCall(s.URL, "status.event", event, nil)
}

func emit(sink ProgressSink, jobID, phase, message string, done, total int, relPath string) {
	if sink == nil {
		return
	}
	event := NewProgressEvent(jobID, phase, message, done, total)
	event.RelPath = relPath
	sink.Emit(event)
}

func emitError(sink ProgressSink, jobID, phase, message string, done, total int, relPath string, err error) {
	if sink == nil {
		return
	}
	event := NewProgressEvent(jobID, phase, message, done, total)
	event.RelPath = relPath
	event.Error = err.Error()
	sink.Emit(event)
}
