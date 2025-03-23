package taskqueue

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPipeline(t *testing.T) {
	jobDuration := 1 * time.Minute

	timedJob := mustCreateJob(t, func(ctx context.Context) error {
		time.Sleep(jobDuration)
		return nil
	})
	quickJob := mustCreateJob(t, func(ctx context.Context) error {
		time.Sleep(100 * time.Second)
		return nil
	})
	failedJob := mustCreateJob(t, func(ctx context.Context) error {
		return errors.New("Unkownn error")
	})

	pipeline, err := NewPipeline("handler-1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if err := pipeline.Start(); err != nil {
		t.Fatalf("could not start pipeline: %v", err)
	}

	jobs := []*Job{timedJob, quickJob, failedJob}
	delay := 250 * time.Millisecond
	start := time.Now().Add(delay)

	for _, jb := range jobs {
		time.Sleep(delay)
		if err := pipeline.Add(jb); err != nil {
			t.Fatalf("could not add to pipeline: %v", err)
		}
	}

	t.Run("should only be running first job", func(tt *testing.T) {
		current := pipeline.State()

		now := time.Now()
		elapsed := now.Sub(start)

		if elapsed >= jobDuration {
			tt.Fatalf("took too long to reach test: %s", elapsed.String())
		}

		compareState(t, Running, current, timedJob.ID)

		for _, j := range jobs[1:] {
			compareState(t, Queued, current, j.ID)
		}
	})

	timeLeft := (time.Now()).Sub(start)
	if timeLeft > 0 {
		time.Sleep(timeLeft + (50 * time.Millisecond))
	}

	t.Run("should run the second job", func(tt *testing.T) {
		current := pipeline.State()

		compareState(t, Done, current, timedJob.ID)
		compareState(t, Running, current, quickJob.ID)
		compareState(t, Queued, current, failedJob.ID)

		time.Sleep(100 * time.Millisecond)
	})

	t.Run("should fail the second job", func(tt *testing.T) {
		current := pipeline.State()

		compareState(t, Done, current, timedJob.ID)
		compareState(t, Done, current, quickJob.ID)
		compareState(t, Failed, current, failedJob.ID)
	})

}

func compareState(t *testing.T, want JobState, pipelineState PipelineState, id string) {
	t.Helper()

	got, err := pipelineState.Check(id)
	if err != nil {
		t.Fatalf("error getting job: %v", err)
	}

	if want != got {
		t.Fatalf("(state) got '%s', want '%s'")
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
