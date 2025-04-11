package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"GoDispatch/dispatcher/config"
	"GoDispatch/dispatcher/task"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// Worker handles task execution
type Worker struct {
	id          string
	redisClient *redis.Client
	config      *config.WorkerConfig
	stopCh      chan struct{}
	wg          sync.WaitGroup
	activeJobs  sync.WaitGroup
	mu          sync.Mutex
	taskTypes   []string
	metrics     *MetricsCollector
}

// NewWorker creates a new worker
func NewWorker(cfg *config.WorkerConfig, redisClient *redis.Client) *Worker {
	id := uuid.New().String()
	return &Worker{
		id:          id,
		redisClient: redisClient,
		config:      cfg,
		stopCh:      make(chan struct{}),
		taskTypes:   []string{"default", "*"}, // Default task type and wildcard
		metrics:     NewMetricsCollector(),
	}
}

// RegisterTaskType registers a task type that this worker can handle
func (w *Worker) RegisterTaskType(taskType string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, t := range w.taskTypes {
		if t == taskType {
			return // Already registered
		}
	}

	w.taskTypes = append(w.taskTypes, taskType)
}

// Start starts the worker
func (w *Worker) Start(ctx context.Context) error {
	w.wg.Add(w.config.Count)

	// Start worker pool
	for i := 0; i < w.config.Count; i++ {
		go w.processLoop(ctx, i)
	}

	// Start metrics reporting
	w.wg.Add(1)
	go w.reportMetrics(ctx)

	// Register this worker for all task types
	go func() {
		// Scan Redis for all existing task types
		taskTypes, err := w.scanForTaskTypes(ctx)
		if err != nil {
			log.Printf("Failed to scan for task types: %v", err)
		} else {
			for _, tt := range taskTypes {
				w.RegisterTaskType(tt)
			}
		}
	}()

	return nil
}

// Stop stops the worker
func (w *Worker) Stop() error {
	close(w.stopCh)

	// Wait for all workers to stop
	w.wg.Wait()

	// Wait for active jobs to complete
	w.activeJobs.Wait()

	return nil
}

// processLoop is the main worker loop
func (w *Worker) processLoop(ctx context.Context, workerIndex int) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	// Setup heartbeat ticker - every 10 seconds should be sufficient
	heartbeatTicker := time.NewTicker(10 * time.Second)
	defer heartbeatTicker.Stop()

	// Update heartbeat immediately on start
	w.updateHeartbeat(ctx)

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			if err := w.fetchAndProcessTask(ctx); err != nil {
				log.Printf("Worker %d error: %v", workerIndex, err)
			}
		case <-heartbeatTicker.C:
			w.updateHeartbeat(ctx)
		}
	}
}

// updateHeartbeat updates the worker's heartbeat in Redis
func (w *Worker) updateHeartbeat(ctx context.Context) {
	// Set heartbeat key with 30 second expiration (3x the heartbeat interval)
	key := fmt.Sprintf("heartbeat:worker:%s", w.id)
	ttl := 30 * time.Second

	if err := w.redisClient.Set(ctx, key, time.Now().Unix(), ttl).Err(); err != nil {
		log.Printf("Failed to update worker heartbeat: %v", err)
	} else {
		log.Printf("Updated worker heartbeat: %s", w.id)
	}

	// Also update worker information in Redis
	workerKey := fmt.Sprintf("worker:%s", w.id)

	// Check for active tasks
	w.mu.Lock()
	activeTasks := w.metrics.GetActiveTaskCount()
	w.mu.Unlock()

	// Get current task if any
	var currentTask string
	currentTaskResult := w.redisClient.HGet(ctx, workerKey, "current_task")
	if currentTaskResult.Err() == nil {
		currentTask = currentTaskResult.Val()
	}

	status := "idle"
	if activeTasks > 0 || currentTask != "" {
		status = "active"
	}

	info := map[string]interface{}{
		"last_heartbeat": time.Now().UTC().Unix(),
		"status":         status,
	}

	if currentTask != "" {
		info["current_task"] = currentTask
	}

	if err := w.redisClient.HMSet(ctx, workerKey, info).Err(); err != nil {
		log.Printf("Failed to update worker info: %v", err)
	} else {
		log.Printf("Updated worker status to %s", status)
	}
}

