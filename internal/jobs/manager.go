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
func (m *Manager) SetContext(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ctx = ctx
}

// SubmitWithParent starts a new job using the provided parent context.
// If parentCtx is nil, the Wails app context is used as a fallback.
func (m *Manager) SubmitWithParent(parentCtx context.Context, feature, label string, fn func(ctx context.Context, emit EmitFn) error) string {
	id := uuid.New().String()

	if parentCtx == nil {
		m.mu.RLock()
		parentCtx = m.ctx
		m.mu.RUnlock()
	}
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	jobCtx, cancel := context.WithCancel(parentCtx)

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
	if !ok {
		return nil, false
	}
	return job.Clone(), true
}

// List returns a snapshot of all jobs.
func (m *Manager) List() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, j.Clone())
	}
	return out
}

// Dismiss removes a completed/failed/cancelled job from the map.
func (m *Manager) Dismiss(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if ok && job.Status != StatusRunning {
		delete(m.jobs, id)
	}
}

// emit sends a job update event to the Wails frontend.
// Event name: "job:<id>" — frontend listens per job ID.
func (m *Manager) emit(job *Job) {
	m.mu.RLock()
	ctx := m.ctx
	event := job.EventSnapshot()
	m.mu.RUnlock()
	if ctx == nil {
		return
	}
	runtime.EventsEmit(ctx, "job:"+event.ID, event)
}
