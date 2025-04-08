// controller serves as the coordinator for the scheduler
package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"godispatch/dispatcher/config"
	"godispatch/dispatcher/task"
	"godispatch/dispatcher/worker"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// Result represents a task execution result
type Result struct {
	TaskID     string    `json:"task_id"`
	Status     string    `json:"status"`
	Output     []byte    `json:"output,omitempty"`
	Error      string    `json:"error,omitempty"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	RetryCount int       `json:"retry_count"`
	WorkerID   string    `json:"worker_id"`
}

// Schedule represents a task schedule
type Schedule struct {
	Type       string    `json:"type"`
	Expression string    `json:"expression,omitempty"`
	Interval   int64     `json:"interval,omitempty"`
	StartTime  time.Time `json:"start_time,omitempty"`
	EndTime    time.Time `json:"end_time,omitempty"`
	TimeZone   string    `json:"time_zone,omitempty"`
}

// Controller coordinates between scheduler and workers
type Controller struct {
	id          string
	config      *config.Config
	scheduler   *task.Scheduler
	workers     []*worker.Worker
	redisClient *redis.Client
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// New creates a new controller
func New(cfg *config.Config) (*Controller, error) {
	// Create Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Create controller
	controller := &Controller{
		id:          uuid.New().String(),
		config:      cfg,
		redisClient: redisClient,
		stopCh:      make(chan struct{}),
	}

	// Create scheduler
	scheduler := task.NewScheduler(&cfg.Scheduler, redisClient)
	controller.scheduler = scheduler

	// Create workers
	for i := 0; i < cfg.Worker.Count; i++ {
		w := worker.NewWorker(&cfg.Worker, redisClient)
		controller.workers = append(controller.workers, w)
	}

	return controller, nil
}

// Start starts the controller, scheduler and workers
func (c *Controller) Start() error {
	ctx := context.Background()

	// Start scheduler
	if err := c.scheduler.Start(ctx); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	// Start workers
	for i, w := range c.workers {
		if err := w.Start(ctx); err != nil {
			log.Printf("Failed to start worker %d: %v", i, err)
			continue
		}
	}

	// Start health check loop
	c.wg.Add(1)
	go c.healthCheckLoop(ctx)

	return nil
}

// Stop stops the controller, scheduler and workers
func (c *Controller) Stop() error {
	close(c.stopCh)
	c.wg.Wait()

	// Stop scheduler
	if err := c.scheduler.Stop(); err != nil {
		log.Printf("Error stopping scheduler: %v", err)
	}

	// Stop workers
	for i, w := range c.workers {
		if err := w.Stop(); err != nil {
			log.Printf("Error stopping worker %d: %v", i, err)
		}
	}

	// Close Redis client
	if err := c.redisClient.Close(); err != nil {
		log.Printf("Error closing Redis client: %v", err)
	}

	return nil
}

// CreateTask creates a new task
func (c *Controller) CreateTask(ctx context.Context, t *task.Task, schedule *task.Schedule) (string, error) {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}

	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()

	if t.MaxRetries == 0 {
		t.MaxRetries = c.config.Scheduler.MaxRetries
	}

	// Schedule the task
	if err := c.scheduler.ScheduleTask(ctx, t, schedule); err != nil {
		return "", fmt.Errorf("failed to schedule task: %w", err)
	}

	return t.ID, nil
}

// GetTask gets a task by ID
func (c *Controller) GetTask(ctx context.Context, taskID string) (*task.Task, error) {
	key := fmt.Sprintf("task:%s", taskID)
	result, err := c.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("task not found: %s", taskID)
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	task, err := task.TaskFromJSON([]byte(result))
	if err != nil {
		return nil, fmt.Errorf("failed to parse task: %w", err)
	}

	return task, nil
}

// GetTaskResult gets the latest result for a task
func (c *Controller) GetTaskResult(ctx context.Context, taskID string) (*Result, error) {
	key := fmt.Sprintf("result:%s", taskID)
	result, err := c.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("task result not found: %s", taskID)
		}
		return nil, fmt.Errorf("failed to get task result: %w", err)
	}

	var taskResult Result
	if err := json.Unmarshal([]byte(result), &taskResult); err != nil {
		return nil, fmt.Errorf("failed to parse task result: %w", err)
	}

	return &taskResult, nil
}

// GetTaskHistory gets the execution history for a task
func (c *Controller) GetTaskHistory(ctx context.Context, taskID string) ([]*Result, error) {
	key := fmt.Sprintf("history:%s", taskID)
	results, err := c.redisClient.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get task history: %w", err)
	}

	var history []*Result
	for _, result := range results {
		var taskResult Result
		if err := json.Unmarshal([]byte(result), &taskResult); err != nil {
			log.Printf("Failed to parse task result in history: %v", err)
			continue
		}
		history = append(history, &taskResult)
	}

	return history, nil
}

// CancelTask cancels a task
func (c *Controller) CancelTask(ctx context.Context, taskID string) error {
	return c.scheduler.CancelTask(ctx, taskID)
}

// ListTasks lists tasks with optional filters
func (c *Controller) ListTasks(ctx context.Context, status task.Status, limit, offset int) ([]*task.Task, error) {
	var pattern string
	if status == "" {
		pattern = "task:*"
	} else {
		// This is not efficient for large datasets, but works for demo
		// In production, you'd use a secondary index
		pattern = "task:*"
	}

	keys, err := c.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	// Apply pagination
	if offset >= len(keys) {
		return []*task.Task{}, nil
	}

	end := offset + limit
	if end > len(keys) {
		end = len(keys)
	}

	keys = keys[offset:end]

	var tasks []*task.Task
	for _, key := range keys {
		result, err := c.redisClient.Get(ctx, key).Result()
		if err != nil {
			log.Printf("Failed to get task %s: %v", key, err)
			continue
		}

		task, err := task.TaskFromJSON([]byte(result))
		if err != nil {
			log.Printf("Failed to parse task %s: %v", key, err)
			continue
		}

		// Filter by status if specified
		if status != "" && task.Status != status {
			continue
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// healthCheckLoop performs periodic health checks
func (c *Controller) healthCheckLoop(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.Scheduler.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.performHealthCheck(ctx)
		}
	}
}

// performHealthCheck performs a health check
func (c *Controller) performHealthCheck(ctx context.Context) {
	// Update controller heartbeat
	key := fmt.Sprintf("heartbeat:controller:%s", c.id)
	c.redisClient.Set(ctx, key, time.Now().Unix(), c.config.Scheduler.HeartbeatInterval*2)

	// Check for dead workers and reassign their tasks
	// This is a simplified version - in production you'd want more sophisticated logic
	workerKeys, err := c.redisClient.Keys(ctx, "heartbeat:worker:*").Result()
	if err != nil {
		log.Printf("Failed to get worker heartbeats: %v", err)
		return
	}

	// Get list of active workers
	activeWorkers := make(map[string]bool)
	for _, key := range workerKeys {
		// Extract worker ID from key
		workerID := key[17:] // Remove "heartbeat:worker:" prefix
		activeWorkers[workerID] = true
	}

	// Find tasks assigned to inactive workers
	taskKeys, err := c.redisClient.Keys(ctx, "task:*").Result()
	if err != nil {
		log.Printf("Failed to get tasks: %v", err)
		return
	}

	for _, key := range taskKeys {
		result, err := c.redisClient.Get(ctx, key).Result()
		if err != nil {
			log.Printf("Failed to get task %s: %v", key, err)
			continue
		}

		task, err := task.TaskFromJSON([]byte(result))
		if err != nil {
			log.Printf("Failed to parse task %s: %v", key, err)
			continue
		}

		// Check if task is running on an inactive worker
		if task.Status == "running" && task.WorkerID != "" {
			if !activeWorkers[task.WorkerID] {
				// Worker is dead, mark task as failed
				log.Printf("Worker %s appears to be dead, marking task %s as failed", task.WorkerID, task.ID)

				task.Status = "failed"
				task.UpdatedAt = time.Now()

				taskJSON, err := task.ToJSON()
				if err != nil {
					log.Printf("Failed to serialize task %s: %v", task.ID, err)
					continue
				}

				if err := c.redisClient.Set(ctx, key, taskJSON, 0).Err(); err != nil {
					log.Printf("Failed to update task %s: %v", task.ID, err)
					continue
				}

				// Create a failure result
				result := &Result{
					TaskID:     task.ID,
					Status:     "failed",
					Error:      "Worker died during execution",
					StartTime:  task.UpdatedAt,
					EndTime:    time.Now(),
					RetryCount: task.RetryCount,
					WorkerID:   task.WorkerID,
				}

				resultJSON, err := json.Marshal(result)
				if err != nil {
					log.Printf("Failed to serialize result for task %s: %v", task.ID, err)
					continue
				}

				resultKey := fmt.Sprintf("result:%s", task.ID)
				if err := c.redisClient.Set(ctx, resultKey, resultJSON, 24*time.Hour).Err(); err != nil {
					log.Printf("Failed to save result for task %s: %v", task.ID, err)
					continue
				}

				// Add to history
				historyKey := fmt.Sprintf("history:%s", task.ID)
				if err := c.redisClient.LPush(ctx, historyKey, resultJSON).Err(); err != nil {
					log.Printf("Failed to add to history for task %s: %v", task.ID, err)
					continue
				}

				// Schedule retry if needed
				if task.RetryCount < task.MaxRetries {
					task.RetryCount++
					task.Status = "retrying"
					task.NextRetryAt = time.Now().Add(c.config.Scheduler.RetryBackoff)
					task.WorkerID = ""
					task.UpdatedAt = time.Now()

					retryTaskJSON, err := task.ToJSON()
					if err != nil {
						log.Printf("Failed to serialize retry task %s: %v", task.ID, err)
						continue
					}

					if err := c.redisClient.Set(ctx, key, retryTaskJSON, 0).Err(); err != nil {
						log.Printf("Failed to update retry task %s: %v", task.ID, err)
						continue
					}

					// Create a schedule for retry
					schedule := &Schedule{
						Type:      "one-time",
						StartTime: task.NextRetryAt,
					}

					scheduleJSON, err := json.Marshal(schedule)
					if err != nil {
						log.Printf("Failed to serialize schedule for retry task %s: %v", task.ID, err)
						continue
					}

					scheduleKey := fmt.Sprintf("schedule:%s", task.ID)
					if err := c.redisClient.Set(ctx, scheduleKey, scheduleJSON, 0).Err(); err != nil {
						log.Printf("Failed to save schedule for retry task %s: %v", task.ID, err)
						continue
					}
				}
			}
		}
	}
}
