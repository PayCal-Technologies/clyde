package clyde

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
