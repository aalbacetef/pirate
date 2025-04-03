package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDrop(t *testing.T) {
	const jobDuration = 500 * time.Millisecond
	drop, err := NewDrop("test-handler")
	if err != nil {
		t.Fatalf("could not create scheduler: %v", err)
	}

	if err := drop.Start(); err != nil {
		t.Fatalf("could not start scheduler: %v", err)
	}

	t.Run("it should execute the job", func(tt *testing.T) {
		jobDoneChan := make(chan struct{}, 1)

		singleJob := mustCreateJob(tt, func(context.Context) error {
			time.Sleep(jobDuration)
			jobDoneChan <- struct{}{}

			return nil
		})

		if err := drop.Add(singleJob); err != nil {
			tt.Fatalf("could not add job: %v", err)
		}

		select {
		case <-jobDoneChan:
			return

		case <-time.After(2 * jobDuration):
			tt.Fatalf("timed out waiting for job to finish")
		}
	})

	t.Run("it should drop jobs if one is running", func(tt *testing.T) {
		const jobCount = 3
		jobs := make([]*Job, 0, jobCount)

		for range jobCount {
			job := mustCreateJob(tt, func(context.Context) error {
				return nil
			})
			jobs = append(jobs, job)
		}

		if err := drop.Add(mustCreateJob(tt, func(context.Context) error {
			time.Sleep(jobDuration)
			return nil
		})); err != nil {
			tt.Fatalf("could not add job: %v", err)
		}

		for k := range jobCount {
			err := drop.Add(jobs[k])
			if !errors.Is(err, ErrJobDropped) {
				tt.Fatalf("expected '%v', got '%v'", ErrJobDropped, err)
			}
		}
	})
}
