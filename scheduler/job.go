package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type JobNotFoundError struct {
	id string
}

func (e JobNotFoundError) Error() string {
	return fmt.Sprintf("job with id '%s' not found", e.id)
}

type JobState string

const (
	NotStarted JobState = "not-started"
	Queued     JobState = "queued"
	Running    JobState = "running"
	Failed     JobState = "failed"
	Done       JobState = "done"
)

type JobFn func(context.Context) error

func NewJob(fn JobFn) (*Job, error) {
	id := uuid.New().String()

	return &Job{
		ID:          id,
		fn:          fn,
		timeCreated: time.Now(),
	}, nil
}

type Job struct {
	ID          string
	state       JobState
	mu          sync.Mutex
	timeAdded   time.Time
	timeCreated time.Time
	fn          JobFn
}

func (job *Job) SetState(state JobState) {
	job.mu.Lock()
	job.state = state
	job.mu.Unlock()
}

func (job *Job) GetState() JobState {
	job.mu.Lock()
	state := job.state
	job.mu.Unlock()

	return state
}
