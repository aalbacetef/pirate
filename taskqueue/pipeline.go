package taskqueue

import (
	"context"
	"fmt"
	"sync"

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
	id := uuid.NewUUID()

	return &Job{ID: id}, nil
}

type Job struct {
	ID    string
	state JobState
	mu    sync.Mutex
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

func NewPipeline(name string) (*Pipeline, error) {
	return &Pipeline{Name: name}, nil
}

type Pipeline struct {
	mu           sync.Mutex
	jobs         []*Job
	currentIndex int
	cancel       context.CancelFunc
	inputJobChan chan *Job

	Name string
}

func (pipeline *Pipeline) Add(job *Job) error {
	pipeline.mu.Lock()
	defer pipeline.mu.Unlock()

	pipeline.jobs = append(pipeline.jobs, job)

	return nil
}

func (pipeline *Pipeline) Start() error {
}

func (pipeline *Pipeline) Pause() error {

}

func (pipeline *Pipeline) State() PipelineState {
	pipeline.mu.Lock()
	defer pipeline.mu.Unlock()

	state := PipelineState{
		jobStates: make(map[string]JobState),
	}

	for _, job := range pipeline.jobs {
		state.jobStates[job.ID] = job.GetState()
	}

	return state
}

type PipelineState struct {
	jobStates map[string]JobState
}

func (ps PipelineState) Check(id string) (JobState, error) {
	for jobID, jobState := range ps.jobStates {
		if jobID == id {
			return jobState, nil
		}
	}

	return "", JobNotFoundError{id}
}
