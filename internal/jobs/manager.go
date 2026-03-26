package jobs

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Manager tracks all submitted jobs and emits progress events to the frontend.
type Manager struct {
	mu   sync.RWMutex
	jobs map[string]*Job
	ctx  context.Context // Wails runtime context, used for EventsEmit
}

// NewManager creates a Manager. ctx must be the Wails startup context.
func NewManager(ctx context.Context) *Manager {
	return &Manager{
		jobs: make(map[string]*Job),
		ctx:  ctx,
	}
}

// SetContext provides the Wails runtime context after startup.
// Also initialises the job map if this is the first call.
func (m *Manager) SetContext(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ctx = ctx
	if m.jobs == nil {
		m.jobs = make(map[string]*Job)
	}
}

// Submit starts a new job and returns its ID immediately.
// The work function receives a cancellable context and an emit function for progress.
func (m *Manager) Submit(feature, label string, fn func(ctx context.Context, emit EmitFn) error) string {
	id := uuid.New().String()
	jobCtx, cancel := context.WithCancel(context.Background())

	job := &Job{
		ID:        id,
		Feature:   feature,
		Label:     label,
		Status:    StatusRunning,
		StartedAt: time.Now(),
		cancel:    cancel,
	}

	m.mu.Lock()
	m.jobs[id] = job
	m.mu.Unlock()

	m.emit(job)

	go func() {
		emit := func(progress int, message string) {
			m.mu.Lock()
			job.Progress = progress
			job.Message = message
			job.Log = append(job.Log, LogEntry{
				Progress:  progress,
				Message:   message,
				Timestamp: time.Now(),
			})
			m.mu.Unlock()
			m.emit(job)
		}

		err := fn(jobCtx, emit)

		now := time.Now()
		m.mu.Lock()
		job.EndedAt = &now
		if err != nil {
			if jobCtx.Err() != nil {
				job.Status = StatusCancelled
			} else {
				job.Status = StatusFailed
				job.Error = err.Error()
			}
		} else {
			job.Status = StatusDone
			job.Progress = 100
		}
		m.mu.Unlock()

		m.emit(job)
		cancel()
	}()

	return id
}

// Cancel requests cancellation of the job with the given ID.
func (m *Manager) Cancel(id string) {
	m.mu.RLock()
	job, ok := m.jobs[id]
	m.mu.RUnlock()
	if ok {
		job.Cancel()
	}
}

// Get returns the current state of a job by ID.
func (m *Manager) Get(id string) (*Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.jobs[id]
	return job, ok
}

// List returns a snapshot of all jobs.
func (m *Manager) List() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, j)
	}
	return out
}

// emit sends a job update event to the Wails frontend.
// Event name: "job:<id>" — frontend listens per job ID.
func (m *Manager) emit(job *Job) {
	if m.ctx == nil {
		return
	}
	runtime.EventsEmit(m.ctx, "job:"+job.ID, job)
}

// --- Wails-exposed methods ---

// JobGet is the Wails binding to fetch a job's current state.
func (m *Manager) JobGet(id string) *Job {
	job, _ := m.Get(id)
	return job
}

// JobList is the Wails binding to list all jobs.
func (m *Manager) JobList() []*Job {
	return m.List()
}

// JobCancel is the Wails binding to cancel a running job.
func (m *Manager) JobCancel(id string) {
	m.Cancel(id)
}
