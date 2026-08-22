package clyde

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
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

type JobStatus struct {
	JobID     string          `json:"job_id"`
	Phase     string          `json:"phase"`
	Message   string          `json:"message"`
	Done      int             `json:"done"`
	Total     int             `json:"total"`
	RelPath   string          `json:"rel_path,omitempty"`
	Error     string          `json:"error,omitempty"`
	UpdatedAt float64         `json:"updated_at"`
	Events    []ProgressEvent `json:"events,omitempty"`
}

type StatusStore struct {
	mu   sync.Mutex
	jobs map[string]*JobStatus
}

func NewStatusStore() *StatusStore {
	return &StatusStore{jobs: map[string]*JobStatus{}}
}

func (s *StatusStore) Event(event ProgressEvent) JobStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	if event.Timestamp == 0 {
		event.Timestamp = float64(time.Now().UnixNano()) / 1e9
	}
	job := s.jobs[event.JobID]
	if job == nil {
		job = &JobStatus{JobID: event.JobID}
		s.jobs[event.JobID] = job
	}
	job.Phase = event.Phase
	job.Message = event.Message
	job.Done = event.Done
	job.Total = event.Total
	job.RelPath = event.RelPath
	job.Error = event.Error
	job.UpdatedAt = event.Timestamp
	job.Events = append(job.Events, event)
	if len(job.Events) > 500 {
		job.Events = job.Events[len(job.Events)-500:]
	}
	return *job
}

func (s *StatusStore) Get(jobID string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	if jobID != "" {
		job := s.jobs[jobID]
		if job == nil {
			return map[string]any{"job": nil}
		}
		return map[string]any{"job": *job}
	}
	jobs := make([]JobStatus, 0, len(s.jobs))
	for _, job := range s.jobs {
		summary := *job
		summary.Events = nil
		jobs = append(jobs, summary)
	}
	return map[string]any{"jobs": jobs}
}

func (s *StatusStore) Reset(jobID string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	if jobID != "" {
		_, ok := s.jobs[jobID]
		delete(s.jobs, jobID)
		return map[string]any{"removed": ok}
	}
	count := len(s.jobs)
	s.jobs = map[string]*JobStatus{}
	return map[string]any{"removed": count}
}

func ServeStatus(host string, port int, out io.Writer) error {
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return errf("Clyde daemon only binds to localhost addresses")
	}
	store := NewStatusStore()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/status") {
			writeJSON(w, http.StatusOK, store.Get(r.URL.Query().Get("job_id")))
			return
		}
		if r.Method == http.MethodPost && (r.URL.Path == "/" || r.URL.Path == "/rpc") {
			handleRPC(w, r, store)
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	})
	server := &http.Server{Addr: fmt.Sprintf("%s:%d", host, port), Handler: mux}
	fmt.Fprintf(out, "clyde daemon listening on http://%s:%d/rpc\n", host, port)
	return server.ListenAndServe()
}

func handleRPC(w http.ResponseWriter, r *http.Request, store *StatusStore) {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONRPCBodyBytes)
	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      any             `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusOK, rpcError(nil, -32700, "invalid json: "+err.Error()))
		return
	}
	if req.JSONRPC != "2.0" {
		writeJSON(w, http.StatusOK, rpcError(req.ID, -32600, "expected jsonrpc 2.0"))
		return
	}
	switch req.Method {
	case "daemon.ping":
		writeJSON(w, http.StatusOK, rpcResult(req.ID, map[string]bool{"ok": true}))
	case "status.get":
		var params struct {
			JobID string `json:"job_id"`
		}
		_ = json.Unmarshal(req.Params, &params)
		writeJSON(w, http.StatusOK, rpcResult(req.ID, store.Get(params.JobID)))
	case "status.reset":
		var params struct {
			JobID string `json:"job_id"`
		}
		_ = json.Unmarshal(req.Params, &params)
		writeJSON(w, http.StatusOK, rpcResult(req.ID, store.Reset(params.JobID)))
	case "status.event":
		var event ProgressEvent
		if err := json.Unmarshal(req.Params, &event); err != nil {
			writeJSON(w, http.StatusOK, rpcError(req.ID, -32602, "invalid status event: "+err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, rpcResult(req.ID, store.Event(event)))
	default:
		writeJSON(w, http.StatusOK, rpcError(req.ID, -32601, "unknown method: "+req.Method))
	}
}

func StatusURL(host string, port int) string {
	return fmt.Sprintf("http://%s:%d/rpc", host, port)
}

func FetchStatus(url, jobID string) (map[string]any, error) {
	params := map[string]string{}
	if jobID != "" {
		params["job_id"] = jobID
	}
	var result map[string]any
	if err := rpcCall(url, "status.get", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func FormatStatus(result map[string]any) string {
	if job, ok := result["job"]; ok {
		if job == nil {
			return "No matching job."
		}
		if data, ok := job.(map[string]any); ok {
			return formatJob(data)
		}
		encoded, _ := json.Marshal(job)
		var decoded map[string]any
		_ = json.Unmarshal(encoded, &decoded)
		return formatJob(decoded)
	}
	switch jobs := result["jobs"].(type) {
	case []JobStatus:
		if len(jobs) == 0 {
			return "No jobs."
		}
		lines := make([]string, 0, len(jobs))
		for _, job := range jobs {
			encoded, _ := json.Marshal(job)
			var decoded map[string]any
			_ = json.Unmarshal(encoded, &decoded)
			lines = append(lines, formatJob(decoded))
		}
		return strings.Join(lines, "\n")
	case []any:
		if len(jobs) == 0 {
			return "No jobs."
		}
		lines := make([]string, 0, len(jobs))
		for _, job := range jobs {
			encoded, _ := json.Marshal(job)
			var decoded map[string]any
			_ = json.Unmarshal(encoded, &decoded)
			lines = append(lines, formatJob(decoded))
		}
		return strings.Join(lines, "\n")
	}
	if result["jobs"] == nil {
		return "No jobs."
	}
	return "No jobs."
}

func formatJob(job map[string]any) string {
	done := numberInt(job["done"])
	total := numberInt(job["total"])
	progress := fmt.Sprintf("%d", done)
	if total > 0 {
		progress = fmt.Sprintf("%d/%d", done, total)
	}
	line := fmt.Sprintf("%v: %v %s - %v", job["job_id"], job["phase"], progress, job["message"])
	if job["error"] != nil && job["error"] != "" {
		line += fmt.Sprintf(" (%v)", job["error"])
	}
	return line
}

func terminalStatus(result map[string]any) bool {
	var jobs []any
	if job, ok := result["job"]; ok {
		if job != nil {
			jobs = []any{job}
		}
	} else {
		jobs, _ = result["jobs"].([]any)
	}
	if len(jobs) == 0 {
		return false
	}
	for _, job := range jobs {
		encoded, _ := json.Marshal(job)
		var decoded map[string]any
		_ = json.Unmarshal(encoded, &decoded)
		phase, _ := decoded["phase"].(string)
		if phase != "complete" && phase != "failed" {
			return false
		}
	}
	return true
}
