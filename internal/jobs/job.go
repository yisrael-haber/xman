package jobs

import "time"

// Status represents the lifecycle state of a job.
type Status string

const (
	StatusRunning   Status = "running"
	StatusDone      Status = "done"
	StatusCancelled Status = "cancelled"
	StatusFailed    Status = "failed"
)

// LogEntry is a single timestamped entry in a job's history.
type LogEntry struct {
	Progress  int       `json:"progress"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Job represents a single long-running operation.
type Job struct {
	ID        string     `json:"id"`
	Feature   string     `json:"feature"`
	Label     string     `json:"label"`
	Status    Status     `json:"status"`
	Progress  int        `json:"progress"` // 0-100
	Message   string     `json:"message"`  // current status message (last emit)
	Error     string     `json:"error,omitempty"`
	Log       []LogEntry `json:"log"` // full history of all emits
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`

	cancel func()
}

// Event is the compact per-update payload emitted to the frontend.
type Event struct {
	ID        string     `json:"id"`
	Feature   string     `json:"feature"`
	Label     string     `json:"label"`
	Status    Status     `json:"status"`
	Progress  int        `json:"progress"`
	Message   string     `json:"message"`
	Error     string     `json:"error,omitempty"`
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
	Log       []LogEntry `json:"log,omitempty"`
	LogLength int        `json:"logLength"`
	LastLog   *LogEntry  `json:"lastLog,omitempty"`
}

// Clone returns a stable copy suitable for returning through bindings.
func (j *Job) Clone() *Job {
	if j == nil {
		return nil
	}

	clone := *j
	clone.Log = append([]LogEntry(nil), j.Log...)
	clone.cancel = nil
	return &clone
}

// EventSnapshot returns a compact snapshot of the job for runtime events.
func (j *Job) EventSnapshot() Event {
	event := Event{
		ID:        j.ID,
		Feature:   j.Feature,
		Label:     j.Label,
		Status:    j.Status,
		Progress:  j.Progress,
		Message:   j.Message,
		Error:     j.Error,
		StartedAt: j.StartedAt,
		EndedAt:   j.EndedAt,
		LogLength: len(j.Log),
	}
	if j.Status != StatusRunning {
		event.Log = append([]LogEntry(nil), j.Log...)
	}
	if len(j.Log) > 0 {
		last := j.Log[len(j.Log)-1]
		event.LastLog = &last
	}
	return event
}

// Cancel requests cancellation of the job.
func (j *Job) Cancel() {
	if j.cancel != nil {
		j.cancel()
	}
}

// EmitFn is passed to job work functions so they can report progress back to the frontend.
type EmitFn func(progress int, message string)
