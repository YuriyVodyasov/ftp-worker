package scheduler

import (
	"context"
	"time"
)

type Doer interface {
	Do(context.Context)
}

type Scheduler struct {
	stop               chan struct{}
	processingInterval time.Duration
	doer               Doer
}

func New(processingInterval time.Duration, doer Doer) *Scheduler {
	return &Scheduler{
		stop:               make(chan struct{}),
		processingInterval: processingInterval,
		doer:               doer,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.processingInterval)

		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.doer.Do(ctx)
			case <-s.stop:
				ticker.Stop()

				return
			}
		}
	}()
}

func (s *Scheduler) Shutdown(ctx context.Context) {
	s.stop <- struct{}{}
}
