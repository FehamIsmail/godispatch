// Task is the main struct for the task
// Has an ID, type,
package task

import (
	"time"
)

type Task struct {
	ID              string     `json:"id"`
	Type            string     `json:"type"`
	Payload         []byte     `json:"payload"`
	Status          Status     `json:"status"`
	Priority        int        `json:"priority"`
	ComplexityScore int        `json:"complexity_score"`
	Dependencies    []string   `json:"dependencies,omitempty"`
	RetryCount      int        `json:"retry_count"`
	MaxRetries      int        `json:"max_retries"`
	Deadline        *time.Time `json:"deadline,omitempty"`
	NextRetryAt     time.Time  `json:"next_retry_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	WorkerID        string     `json:"worker_id,omitempty"`
}

type Result struct {
	TaskID     string       `json:"task_id"`
	Status     Status       `json:"status"`
	Output     []byte       `json:"output,omitempty"`
	Error      string       `json:"error,omitempty"`
	StartTime  time.Time    `json:"start_time"`
	EndTime    time.Time    `json:"end_time"`
	RetryCount int          `json:"retry_count"`
	WorkerID   string       `json:"worker_id"`
	Metrics    *TaskMetrics `json:"metrics,omitempty"`
}

type TaskMetrics struct {
	ProcessingTime time.Duration `json:"processing_time"`
	QueueWaitTime  time.Duration `json:"queue_wait_time"`
	MemoryUsage    uint64        `json:"memory_usage"`
	CPUTime        float64       `json:"cpu_time"`
}
