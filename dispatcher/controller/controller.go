// controller serves as the coordinator for the scheduler
package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"GoDispatch/dispatcher/config"
	"GoDispatch/dispatcher/task"
	"GoDispatch/dispatcher/worker"

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

// TaskStats represents statistics about tasks
type TaskStats struct {
	TotalTasks     int `json:"total_tasks"`
	ActiveTasks    int `json:"active_tasks"`
	CompletedTasks int `json:"completed_tasks"`
	FailedTasks    int `json:"failed_tasks"`
}

// WorkerStats represents statistics about workers
type WorkerStats struct {
	ActiveWorkers     int           `json:"active_workers"`
	IdleWorkers       int           `json:"idle_workers"`
	AvgProcessingTime time.Duration `json:"avg_processing_time"`
}

// WorkerInfo holds information about a worker
type WorkerInfo struct {
	ID            string    `json:"id"`
	Status        string    `json:"status"`
	CurrentTask   string    `json:"current_task,omitempty"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
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
	startTime   time.Time
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
		startTime:   time.Now(),
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
func (c *Controller) ListTasks(ctx context.Context, status task.Status, limit, offset int, sortBy string, sortOrder string) ([]*task.Task, error) {
	// Get all task keys (improvement: use SCAN for large datasets)
	pattern := "task:*"
	keys, err := c.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	// Use Redis MGET to get multiple tasks at once
	results, err := c.redisClient.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks: %w", err)
	}

	// Parse all tasks first
	var tasks []*task.Task
	for _, result := range results {
		if result == nil {
			continue
		}

		taskStr, ok := result.(string)
		if !ok {
			continue
		}

		task, err := task.TaskFromJSON([]byte(taskStr))
		if err != nil {
			log.Printf("Failed to parse task: %v", err)
			continue
		}

		// Filter by status if specified
		if status != "" && task.Status != status {
			continue
		}

		tasks = append(tasks, task)
	}

	// Sort the tasks based on sortBy and sortOrder parameters
	sortTasks(tasks, sortBy, sortOrder)

	// Apply pagination
	if offset >= len(tasks) {
		return []*task.Task{}, nil
	}

	end := offset + limit
	if end > len(tasks) {
		end = len(tasks)
	}

	// Return the paginated subset
	return tasks[offset:end], nil
}

// sortTasks sorts tasks based on specified field and order
func sortTasks(tasks []*task.Task, sortBy string, sortOrder string) {
	sort.Slice(tasks, func(i, j int) bool {
		// Handle different sort fields
		switch sortBy {
		case "created_at":
			if sortOrder == "asc" {
				return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
			}
			return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
		case "updated_at":
			if sortOrder == "asc" {
				return tasks[i].UpdatedAt.Before(tasks[j].UpdatedAt)
			}
			return tasks[i].UpdatedAt.After(tasks[j].UpdatedAt)
		case "name":
			if sortOrder == "asc" {
				return tasks[i].Name < tasks[j].Name
			}
			return tasks[i].Name > tasks[j].Name
		case "status":
			if sortOrder == "asc" {
				return string(tasks[i].Status) < string(tasks[j].Status)
			}
			return string(tasks[i].Status) > string(tasks[j].Status)
		case "priority":
			if sortOrder == "asc" {
				return tasks[i].Priority < tasks[j].Priority
			}
			return tasks[i].Priority > tasks[j].Priority
		default:
			// Default to sort by creation date, newest first
			return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
		}
	})
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
				// Worker is dead, mark task as failed or retry
				log.Printf("Worker %s appears to be dead, handling task %s", task.WorkerID, task.ID)

				// Check if we should retry
				if task.RetryCount < task.MaxRetries {
					task.RetryCount++
					task.Status = "retrying"
					task.NextRetryAt = time.Now().Add(c.config.Scheduler.RetryBackoff)
					task.WorkerID = ""
					task.UpdatedAt = time.Now()

					log.Printf("Task %s will be retried (%d/%d) after dead worker",
						task.ID, task.RetryCount, task.MaxRetries)

					taskJSON, err := task.ToJSON()
					if err != nil {
						log.Printf("Failed to serialize retry task %s: %v", task.ID, err)
						continue
					}

					if err := c.redisClient.Set(ctx, key, taskJSON, 0).Err(); err != nil {
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

					// Create a result indicating retry
					result := &Result{
						TaskID: task.ID,
						Status: "retrying",
						Error: fmt.Sprintf("Worker %s died during execution (will retry %d/%d)",
							task.WorkerID, task.RetryCount, task.MaxRetries),
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

					// Add to history
					historyKey := fmt.Sprintf("history:%s", task.ID)
					if err := c.redisClient.LPush(ctx, historyKey, resultJSON).Err(); err != nil {
						log.Printf("Failed to add to history for task %s: %v", task.ID, err)
						continue
					}
				} else {
					// No more retries, mark as failed
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
						Error:      "Worker died during execution and max retries exceeded",
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
				}
			}
		}

		// Also check for tasks that have been in 'retrying' state for too long (might be stuck)
		if task.Status == "retrying" {
			// Check if NextRetryAt is way in the past (more than 10 minutes)
			if !task.NextRetryAt.IsZero() && time.Since(task.NextRetryAt) > 10*time.Minute {
				log.Printf("Task %s has been stuck in retrying state, re-scheduling", task.ID)

				// Re-schedule the task
				schedule := &Schedule{
					Type:      "one-time",
					StartTime: time.Now(),
				}

				scheduleJSON, err := json.Marshal(schedule)
				if err != nil {
					log.Printf("Failed to serialize schedule for stuck task %s: %v", task.ID, err)
					continue
				}

				scheduleKey := fmt.Sprintf("schedule:%s", task.ID)
				if err := c.redisClient.Set(ctx, scheduleKey, scheduleJSON, 0).Err(); err != nil {
					log.Printf("Failed to save schedule for stuck task %s: %v", task.ID, err)
					continue
				}

				// Add to history
				result := &Result{
					TaskID:     task.ID,
					Status:     "retrying",
					Error:      "Task was stuck in retrying state, re-scheduled",
					StartTime:  time.Now(),
					EndTime:    time.Now(),
					RetryCount: task.RetryCount,
				}

				resultJSON, err := json.Marshal(result)
				if err != nil {
					log.Printf("Failed to serialize result for stuck task %s: %v", task.ID, err)
					continue
				}

				historyKey := fmt.Sprintf("history:%s", task.ID)
				if err := c.redisClient.LPush(ctx, historyKey, resultJSON).Err(); err != nil {
					log.Printf("Failed to add to history for stuck task %s: %v", task.ID, err)
					continue
				}
			}
		}
	}
}

// GetTasksStats returns statistics about tasks
func (c *Controller) GetTasksStats(ctx context.Context) (*TaskStats, error) {
	// Debug: list all keys in Redis with detailed logging
	allKeys, err := c.redisClient.Keys(ctx, "*").Result()
	if err != nil {
		log.Printf("Error getting all Redis keys: %v", err)
	} else {
		log.Printf("All Redis keys (%d): %v", len(allKeys), allKeys)

		// Group keys by prefix for easier debugging
		keysByPrefix := make(map[string][]string)
		for _, key := range allKeys {
			parts := strings.Split(key, ":")
			prefix := parts[0]
			if len(keysByPrefix[prefix]) < 10 { // Limit to 10 examples per prefix
				keysByPrefix[prefix] = append(keysByPrefix[prefix], key)
			}
		}

		// Log keys grouped by prefix
		for prefix, keys := range keysByPrefix {
			log.Printf("Keys with prefix '%s' (%d): %v", prefix, len(keys), keys)
		}
	}

	// Get task counts by directly using the task keys
	pattern := "task:*"
	keys, err := c.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		log.Printf("Error getting task keys: %v", err)
		return nil, err
	}

	log.Printf("Found %d task keys with pattern '%s': %v", len(keys), pattern, keys)

	// Initialize stats
	stats := &TaskStats{
		TotalTasks:     len(keys),
		ActiveTasks:    0,
		CompletedTasks: 0,
		FailedTasks:    0,
	}

	// If there are no tasks, return early
	if stats.TotalTasks == 0 {
		return stats, nil
	}

	// Use MGET to efficiently get all tasks at once
	results, err := c.redisClient.MGet(ctx, keys...).Result()
	if err != nil {
		log.Printf("Error getting task values: %v", err)
		return nil, err
	}

	// Count tasks by status
	for i, result := range results {
		if result == nil {
			log.Printf("Task value is nil for key: %s", keys[i])
			continue
		}

		taskStr, ok := result.(string)
		if !ok {
			log.Printf("Task value is not a string for key: %s", keys[i])
			continue
		}

		// Try to parse the task
		task, err := task.TaskFromJSON([]byte(taskStr))
		if err != nil {
			log.Printf("Failed to parse task for key %s: %v", keys[i], err)
			log.Printf("Task JSON content: %s", taskStr)
			continue
		}

		log.Printf("Task found - ID: %s, Type: %s, Name: %s, Status: %s",
			task.ID, task.Type, task.Name, task.Status)

		// Count by status
		status := task.Status
		if status == "pending" || status == "scheduled" || status == "running" || status == "retrying" {
			stats.ActiveTasks++
		} else if status == "completed" {
			stats.CompletedTasks++
		} else if status == "failed" || status == "cancelled" || status == "timeout" {
			stats.FailedTasks++
		}
	}

	return stats, nil
}

// GetWorkersStats returns statistics about workers
func (c *Controller) GetWorkersStats(ctx context.Context) (*WorkerStats, error) {
	// Collect stats from all workers
	stats := &WorkerStats{
		ActiveWorkers:     0,
		IdleWorkers:       0,
		AvgProcessingTime: 0,
	}

	totalProcessingTime := time.Duration(0)
	processedTasks := 0

	// Count active and idle workers
	for _, w := range c.workers {
		// Check if worker has active jobs
		if w.GetMetrics().GetStats().ActiveTasks > 0 {
			stats.ActiveWorkers++
		} else {
			stats.IdleWorkers++
		}

		// Get worker metrics
		workerMetrics := w.GetMetrics().GetStats()
		totalProcessingTime += workerMetrics.AvgProcessingTime * time.Duration(workerMetrics.CompletedTasks)
		processedTasks += workerMetrics.CompletedTasks
	}

	// Calculate average processing time
	if processedTasks > 0 {
		stats.AvgProcessingTime = totalProcessingTime / time.Duration(processedTasks)
	}

	return stats, nil
}

// GetStartTime returns the time when the controller was started
func (c *Controller) GetStartTime() time.Time {
	return c.startTime
}

// GetWorkers returns information about all workers
func (c *Controller) GetWorkers(ctx context.Context) ([]WorkerInfo, error) {
	var workers []WorkerInfo

	// Get all worker heartbeat keys
	workerKeys, err := c.redisClient.Keys(ctx, "heartbeat:worker:*").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get worker heartbeats: %w", err)
	}

	log.Printf("Found %d worker heartbeats", len(workerKeys))

	for _, key := range workerKeys {
		// Extract worker ID from key
		workerID := key[17:] // Remove "heartbeat:worker:" prefix
		log.Printf("Processing worker with ID: %s", workerID)

		// Get worker data
		workerKey := fmt.Sprintf("worker:%s", workerID)
		workerData, err := c.redisClient.HGetAll(ctx, workerKey).Result()
		if err != nil {
			log.Printf("Failed to get data for worker %s: %v", workerID, err)
			continue
		}

		// Default values
		status := "idle"
		var currentTask string
		lastHeartbeat := time.Now()

		// Get status
		if s, ok := workerData["status"]; ok {
			status = s
		}

		// Get current task
		if task, ok := workerData["current_task"]; ok && task != "" {
			currentTask = task
		}

		// Get last heartbeat
		if hb, ok := workerData["last_heartbeat"]; ok {
			timestamp, err := strconv.ParseInt(hb, 10, 64)
			if err == nil {
				lastHeartbeat = time.Unix(timestamp, 0)
			}
		}

		// Create WorkerInfo
		workerInfo := WorkerInfo{
			ID:            workerID,
			Status:        status,
			CurrentTask:   currentTask,
			LastHeartbeat: lastHeartbeat,
		}

		workers = append(workers, workerInfo)
	}

	// If no workers were found, add the in-memory workers as fallback
	if len(workers) == 0 {
		for _, w := range c.workers {
			metrics := w.GetMetrics().GetStats()

			status := "idle"
			if metrics.ActiveTasks > 0 {
				status = "active"
			}

			workerInfo := WorkerInfo{
				ID:            w.GetID(),
				Status:        status,
				LastHeartbeat: time.Now(),
			}

			workers = append(workers, workerInfo)
		}
	}

	return workers, nil
}
