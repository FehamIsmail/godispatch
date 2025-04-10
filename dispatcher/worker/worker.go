package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
		taskTypes:   []string{"default"}, // Default task type
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

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			if err := w.fetchAndProcessTask(ctx); err != nil {
				log.Printf("Worker %d error: %v", workerIndex, err)
			}
		}
	}
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
			return fmt.Errorf("failed to fetch task: %w", result.Err())
		}

		tasks, err := result.Result()
		if err != nil {
			return fmt.Errorf("failed to get result: %w", err)
		}

		if len(tasks) == 0 {
			continue // No tasks in this queue
		}

		taskJSON := tasks[0]

		// Try to claim the task by removing it from the queue
		removed := w.redisClient.ZRem(ctx, queueKey, taskJSON)
		if removed.Err() != nil {
			return fmt.Errorf("failed to remove task from queue: %w", removed.Err())
		}

		if removed.Val() == 0 {
			continue // Task was already claimed by another worker
		}

		// Parse the task
		task, err := task.TaskFromJSON([]byte(taskJSON))
		if err != nil {
			return fmt.Errorf("failed to parse task: %w", err)
		}

		// Process the task in a separate goroutine
		w.activeJobs.Add(1)
		go w.processTask(ctx, task)

		return nil // Successfully fetched and started processing a task
	}

	return nil // No tasks to process
}

// processTask processes a single task
func (w *Worker) processTask(ctx context.Context, t *task.Task) {
	defer w.activeJobs.Done()

	// Create result with initial status
	result := &task.Result{
		TaskID:     t.ID,
		Status:     task.StatusRunning,
		StartTime:  time.Now(),
		WorkerID:   w.id,
		RetryCount: t.RetryCount,
	}

	// Update task status to running
	t.Status = task.StatusRunning
	t.WorkerID = w.id
	t.UpdatedAt = time.Now()

	// Save task status
	taskJSON, err := t.ToJSON()
	if err == nil {
		key := fmt.Sprintf("task:%s", t.ID)
		w.redisClient.Set(ctx, key, taskJSON, 0)
	}

	// Create a context with timeout if deadline is set
	execCtx := ctx
	var cancel context.CancelFunc

	if t.Deadline != nil && !t.Deadline.IsZero() {
		timeout := time.Until(*t.Deadline)
		if timeout <= 0 {
			// Task has already expired
			result.Status = task.StatusTimeout
			result.Error = "Task deadline exceeded before execution"
			result.EndTime = time.Now()
			w.saveResult(ctx, result)
			return
		}

		execCtx, cancel = context.WithTimeout(ctx, timeout)
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
	result.EndTime = time.Now()

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			result.Status = task.StatusTimeout
			result.Error = "Task execution timed out"
		} else {
			result.Status = task.StatusFailed
			result.Error = err.Error()

			// Check if the task should be retried
			if t.RetryCount < t.MaxRetries {
				t.RetryCount++
				t.Status = task.StatusRetrying
				t.NextRetryAt = time.Now().Add(time.Second * time.Duration(t.RetryCount) * 10) // Simple backoff strategy
				t.UpdatedAt = time.Now()

				// Re-queue the task with updated retry info
				w.requeueTask(ctx, t)
			}
		}
	} else {
		result.Status = task.StatusCompleted
	}

	// Update task status
	t.Status = result.Status
	t.UpdatedAt = time.Now()

	// Save the final task state
	updatedTaskJSON, err := t.ToJSON()
	if err == nil {
		key := fmt.Sprintf("task:%s", t.ID)
		w.redisClient.Set(ctx, key, updatedTaskJSON, 0)
	}

	// Save the result
	w.saveResult(ctx, result)
}

// executeTask executes a task based on its type
func (w *Worker) executeTask(ctx context.Context, t *task.Task, result *task.Result) error {
	// This is where you would implement task execution based on task type
	// For now, we'll just simulate task execution with a delay

	select {
	case <-time.After(time.Second * 2): // Simulate task execution
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// requeueTask puts a task back into the queue for retry
func (w *Worker) requeueTask(ctx context.Context, t *task.Task) {
	queueKey := fmt.Sprintf("queue:tasks:%s", t.Type)

	// Add to sorted set with priority as score and delay based on retry count
	score := float64(t.Priority) - (float64(t.RetryCount) * 0.1) // Lower priority slightly for retries

	taskJSON, err := t.ToJSON()
	if err != nil {
		log.Printf("Failed to serialize task for requeue: %v", err)
		return
	}

	// Add to sorted set
	if err := w.redisClient.ZAdd(ctx, queueKey, &redis.Z{
		Score:  score,
		Member: taskJSON,
	}).Err(); err != nil {
		log.Printf("Failed to requeue task: %v", err)
	}
}

// saveResult saves a task result to Redis
func (w *Worker) saveResult(ctx context.Context, result *task.Result) {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		log.Printf("Failed to serialize result: %v", err)
		return
	}

	// Save to results set
	resultKey := fmt.Sprintf("result:%s", result.TaskID)
	if err := w.redisClient.Set(ctx, resultKey, resultJSON, 24*time.Hour).Err(); err != nil {
		log.Printf("Failed to save result: %v", err)
	}

	// Add to history set
	historyKey := fmt.Sprintf("history:%s", result.TaskID)
	if err := w.redisClient.LPush(ctx, historyKey, resultJSON).Err(); err != nil {
		log.Printf("Failed to add to history: %v", err)
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
