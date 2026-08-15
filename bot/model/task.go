package model

import "time"

type Task struct {
    ID           string
    Interval     time.Duration      // How often to run
    LastRun      time.Time
    NextRun      time.Time
    Retrieve     func() (string, error) // The work to do
}

func NewTask(id string, interval time.Duration, task func() (string, error)) *Task {
    return &Task{
        ID:        id,
        Retrieve:  task,
        Interval:  interval,
        LastRun:   time.Now(),
        NextRun:   time.Now().Add(interval),
    }
}

type TaskResult struct {
    TaskID    string
    Content   string
    Error     error
    Timestamp time.Time
}


type TaskScheduler struct {
    tasks     map[string]*Task
    taskQueue chan *Task
    ticker    *time.Ticker
}

func NewTaskScheduler(taskQueue chan *Task) *TaskScheduler {
    return &TaskScheduler{
        tasks:     make(map[string]*Task),
        taskQueue: taskQueue,
        ticker:    time.NewTicker(3 * time.Second), // Check every 3 second
    }
}

// Add a new task
func (ts *TaskScheduler) AddTask(task *Task) {
    task.NextRun = time.Now()
    ts.tasks[task.ID] = task
}

// Run the scheduler
func (ts *TaskScheduler) Start() {
    go func() {
        for range ts.ticker.C {
            now := time.Now()
            
            // Check each task
            for _, task := range ts.tasks {
                if now.After(task.NextRun) {
                    // Time to run this task
                    ts.taskQueue <- task
                    
                    // Schedule next run
                    task.LastRun = now
                    task.NextRun = now.Add(task.Interval)
                }
            }
        }
    }()
}

func (ts *TaskScheduler) Stop() {
    ts.ticker.Stop()
}