// scanForTaskTypes scans Redis for all existing task types
func (w *Worker) scanForTaskTypes(ctx context.Context) ([]string, error) {
	var taskTypes []string
	var cursor uint64
	var err error
	var keys []string

	// Scan for all task queues
	for {
		keys, cursor, err = w.redisClient.Scan(ctx, cursor, "queue:tasks:*", 100).Result()
		if err != nil {
			return nil, err
		}

		// Extract task types from queue keys
		for _, key := range keys {
			taskType := key[12:] // Remove "queue:tasks:" prefix
			if taskType != "" {
				taskTypes = append(taskTypes, taskType)
			}
		}

		if cursor == 0 {
			break
		}
	}

	// Also scan all task keys to find task types
	for {
		keys, cursor, err = w.redisClient.Scan(ctx, cursor, "task:*", 100).Result()
		if err != nil {
			return nil, err
		}

		for _, key := range keys {
			taskData, err := w.redisClient.Get(ctx, key).Result()
			if err != nil {
				continue
			}

			var taskMap map[string]interface{}
			if err := json.Unmarshal([]byte(taskData), &taskMap); err != nil {
				continue
			}

			if taskType, ok := taskMap["type"].(string); ok && taskType != "" {
				found := false
				for _, tt := range taskTypes {
					if tt == taskType {
						found = true
						break
					}
				}
				if !found {
					taskTypes = append(taskTypes, taskType)
				}
			}
		}

		if cursor == 0 {
			break
		}
	}

	return taskTypes, nil
}

// fetchAndProcessTask fetches a task from the queue and processes it
func (w *Worker) fetchAndProcessTask(ctx context.Context) error {
	w.mu.Lock()
	taskTypes := make([]string, len(w.taskTypes))
	copy(taskTypes, w.taskTypes)
	w.mu.Unlock()

	// Try to fetch a task from any of the registered queues
	for _, taskType := range taskTypes {
		queueKey := fmt.Sprintf("queue:tasks:%s", taskType)

		// Get the highest priority task
		result := w.redisClient.ZRevRange(ctx, queueKey, 0, 0)
		if result.Err() != nil {
			if result.Err() != redis.Nil {
				log.Printf("Error fetching tasks from %s: %v", queueKey, result.Err())
			}
			continue
		}

		tasks, err := result.Result()
		if err != nil {
			log.Printf("Failed to get result from %s: %v", queueKey, err)
			continue
		}

		if len(tasks) == 0 {
			// Also try looking for pending tasks directly
			if taskType == "*" {
				pendingTasks, err := w.findPendingTasks(ctx)
				if err != nil {
					log.Printf("Error finding pending tasks: %v", err)
					continue
				}

				for _, pendingTask := range pendingTasks {
					// Process each pending task
					w.activeJobs.Add(1)
					go w.processTask(ctx, pendingTask)
					return nil
				}
			}
			continue // No tasks in this queue
		}

		taskJSON := tasks[0]

		// Try to claim the task by removing it from the queue
		removed := w.redisClient.ZRem(ctx, queueKey, taskJSON)
		if removed.Err() != nil {
			log.Printf("Failed to remove task from queue %s: %v", queueKey, removed.Err())
			continue
		}

		if removed.Val() == 0 {
			log.Printf("Task was already claimed by another worker from queue %s", queueKey)
			continue // Task was already claimed by another worker
		}

		// Parse the task
		task, err := task.TaskFromJSON([]byte(taskJSON))
		if err != nil {
			log.Printf("Failed to parse task from queue %s: %v", queueKey, err)
			continue
		}

		log.Printf("Worker %s claimed task %s of type %s from queue %s", w.id, task.ID, task.Type, queueKey)

		// Process the task in a separate goroutine
		w.activeJobs.Add(1)
		go w.processTask(ctx, task)

		return nil // Successfully fetched and started processing a task
	}

	// If we reach here, we should check for pending tasks that might not be in the queue
	pendingTasks, err := w.findPendingTasks(ctx)
	if err != nil {
		log.Printf("Error finding pending tasks: %v", err)
		return nil
	}

	if len(pendingTasks) > 0 {
		// Process the first pending task
		w.activeJobs.Add(1)
		go w.processTask(ctx, pendingTasks[0])
		return nil
	}

	return nil // No tasks to process
}

