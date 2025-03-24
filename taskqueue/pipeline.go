package taskqueue

import (
	"context"
	"errors"
	"time"
)

type Event struct {
	Type EventType
	Job  *Job

	responseCh chan<- PipelineState
}

type EventType string

const (
	JobAdded           EventType = "job-added"
	JobEnded           EventType = "job-ended"
	PipelineStarted    EventType = "pipeline-started"
	PipelinePaused     EventType = "pipeline-ended"
	QueryPipelineState EventType = "query-pipeline-state"
)

const eventChanSize = 100

func NewPipeline(name string) (*Pipeline, error) {
	ctx, cancel := context.WithCancel(context.Background())

	pipeline := &Pipeline{
		Name:    name,
		cancel:  cancel,
		eventCh: make(chan Event, eventChanSize),
	}

	go pipeline.runEventLoop(ctx)

	return pipeline, nil
}

type Pipeline struct {
	jobs         []*Job
	currentIndex int
	cancel       context.CancelFunc
	eventCh      chan Event
	isStarted    bool

	Name string
}

func (pipeline *Pipeline) Add(job *Job) error {
	pipeline.eventCh <- Event{
		Type: JobAdded,
		Job:  job,
	}
	return nil
}

func (pipeline *Pipeline) runEventLoop(ctx context.Context) {
	for {
		select {
		case event := <-pipeline.eventCh:
			pipeline.handleEvent(ctx, event)

		case <-ctx.Done():
			return
		}
	}
}

func (pipeline *Pipeline) handleEvent(ctx context.Context, event Event) {
	switch event.Type {
	case JobAdded:
		job := event.Job
		job.timeAdded = time.Now()
		job.SetState(Queued)

		pipeline.jobs = append(pipeline.jobs, job)
		pipeline.runNextJob(ctx)

	case JobEnded:
		pipeline.runNextJob(ctx)

	case QueryPipelineState:
		state := PipelineState{
			jobStates: make(map[string]JobState),
		}

		for _, job := range pipeline.jobs {
			state.jobStates[job.ID] = job.GetState()
		}

		event.responseCh <- state

	case PipelineStarted:
		pipeline.isStarted = true

	case PipelinePaused:
		pipeline.isStarted = false
		if pipeline.cancel != nil {
			pipeline.cancel()
			pipeline.cancel = nil
		}
	}
}

func (pipeline *Pipeline) runNextJob(ctx context.Context) {
	if !pipeline.isStarted {
		return
	}

	index := pipeline.currentIndex
	n := len(pipeline.jobs)

	if index >= (n - 1) {
		return
	}

	current := pipeline.jobs[index]
	state := current.GetState()

	if state == Running {
		return
	}

	if state == Queued {
		current.SetState(Running)
		go pipeline.execute(ctx, current)
		return
	}

	pipeline.currentIndex++
	job := pipeline.jobs[pipeline.currentIndex]
	job.SetState(Running)
	go pipeline.execute(ctx, job)
}

func (pipeline *Pipeline) execute(ctx context.Context, job *Job) {
	err := job.fn(ctx)

	job.SetState(Done)
	if err != nil {
		job.SetState(Failed)
	}

	pipeline.eventCh <- Event{
		Type: JobEnded,
	}
}

func (pipeline *Pipeline) Start() error {
	pipeline.eventCh <- Event{
		Type: PipelineStarted,
	}
	return nil
}

func (pipeline *Pipeline) Pause() error {
	return nil
}

var ErrQueryPipelineTimeout = errors.New("timed out waiting for pipeline state")

func (pipeline *Pipeline) State() (PipelineState, error) {
	const queryPipelineStateTimeout = 15 * time.Second

	responseCh := make(chan PipelineState, 1)

	pipeline.eventCh <- Event{
		Type:       QueryPipelineState,
		responseCh: responseCh,
	}

	select {
	case <-time.After(queryPipelineStateTimeout):
		return PipelineState{}, ErrQueryPipelineTimeout
	case state := <-responseCh:
		return state, nil
	}
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
