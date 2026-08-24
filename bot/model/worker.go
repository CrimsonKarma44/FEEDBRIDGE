package model

import (
	"context"
	"log"
	"sync"
	"time"
)

const (
	workerCount    = 10
	taskQueueSize  = 100
	resultChanSize = 100
)

type WorkerPool struct {
	workers    int
	TaskQueue  chan *Task
	ResultChan chan TaskResult

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func WorkerCount() int {
	return workerCount
}

func NewWorkerPool(numWorkers int) *WorkerPool {
	if numWorkers <= 0 {
		numWorkers = workerCount
	}
	return &WorkerPool{
		workers:    numWorkers,
		TaskQueue:  make(chan *Task, taskQueueSize),
		ResultChan: make(chan TaskResult, resultChanSize),
	}
}

func (wp *WorkerPool) Start(ctx context.Context) {
	wp.ctx, wp.cancel = context.WithCancel(ctx)
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
	go wp.consumeResults()
}

func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()
	for {
		select {
		case <-wp.ctx.Done():
			return
		case task := <-wp.TaskQueue:
			if task == nil || task.Run == nil {
				continue
			}
			err := task.Run(wp.ctx)
			select {
			case wp.ResultChan <- TaskResult{TaskID: task.ID, Error: err, Timestamp: time.Now()}:
			case <-wp.ctx.Done():
				return
			}
		}
	}
}

func (wp *WorkerPool) consumeResults() {
	for {
		select {
		case <-wp.ctx.Done():
			return
		case res := <-wp.ResultChan:
			if res.Error != nil {
				log.Printf("task %s failed: %v", res.TaskID, res.Error)
			}
		}
	}
}

func (wp *WorkerPool) Stop() {
	if wp.cancel != nil {
		wp.cancel()
	}
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		log.Println("worker pool shutdown timed out")
	}
}
