package scheduler

import "context"

type Parallel struct {
	isStarted bool
	eventCh   chan Event
	cancel    context.CancelFunc
}

func NewParallel() *Parallel {
	ctx, cancel := context.WithCancel(context.Background())

	parallel := &Parallel{
		cancel:  cancel,
		eventCh: make(chan Event, eventChanSize),
	}

	go parallel.runEventLoop(ctx)

	return parallel
}

func (parallel *Parallel) Start() error {
	parallel.eventCh <- Event{
		Type: SchedulerStarted,
	}

	return nil
}

func (parallel *Parallel) Pause() error {
	parallel.eventCh <- Event{
		Type: SchedulerPaused,
	}

	return nil
}

func (parallel *Parallel) Add(job *Job) error {
	parallel.eventCh <- Event{
		Type: JobAdded,
		Job:  job,
	}

	return nil
}

func (parallel *Parallel) runEventLoop(ctx context.Context) {
	for {
		select {
		case event := <-parallel.eventCh:
			parallel.handleEvent(ctx, event)

		case <-ctx.Done():
			return
		}
	}
}

func (parallel *Parallel) handleEvent(ctx context.Context, event Event) {
	switch event.Type {
	case JobAdded:
		if !parallel.isStarted {
			return
		}

		go parallel.execute(ctx, event.Job)

	case JobEnded:
		return

	case SchedulerStarted:
		parallel.isStarted = true

	case SchedulerPaused:
		parallel.isStarted = false
		if parallel.cancel != nil {
			parallel.cancel()
			parallel.cancel = nil
		}

	case QueryPipelineState:
		return
	}
}

func (parallel *Parallel) execute(ctx context.Context, job *Job) {
	err := job.fn(ctx)

	parallel.eventCh <- Event{
		Type: JobEnded,
	}

	job.SetState(Done)

	if err != nil {
		job.SetState(Failed)
	}
}