// findPendingTasks finds tasks with pending status in Redis
func (w *Worker) findPendingTasks(ctx context.Context) ([]*task.Task, error) {
	var pendingTasks []*task.Task
	var cursor uint64
	var err error
	var keys []string

	for {
		keys, cursor, err = w.redisClient.Scan(ctx, cursor, "task:*", 100).Result()
		if err != nil {
			return nil, err
		}

		for _, key := range keys {
			taskData, err := w.redisClient.Get(ctx, key).Result()
			if err != nil {
				continue
			}

			t, err := task.TaskFromJSON([]byte(taskData))
			if err != nil {
				continue
			}

			if t.Status == task.StatusPending {
				// Also check if it's in any queue
				inQueue, err := w.isTaskInQueue(ctx, t)
				if err != nil {
					log.Printf("Error checking if task %s is in queue: %v", t.ID, err)
					continue
				}

				if !inQueue {
					// If not in queue, add it to our list to process
					pendingTasks = append(pendingTasks, t)
					log.Printf("Found pending task %s not in any queue", t.ID)
				}
			}
		}

		if cursor == 0 {
			break
		}
	}

	return pendingTasks, nil
}

// isTaskInQueue checks if a task is in any queue
func (w *Worker) isTaskInQueue(ctx context.Context, t *task.Task) (bool, error) {
	queueKey := fmt.Sprintf("queue:tasks:%s", t.Type)

	// Convert task to JSON for comparison
	taskJSON, err := t.ToJSON()
	if err != nil {
		return false, err
	}

	// Check if task is in its type-specific queue
	count, err := w.redisClient.ZScore(ctx, queueKey, string(taskJSON)).Result()
	if err != nil && err != redis.Nil {
		return false, err
	}

	return count > 0, nil
}

