package model

import "time"

type WorkerPool struct {
	workers    int
	TaskQueue  chan *Task
	resultChan chan TaskResult
}

func NewWorkerPool(numWorkers int) *WorkerPool {
	return &WorkerPool{
		workers:    numWorkers,
		TaskQueue:  make(chan *Task, 100), // Buffered queue
		resultChan: make(chan TaskResult),
	}
}

// Start workers
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		go wp.worker(i)
	}
}

// Individual worker
func (wp *WorkerPool) worker(id int) {
	for task := range wp.TaskQueue {
		// Execute the task
		content, err := task.Retrieve()

		result := TaskResult{
			TaskID:    task.ID,
			Content:   content,
			Error:     err,
			Timestamp: time.Now(),
		}

		wp.resultChan <- result
	}
}

// Stop workers
func (wp *WorkerPool) Stop() {
	close(wp.TaskQueue)
}
