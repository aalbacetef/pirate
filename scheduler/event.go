package scheduler

type Event struct {
	Type EventType
	Job  *Job
	ID   string

	responseCh  chan<- PipelineState
	isRunningCh chan<- bool
}

type EventType string

const (
	JobAdded           EventType = "job-added"
	JobEnded           EventType = "job-ended"
	SchedulerStarted   EventType = "scheduler-started"
	SchedulerPaused    EventType = "scheduler-ended"
	QueryPipelineState EventType = "query-pipeline-state"
)

const eventChanSize = 100