// processTask processes a single task
func (w *Worker) processTask(ctx context.Context, t *task.Task) {
	defer w.activeJobs.Done()

	log.Printf("Worker %s starting to process task %s of type %s", w.id, t.ID, t.Type)

	// First, get the latest task status from Redis to make sure it hasn't been canceled
	key := fmt.Sprintf("task:%s", t.ID)
	latestTaskJSON, err := w.redisClient.Get(ctx, key).Result()
	if err != nil {
		log.Printf("Error fetching latest task status for %s: %v", t.ID, err)
		return
	}

	latestTask, err := task.TaskFromJSON([]byte(latestTaskJSON))
	if err != nil {
		log.Printf("Error parsing task %s: %v", t.ID, err)
		return
	}

	// Use the latest task data
	t = latestTask

	// Check if the task status is appropriate for processing
	if t.Status != task.StatusPending && t.Status != task.StatusScheduled && t.Status != task.StatusRetrying {
		log.Printf("Task %s has status %s which is not processable, skipping", t.ID, t.Status)
		return
	}

	// Specifically check for canceled status
	if t.Status == task.StatusCancelled || t.Status == task.StatusCompleted {
		log.Printf("Task %s is already %s, not processing", t.ID, t.Status)
		return
	}

	// Create result with initial status
	result := &task.Result{
		TaskID:     t.ID,
		Status:     task.StatusRunning,
		StartTime:  time.Now().UTC(), // Use UTC for consistent timestamps
		WorkerID:   w.id,
		RetryCount: t.RetryCount,
	}

	// Ensure we have a valid task ID to prevent workers from processing the same task
	if t.ID == "" {
		log.Printf("Task has empty ID, cannot process")
		return
	}

	// Try to claim the task by setting its status and worker ID
	claimed, err := w.claimTask(ctx, t.ID, w.id)
	if err != nil {
		log.Printf("Error claiming task %s: %v", t.ID, err)
		return
	}

	if !claimed {
		log.Printf("Task %s is already being processed by another worker", t.ID)
		return
	}

	// Update task status to running
	t.Status = task.StatusRunning
	t.WorkerID = w.id
	t.UpdatedAt = time.Now().UTC() // Use UTC

	// Save task status
	var taskJSONBytes []byte
	taskJSONBytes, err = t.ToJSON()
	if err == nil {
		key := fmt.Sprintf("task:%s", t.ID)
		if err := w.redisClient.Set(ctx, key, taskJSONBytes, 0).Err(); err != nil {
			log.Printf("Failed to update task %s status to running: %v", t.ID, err)
		} else {
			log.Printf("Updated task %s status to running", t.ID)
		}
	}

	// Remove task from pending set if it exists
	pendingSetKey := "set:pending_tasks"
	if err := w.redisClient.SRem(ctx, pendingSetKey, t.ID).Err(); err != nil && err != redis.Nil {
		log.Printf("Failed to remove task %s from pending set: %v", t.ID, err)
	}

	// Update worker info in Redis to indicate current task
	workerKey := fmt.Sprintf("worker:%s", w.id)
	w.redisClient.HSet(ctx, workerKey, "current_task", t.ID)
	w.redisClient.HSet(ctx, workerKey, "status", "active") // Make sure to set status to active

	// Create a context with timeout if deadline is set
	execCtx := ctx
	var cancel context.CancelFunc

	if t.Deadline != nil && !t.Deadline.IsZero() {
		timeout := time.Until(*t.Deadline)
		if timeout <= 0 {
			// Task has already expired
			result.Status = task.StatusTimeout
			result.Error = "Task deadline exceeded before execution"
			result.EndTime = time.Now().UTC() // Use UTC
			w.saveResult(ctx, result)
			return
		}

		execCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	} else {
		// If no deadline is set, use a default timeout of 10 minutes
		execCtx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
	}

	// Start metrics collection
	startMetrics := w.metrics.StartTask(t.ID)

	// Execute task
	err = w.executeTask(execCtx, t, result)

	// End metrics collection
	metrics := w.metrics.EndTask(t.ID, startMetrics)
	result.Metrics = metrics

	// Set end time and update status based on execution result
	result.EndTime = time.Now().UTC() // Use UTC

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			result.Status = task.StatusTimeout
			result.Error = "Task execution timed out"
			log.Printf("Task %s execution timed out", t.ID)
		} else {
			result.Status = task.StatusFailed
			result.Error = err.Error()
			log.Printf("Task %s execution failed: %v", t.ID, err)

			// Check if the task should be retried
			if t.RetryCount < t.MaxRetries {
				t.RetryCount++
				t.Status = task.StatusRetrying
				t.NextRetryAt = time.Now().UTC().Add(time.Second * time.Duration(t.RetryCount) * 10) // Simple backoff strategy
				t.UpdatedAt = time.Now().UTC()                                                       // Use UTC
				t.WorkerID = ""                                                                      // Clear worker ID so another worker can pick it up

				log.Printf("Task %s will be retried (%d/%d) at %s",
					t.ID, t.RetryCount, t.MaxRetries, t.NextRetryAt.Format(time.RFC3339))

				// Re-queue the task with updated retry info
				w.requeueTask(ctx, t)

				// Also update the task result for history
				result.Status = task.StatusRetrying
				result.Error = fmt.Sprintf("%s (will retry %d/%d)", err.Error(), t.RetryCount, t.MaxRetries)
			}
		}
	} else {
		result.Status = task.StatusCompleted
		log.Printf("Task %s completed successfully", t.ID)
	}

	// Update task status
	t.Status = result.Status
	t.UpdatedAt = time.Now().UTC() // Use UTC

	// Save the final task state
	taskJSONBytes, err = t.ToJSON()
	if err == nil {
		key := fmt.Sprintf("task:%s", t.ID)
		if err := w.redisClient.Set(ctx, key, taskJSONBytes, 0).Err(); err != nil {
			log.Printf("Failed to save final task %s state: %v", t.ID, err)
		} else {
			log.Printf("Saved final state for task %s", t.ID)
		}
	}

	// Clear current task from worker info
	w.redisClient.HSet(ctx, workerKey, "current_task", "")
	w.redisClient.HSet(ctx, workerKey, "status", "idle")

	// Save the result
	w.saveResult(ctx, result)
}

