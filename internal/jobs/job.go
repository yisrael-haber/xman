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

// Job represents a single long-running operation.
type Job struct {
	ID        string     `json:"id"`
	Feature   string     `json:"feature"`
	Label     string     `json:"label"`
	Status    Status     `json:"status"`
	Progress  int        `json:"progress"`          // 0-100
	Message   string     `json:"message"`           // current status message
	Error     string     `json:"error,omitempty"`
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`

	cancel func()
}

// Cancel requests cancellation of the job.
func (j *Job) Cancel() {
	if j.cancel != nil {
		j.cancel()
	}
}

// EmitFn is passed to job work functions so they can report progress back to the frontend.
type EmitFn func(progress int, message string)
