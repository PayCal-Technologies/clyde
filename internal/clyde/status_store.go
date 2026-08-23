package clyde

import (
	"sync"
	"time"
)

const maxStatusJobs = 1000

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
	mu    sync.Mutex
	jobs  map[string]*JobStatus
	order []string
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
		if len(s.jobs) >= maxStatusJobs && len(s.order) > 0 {
			delete(s.jobs, s.order[0])
			s.order = s.order[1:]
		}
		job = &JobStatus{JobID: event.JobID}
		s.jobs[event.JobID] = job
		s.order = append(s.order, event.JobID)
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
	return cloneJobStatus(*job)
}

func (s *StatusStore) Get(jobID string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	if jobID != "" {
		job := s.jobs[jobID]
		if job == nil {
			return map[string]any{"job": nil}
		}
		return map[string]any{"job": cloneJobStatus(*job)}
	}
	jobs := make([]JobStatus, 0, len(s.jobs))
	for _, job := range s.jobs {
		summary := cloneJobStatus(*job)
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
		s.removeOrderLocked(jobID)
		return map[string]any{"removed": ok}
	}
	count := len(s.jobs)
	s.jobs = map[string]*JobStatus{}
	s.order = nil
	return map[string]any{"removed": count}
}

func (s *StatusStore) removeOrderLocked(jobID string) {
	for i, id := range s.order {
		if id == jobID {
			s.order = append(s.order[:i], s.order[i+1:]...)
			return
		}
	}
}

func cloneJobStatus(job JobStatus) JobStatus {
	if len(job.Events) > 0 {
		job.Events = append([]ProgressEvent(nil), job.Events...)
	}
	return job
}