// claimTask attempts to claim a task for a worker
// Returns true if claim was successful, false if task is already claimed
func (w *Worker) claimTask(ctx context.Context, taskID string, workerID string) (bool, error) {
	key := fmt.Sprintf("task:%s", taskID)

	// Get current task state
	taskJSON, err := w.redisClient.Get(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to get task: %w", err)
	}

	t, err := task.TaskFromJSON([]byte(taskJSON))
	if err != nil {
		return false, fmt.Errorf("failed to parse task: %w", err)
	}

	// Check if task is already claimed
	if t.Status == task.StatusRunning && t.WorkerID != "" && t.WorkerID != workerID {
		return false, nil // Task already claimed by another worker
	}

	// Use Redis transaction to ensure atomic update
	txf := func(tx *redis.Tx) error {
		// Get current task data again within transaction
		taskJSON, err := tx.Get(ctx, key).Result()
		if err != nil {
			return err
		}

		t, err := task.TaskFromJSON([]byte(taskJSON))
		if err != nil {
			return err
		}

		// Check again if task is already claimed
		if t.Status == task.StatusRunning && t.WorkerID != "" && t.WorkerID != workerID {
			return fmt.Errorf("task already claimed")
		}

		// Update task status and worker ID
		t.Status = task.StatusRunning
		t.WorkerID = workerID
		t.UpdatedAt = time.Now().UTC()

		// Save updated task
		updatedJSON, err := t.ToJSON()
		if err != nil {
			return err
		}

		// Update the task
		return tx.Set(ctx, key, updatedJSON, 0).Err()
	}

	// Execute transaction
	err = w.redisClient.Watch(ctx, txf, key)
	if err != nil {
		if err.Error() == "task already claimed" {
			return false, nil
		}
		return false, fmt.Errorf("transaction failed: %w", err)
	}

	return true, nil
}

// executeTask executes a task based on its type
func (w *Worker) executeTask(ctx context.Context, t *task.Task, result *task.Result) error {
	log.Printf("Executing task %s of type %s", t.ID, t.Type)

	// Create a context with cancellation for task status checking
	execCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start a goroutine to periodically check if the task has been canceled or completed
	statusCheckDone := make(chan struct{})
	go func() {
		defer close(statusCheckDone)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Check current task status in Redis
				key := fmt.Sprintf("task:%s", t.ID)
				taskJSON, err := w.redisClient.Get(ctx, key).Result()
				if err != nil {
					log.Printf("Error checking task status for %s: %v", t.ID, err)
					continue
				}

				currentTask, err := task.TaskFromJSON([]byte(taskJSON))
				if err != nil {
					log.Printf("Error parsing task during status check for %s: %v", t.ID, err)
					continue
				}

				// If task has been canceled or completed externally, stop execution
				if currentTask.Status == task.StatusCancelled {
					log.Printf("Task %s has been canceled, stopping execution", t.ID)
					result.Status = task.StatusCancelled
					result.Error = "Task was canceled while executing"
					cancel() // Cancel the execution context
					return
				}
			case <-execCtx.Done():
				return
			}
		}
	}()

	// This is where you would implement task execution based on task type
	// For now, we'll simulate task execution with a delay and log the task

	// Log task details
	log.Printf("Task details: ID=%s, Type=%s, Name=%s, Priority=%d",
		t.ID, t.Type, t.Name, t.Priority)

	// If task has payload, log a preview
	if len(t.Payload) > 0 {
		preview := ""
		if len(t.Payload) > 100 {
			preview = string(t.Payload[:100]) + "..."
		} else {
			preview = string(t.Payload)
		}
		log.Printf("Task %s payload preview: %s", t.ID, preview)
	}

	// Simulate a random execution time between 1 and 3 seconds
	execTime := time.Duration(1+rand.Intn(2)) * time.Second
	log.Printf("Task %s simulated execution time: %v", t.ID, execTime)

	select {
	case <-time.After(execTime):
		log.Printf("Task %s execution completed", t.ID)
		// Wait for status checking goroutine to finish
		<-statusCheckDone
		return nil
	case <-execCtx.Done():
		// Check if cancellation was due to task being canceled
		if result.Status == task.StatusCancelled {
			log.Printf("Task %s execution abandoned due to cancellation", t.ID)
			// Wait for status checking goroutine to finish
			<-statusCheckDone
			return fmt.Errorf("task was canceled")
		}
		log.Printf("Task %s execution interrupted: %v", t.ID, ctx.Err())
		// Wait for status checking goroutine to finish
		<-statusCheckDone
		return ctx.Err()
	}
}

