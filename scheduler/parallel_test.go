package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestParallelJobs(t *testing.T) {
	parallel, err := NewParallel("test-handler")
	if err != nil {
		t.Fatalf("could not create scheduler: %v", err)
	}

	if err := parallel.Start(); err != nil {
		t.Fatalf("could not start scheduler: %v", err)
	}

	const (
		jobDuration = 500 * time.Millisecond
		jobCount    = 5
	)

	jobs := make([]*Job, 0, jobCount)

	waitgroup := sync.WaitGroup{}
	waitgroup.Add(jobCount)

	for k := range jobCount {
		jobs = append(jobs, mustCreateJob(t, func(context.Context) error {
			time.Sleep(jobDuration)
			waitgroup.Done()
			return nil
		}))

		if err := parallel.Add(jobs[k]); err != nil {
			t.Fatalf("could not add job: %v", err)
		}
	}

	done := make(chan struct{}, 1)
	go func() {
		waitgroup.Wait()
		done <- struct{}{}
	}()

	t.Run("it should execute all jobs", func(tt *testing.T) {
		select {
		case <-time.After(jobDuration + (50 * time.Millisecond)):
			tt.Fatalf("timed out waiting for jobs to run")

		case <-done:
			return
		}
	})
}
