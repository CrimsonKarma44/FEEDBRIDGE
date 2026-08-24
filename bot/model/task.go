package model

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/CrimsonKarma44/FEEDBRIDGE/bot/store"
)

type Task struct {
	ID  string
	Run func(ctx context.Context) error
}

func NewTask(id string, run func(ctx context.Context) error) *Task {
	return &Task{ID: id, Run: run}
}

type TaskResult struct {
	TaskID    string
	Error     error
	Timestamp time.Time
}

const (
	schedulerTick   = 30 * time.Second
	maxTasksPerTick = 25
)

type TaskScheduler struct {
	mu        sync.Mutex
	tasks     map[string]*Task
	taskQueue chan<- *Task
	store     *store.Store
	factory   func(sub *store.Subscription) *Task
}

func NewTaskScheduler(taskQueue chan<- *Task, st *store.Store, factory func(sub *store.Subscription) *Task) *TaskScheduler {
	return &TaskScheduler{
		tasks:     make(map[string]*Task),
		taskQueue: taskQueue,
		store:     st,
		factory:   factory,
	}
}

func (ts *TaskScheduler) AddTask(task *Task) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.tasks[task.ID] = task
}

func (ts *TaskScheduler) RemoveTask(id string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	delete(ts.tasks, id)
}

func (ts *TaskScheduler) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(schedulerTick)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ts.dispatchDue(ctx)
			}
		}
	}()
}

func (ts *TaskScheduler) dispatchDue(ctx context.Context) {
	now := time.Now()

	due, err := ts.store.Due(now, maxTasksPerTick)
	if err != nil {
		log.Println("scheduler due query failed:", err)
		return
	}
	if len(due) == 0 {
		return
	}

	ids := make([]uint, len(due))
	for i := range due {
		ids[i] = due[i].ID
	}
	if err := ts.store.MarkChecked(ids, now); err != nil {
		log.Println("scheduler mark-checked failed:", err)
		return
	}

	for i := range due {
		sub := due[i]
		select {
		case ts.taskQueue <- ts.factory(&sub):
		case <-ctx.Done():
			return
		default:
			log.Printf("task queue full, dropping fetch for subscription %d", sub.ID)
		}
	}
}

func (ts *TaskScheduler) Stop() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.tasks = make(map[string]*Task)
}
