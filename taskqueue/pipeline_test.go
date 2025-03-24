package taskqueue

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPipeline(t *testing.T) {
	jobDuration := 30 * time.Second
	quickJobDuration := 100 * time.Millisecond
	interStepDelay := 100 * time.Millisecond

	timedJob := mustCreateJob(t, func(ctx context.Context) error {
		time.Sleep(jobDuration)
		return nil
	})
	quickJob := mustCreateJob(t, func(ctx context.Context) error {
		time.Sleep(quickJobDuration)
		return nil
	})
	failedJob := mustCreateJob(t, func(ctx context.Context) error {
		return errors.New("forcing job to fail")
	})

	pipeline, err := NewPipeline("handler-1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if err := pipeline.Start(); err != nil {
		t.Fatalf("could not start pipeline: %v", err)
	}

	jobs := []*Job{timedJob, quickJob, failedJob}

	start := time.Now()

	for _, jb := range jobs {
		if err := pipeline.Add(jb); err != nil {
			t.Fatalf("could not add to pipeline: %v", err)
		}
	}

	time.Sleep(interStepDelay)

	t.Run("should only be running first job", func(tt *testing.T) {
		current := mustGetPipelineState(tt, pipeline)

		now := time.Now()
		elapsed := now.Sub(start)

		if elapsed >= jobDuration {
			tt.Fatalf("took too long to reach test: %s", elapsed.String())
		}

		compareState(tt, Running, current, timedJob.ID)

		for _, j := range jobs[1:] {
			compareState(tt, Queued, current, j.ID)
		}
	})

	timeElapsed := time.Since(start)
	if jobDuration >= timeElapsed {
		timeLeft := jobDuration - timeElapsed
		time.Sleep(timeLeft + (50 * time.Millisecond))
	}

	t.Run("should run the second job", func(tt *testing.T) {
		current := mustGetPipelineState(tt, pipeline)

		compareState(tt, Done, current, timedJob.ID)
		compareState(tt, Running, current, quickJob.ID)
		compareState(tt, Queued, current, failedJob.ID)

		time.Sleep(500 * time.Millisecond)
	})

	time.Sleep(interStepDelay)

	t.Run("should fail the third job", func(tt *testing.T) {
		current := mustGetPipelineState(tt, pipeline)

		compareState(tt, Done, current, timedJob.ID)
		compareState(tt, Done, current, quickJob.ID)
		compareState(tt, Failed, current, failedJob.ID)
	})
}

func compareState(t *testing.T, want JobState, pipelineState PipelineState, id string) {
	t.Helper()

	got, err := pipelineState.Check(id)
	if err != nil {
		t.Fatalf("error getting job: %v", err)
	}

	if want != got {
		t.Fatalf("(state) got '%s', want '%s'", got, want)
	}
}

func mustCreateJob(t *testing.T, fn JobFn) *Job {
	t.Helper()

	job, err := NewJob(fn)
	if err != nil {
		t.Fatalf("error creating job: %v", err)
	}

	return job
}

func mustGetPipelineState(t *testing.T, pipeline *Pipeline) PipelineState {
	t.Helper()

	state, err := pipeline.State()
	if err != nil {
		t.Fatalf("error getting pipeline state: %v", err)
	}

	return state
}
