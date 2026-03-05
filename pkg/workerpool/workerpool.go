package workerpool

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
)

type WorkerPool[T, R any] struct {
	bufferSize int
	numWorkers int
}

func New[T, R any](bufferSize, numWorkers int) *WorkerPool[T, R] {
	if bufferSize < 0 {
		bufferSize = 0
	}

	if numWorkers <= 0 {
		numWorkers = 1
	}

	return &WorkerPool[T, R]{
		bufferSize: bufferSize,
		numWorkers: numWorkers,
	}
}

type job[T, R any] struct {
	data T
	fn   func(T) R
}

func (j job[T, R]) do() R {
	return j.fn(j.data)
}

type worker[T, R any] struct {
	jobs     <-chan job[T, R]
	results  chan<- R
	wg       *sync.WaitGroup
	workerID int
	errorFn  func(error)
}

func (w worker[T, R]) start(ctx context.Context) {
	defer w.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case j, ok := <-w.jobs:
			if !ok {
				return
			}

			func() {
				defer func() {
					if r := recover(); r != nil {
						buf := make([]byte, 4096)
						n := runtime.Stack(buf, false)

						err := fmt.Errorf("panic in jobFn: %v\nStack: %s", r, string(buf[:n]))

						if w.errorFn != nil {
							w.errorFn(err)
						} else {
							log.Printf("workerpool: %v\n", err)
						}
					}
				}()

				result := j.do()

				w.results <- result
			}()
		}
	}
}

// Do processes inputs in parallel using workers.
// - ctx: for cancellation/timeout.
// - inputFn: produces inputs until (T, false).
// - jobFn: processes each input.
// - outputFn: handles each result (called sequentially in separate goroutine).
// - errorFn: optional, handles errors/panics from jobFn.
func (wp *WorkerPool[T, R]) Do(
	ctx context.Context,
	inputFn func() (T, bool),
	jobFn func(val T) R,
	outputFn func(val R),
	errorFn func(err error),
) {
	var wg sync.WaitGroup

	jobs := make(chan job[T, R], wp.bufferSize)
	results := make(chan R, wp.bufferSize)

	for i := 0; i < wp.numWorkers; i++ {
		wg.Add(1)

		w := worker[T, R]{
			jobs:    jobs,
			results: results,
			wg:      &wg,
			errorFn: errorFn,
		}

		go w.start(ctx)
	}

	wg.Add(1)

	go func() {
		defer wg.Done()
		defer close(results)

		for {
			select {
			case <-ctx.Done():
				return
			case result, ok := <-results:
				if !ok {
					return
				}

				outputFn(result)
			}
		}
	}()

	go func() {
		defer close(jobs)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				input, ok := inputFn()
				if !ok {
					return
				}

				jobs <- job[T, R]{
					data: input,
					fn:   jobFn,
				}
			}
		}
	}()

	wg.Wait()
}
