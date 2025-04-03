package scheduler

import (
	"context"
	"errors"
	"time"
)

var ErrJobDropped = errors.New("job dropped")

func NewDrop(name string) (*Drop, error) {
	ctx, cancel := context.WithCancel(context.Background())

	drop := &Drop{
		eventCh: make(chan Event, eventChanSize),
		cancel:  cancel,
	}

	go drop.runEventLoop(ctx)

	return drop, nil
}

type Drop struct {
	currentJob *Job
	eventCh    chan Event
	cancel     context.CancelFunc
	isStarted  bool
	name       string
}

func (drop *Drop) Name() string {
	return drop.name
}

func (drop *Drop) Add(job *Job) error {
	isRunningCh := make(chan bool, 1)

	drop.eventCh <- Event{
		Type: JobAdded,
		Job:  job,

		isRunningCh: isRunningCh,
	}

	const waitForResponseTimeout = 100 * time.Millisecond

	select {
	case isRunning := <-isRunningCh:
		if isRunning {
			return ErrJobDropped
		}

		return nil
	case <-time.After(waitForResponseTimeout):
		return errors.New("timed out waiting for response")
	}
}

func (drop *Drop) Start() error {
	drop.eventCh <- Event{
		Type: SchedulerStarted,
	}
	return nil
}

func (drop *Drop) Pause() error {
	drop.eventCh <- Event{
		Type: SchedulerPaused,
	}

	return nil
}

func (drop *Drop) runEventLoop(ctx context.Context) {
	for {
		select {
		case event := <-drop.eventCh:
			drop.handleEvent(ctx, event)

		case <-ctx.Done():
			return
		}
	}
}

func (drop *Drop) handleEvent(ctx context.Context, event Event) {
	switch event.Type {
	case JobAdded:
		if !drop.isStarted {
			return
		}

		isAlreadyRunning := drop.currentJob != nil

		event.isRunningCh <- isAlreadyRunning
		if isAlreadyRunning {
			return
		}

		drop.currentJob = event.Job
		go drop.execute(ctx, drop.currentJob)

	case JobEnded:
		drop.currentJob = nil

	case SchedulerStarted:
		drop.isStarted = true

	case SchedulerPaused:
		drop.isStarted = false
		if drop.cancel != nil {
			drop.cancel()
			drop.cancel = nil
		}

	case QueryPipelineState:
		return
	}
}

func (drop *Drop) execute(ctx context.Context, job *Job) {
	err := job.fn(ctx)

	drop.eventCh <- Event{
		Type: JobEnded,
	}

	job.SetState(Done)

	if err != nil {
		job.SetState(Failed)
	}
}
