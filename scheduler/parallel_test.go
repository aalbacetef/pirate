package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestParallelJobs(t *testing.T) {

	parallel := NewParallel()
	if err := parallel.Start(); err != nil {
		t.Fatalf("could not start scheduler: %v", err)
	}

	const (
		jobDuration = 500 * time.Millisecond
		n           = 5
	)
	jobs := make([]*Job, 0, n)
	wg := sync.WaitGroup{}
	wg.Add(n)

	for k := range n {
		jobs = append(jobs, mustCreateJob(t, func(context.Context) error {
			time.Sleep(jobDuration)
			wg.Done()
			return nil
		}))

		if err := parallel.Add(jobs[k]); err != nil {
			t.Fatalf("could not add job: %v", err)
		}
	}

	done := make(chan struct{}, 1)
	go func() {
		wg.Wait()
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