// requeueTask handles requeuing a task for retry
func (w *Worker) requeueTask(ctx context.Context, t *task.Task) {
	// First, save the updated task state with retrying status
	taskKey := fmt.Sprintf("task:%s", t.ID)
	taskJSON, err := t.ToJSON()
	if err != nil {
		log.Printf("Failed to serialize task for requeue: %v", err)
		return
	}

	if err := w.redisClient.Set(ctx, taskKey, taskJSON, 0).Err(); err != nil {
		log.Printf("Failed to save updated task state for retry: %v", err)
		return
	}

	// Create a schedule for retry
	schedule := &task.Schedule{
		Type:      "one-time",
		StartTime: t.NextRetryAt,
	}

	// Save the schedule
	scheduleJSON, err := json.Marshal(schedule)
	if err != nil {
		log.Printf("Failed to serialize schedule for retry: %v", err)
		return
	}

	scheduleKey := fmt.Sprintf("schedule:%s", t.ID)
	if err := w.redisClient.Set(ctx, scheduleKey, scheduleJSON, 0).Err(); err != nil {
		log.Printf("Failed to save schedule for retry: %v", err)
		return
	}

	log.Printf("Created retry schedule for task %s at %s",
		t.ID, t.NextRetryAt.Format(time.RFC3339))

	// Remove from active queues since it's scheduled for later
	queueKey := fmt.Sprintf("queue:tasks:%s", t.Type)
	if err := w.redisClient.ZRem(ctx, queueKey, taskJSON).Err(); err != nil {
		log.Printf("Failed to remove from active queue: %v", err)
	}

	allPendingKey := "queue:tasks:all_pending"
	if err := w.redisClient.ZRem(ctx, allPendingKey, taskJSON).Err(); err != nil {
		log.Printf("Failed to remove from all_pending queue: %v", err)
	}

	// Remove from pending set as it's now in retry state
	pendingSetKey := "set:pending_tasks"
	if err := w.redisClient.SRem(ctx, pendingSetKey, t.ID).Err(); err != nil {
		log.Printf("Failed to remove from pending set: %v", err)
	}
}

// saveResult saves a task result to Redis
func (w *Worker) saveResult(ctx context.Context, result *task.Result) {
	// Ensure timestamps are in UTC
	if !result.StartTime.IsZero() {
		result.StartTime = result.StartTime.UTC()
	}

	if !result.EndTime.IsZero() {
		result.EndTime = result.EndTime.UTC()
	}

	// Make sure we have valid times
	if result.StartTime.IsZero() {
		result.StartTime = time.Now().UTC()
	}

	if result.EndTime.IsZero() {
		result.EndTime = time.Now().UTC()
	}

	// Format task metrics if available
	if result.Metrics != nil {
		// Ensure we have valid metrics values
		if result.Metrics.ProcessingTime <= 0 {
			result.Metrics.ProcessingTime = result.EndTime.Sub(result.StartTime)
		}
	}

	// Serialize result to JSON
	resultJSON, err := json.Marshal(result)
	if err != nil {
		log.Printf("Failed to serialize result: %v", err)
		return
	}

	log.Printf("Saving result for task %s with status %s (Start: %s, End: %s)",
		result.TaskID, result.Status, result.StartTime.Format(time.RFC3339), result.EndTime.Format(time.RFC3339))

	// Save to results set
	resultKey := fmt.Sprintf("result:%s", result.TaskID)
	if err := w.redisClient.Set(ctx, resultKey, resultJSON, 24*time.Hour).Err(); err != nil {
		log.Printf("Failed to save result: %v", err)
	} else {
		log.Printf("Saved result for task %s to Redis", result.TaskID)
	}

	// Add to history set
	historyKey := fmt.Sprintf("history:%s", result.TaskID)
	if err := w.redisClient.LPush(ctx, historyKey, resultJSON).Err(); err != nil {
		log.Printf("Failed to add to history: %v", err)
	} else {
		log.Printf("Added result to history for task %s", result.TaskID)
	}

	// Trim history to keep it from growing too large
	w.redisClient.LTrim(ctx, historyKey, 0, 19) // Keep last 20 entries
}

// reportMetrics periodically reports worker metrics
func (w *Worker) reportMetrics(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			stats := w.metrics.GetStats()
			log.Printf("Worker metrics: Active=%d, Completed=%d, Failed=%d, AvgProcessingTime=%v",
				stats.ActiveTasks, stats.CompletedTasks, stats.FailedTasks, stats.AvgProcessingTime)

			// Save metrics to Redis
			metricsKey := fmt.Sprintf("metrics:worker:%s", w.id)
			metricsJSON, err := json.Marshal(stats)
			if err == nil {
				w.redisClient.Set(ctx, metricsKey, metricsJSON, time.Hour)
			}
		}
	}
}

// GetMetrics returns the worker's metrics
func (w *Worker) GetMetrics() *MetricsCollector {
	return w.metrics
}

// GetID returns the worker's ID
func (w *Worker) GetID() string {
	return w.id
}
