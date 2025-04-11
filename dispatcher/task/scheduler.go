// Scheduler is handles task scheduling and execution
// It contains the logic for scheduling task, on failure, retry logic, etc.
package task

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"GoDispatch/dispatcher/config"

	"github.com/go-redis/redis/v8"
	"github.com/gorhill/cronexpr"
)

// ScheduleType defines the type of scheduling
type ScheduleType string

const (
	// ScheduleOneTime represents a task that runs only once
	ScheduleOneTime ScheduleType = "one-time"
	// ScheduleRecurring represents a task that runs on a schedule
	ScheduleRecurring ScheduleType = "recurring"
	// ScheduleCron represents a task that runs on a cron schedule
	ScheduleCron ScheduleType = "cron"
)

// Schedule represents a task schedule
type Schedule struct {
	Type       ScheduleType `json:"type"`
	Expression string       `json:"expression,omitempty"` // cron expression
	Interval   int64        `json:"interval,omitempty"`   // in seconds, for recurring tasks
	StartTime  time.Time    `json:"start_time,omitempty"` // for one-time or start of recurring
	EndTime    time.Time    `json:"end_time,omitempty"`   // optional end time for recurring
	TimeZone   string       `json:"time_zone,omitempty"`  // timezone for the schedule
}

// Scheduler manages task scheduling
type Scheduler struct {
	config      *config.SchedulerConfig
	redisClient *redis.Client
	tasks       map[string]*Task
	schedules   map[string]*Schedule
	mu          sync.RWMutex
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewScheduler creates a new scheduler
func NewScheduler(cfg *config.SchedulerConfig, redisClient *redis.Client) *Scheduler {
	return &Scheduler{
		config:      cfg,
		redisClient: redisClient,
		tasks:       make(map[string]*Task),
		schedules:   make(map[string]*Schedule),
		stopCh:      make(chan struct{}),
	}
}

// Start starts the scheduler
func (s *Scheduler) Start(ctx context.Context) error {
	// Load existing tasks and schedules from Redis
	if err := s.loadTasks(ctx); err != nil {
		return fmt.Errorf("failed to load tasks: %w", err)
	}

	// Start background scheduler loop
	s.wg.Add(1)
	go s.scheduleLoop(ctx)

	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() error {
	close(s.stopCh)
	s.wg.Wait()
	return nil
}

// ScheduleTask schedules a task for execution
func (s *Scheduler) ScheduleTask(ctx context.Context, task *Task, schedule *Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("Scheduling task: ID=%s, Type=%s, Name=%s, Status=%s", task.ID, task.Type, task.Name, task.Status)

	// Validate task and schedule
	if task.ID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}

	if schedule.Type == ScheduleCron && schedule.Expression == "" {
		return fmt.Errorf("cron expression cannot be empty for cron schedule")
	}

	if schedule.Type == ScheduleRecurring && schedule.Interval <= 0 {
		return fmt.Errorf("interval must be positive for recurring schedule")
	}

	// Set task status to scheduled
	task.Status = StatusScheduled
	task.UpdatedAt = time.Now()

	log.Printf("Setting task status to 'scheduled': ID=%s", task.ID)

	// Store task and schedule in memory
	s.tasks[task.ID] = task
	s.schedules[task.ID] = schedule

	// Store in Redis for persistence
	if err := s.saveTaskAndSchedule(ctx, task, schedule); err != nil {
		log.Printf("Failed to save task and schedule: %v", err)
		return fmt.Errorf("failed to save task and schedule: %w", err)
	}

	// If the task is scheduled to run now, enqueue it
	if shouldRunNow(schedule) {
		log.Printf("Task scheduled to run immediately: ID=%s", task.ID)
		return s.enqueueTask(ctx, task)
	}

	log.Printf("Task scheduled successfully: ID=%s", task.ID)
	return nil
}

// CancelTask cancels a scheduled task
func (s *Scheduler) CancelTask(ctx context.Context, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// First, try to get the task from our in-memory map
	task, exists := s.tasks[taskID]

	// If not in memory, try to get it from Redis
	if !exists {
		// Check Redis for the task
		key := fmt.Sprintf("task:%s", taskID)
		taskJSON, err := s.redisClient.Get(ctx, key).Result()
		if err != nil {
			if err == redis.Nil {
				return fmt.Errorf("task not found: %s", taskID)
			}
			return fmt.Errorf("failed to fetch task from Redis: %w", err)
		}

		// Parse the task
		parsedTask, parseErr := TaskFromJSON([]byte(taskJSON))
		if parseErr != nil {
			return fmt.Errorf("failed to parse task: %w", parseErr)
		}

		// Add to our in-memory map for cancellation
		task = parsedTask
		s.tasks[taskID] = task
	}

	// Only allow cancellation of tasks that are not already completed, failed, or cancelled
	if task.Status == StatusCompleted || task.Status == StatusFailed || task.Status == StatusCancelled {
		return fmt.Errorf("cannot cancel task with status: %s", task.Status)
	}

	// Update the task status
	task.Status = StatusCancelled
	task.UpdatedAt = time.Now()

	log.Printf("Cancelling task %s, previous status: %s", taskID, task.Status)

	// Update in Redis
	if err := s.saveTask(ctx, task); err != nil {
		return fmt.Errorf("failed to save cancelled task: %w", err)
	}

	// Remove from schedules
	delete(s.schedules, taskID)

	// If task is in a queue, remove it
	if err := s.removeTaskFromQueues(ctx, task); err != nil {
		log.Printf("Failed to remove task from queues: %v", err)
		// Don't return error as we've already updated the task status
	}

	return nil
}

// removeTaskFromQueues removes a task from all queues
func (s *Scheduler) removeTaskFromQueues(ctx context.Context, task *Task) error {
	// Try to remove from type-specific queue
	queueKey := fmt.Sprintf("queue:tasks:%s", task.Type)
	taskJSON, err := task.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize task: %w", err)
	}

	_, err = s.redisClient.ZRem(ctx, queueKey, taskJSON).Result()
	if err != nil {
		log.Printf("Error removing task from type queue: %v", err)
	}

	// Also try to remove from all_pending queue
	_, err = s.redisClient.ZRem(ctx, "queue:tasks:all_pending", taskJSON).Result()
	if err != nil {
		log.Printf("Error removing task from all_pending queue: %v", err)
	}

	// Remove from pending tasks set
	_, err = s.redisClient.SRem(ctx, "set:pending_tasks", task.ID).Result()
	if err != nil {
		log.Printf("Error removing task from pending set: %v", err)
	}

	return nil
}

// scheduleLoop is the main scheduler loop
func (s *Scheduler) scheduleLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkSchedules(ctx)
		}
	}
}

// checkSchedules checks all schedules and enqueues tasks that are due
func (s *Scheduler) checkSchedules(ctx context.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()

	for taskID, schedule := range s.schedules {
		task := s.tasks[taskID]
		if task == nil {
			continue
		}

		// Allow both scheduled tasks and retrying tasks to be processed
		if task.Status != StatusScheduled && task.Status != StatusRetrying {
			continue
		}

		// For retrying tasks, check if it's time to retry based on NextRetryAt
		if task.Status == StatusRetrying && task.NextRetryAt.After(now) {
			continue // Not yet time to retry
		}

		if isDue(schedule, now) || task.Status == StatusRetrying {
			taskCopy := *task
			scheduleCopy := *schedule
			taskIDCopy := taskID

			go func() {
				// Using a separate context to avoid cancellation
				enqCtx := context.Background()

				if err := s.enqueueTask(enqCtx, &taskCopy); err != nil {
					log.Printf("Failed to enqueue task %s: %v", taskCopy.ID, err)
				}

				// For recurring tasks, update the next execution time
				if scheduleCopy.Type == ScheduleRecurring || scheduleCopy.Type == ScheduleCron {
					s.mu.Lock()
					defer s.mu.Unlock()

					// The task might have been cancelled while we were enqueueing
					currentTask, exists := s.tasks[taskIDCopy]
					if exists && currentTask.Status == StatusScheduled {
						// Keep it scheduled for next execution
						// For recurring tasks with intervals, simply leave as is
						// For cron tasks, we don't need to update anything as the expression determines the schedule
					}
				} else if scheduleCopy.Type == ScheduleOneTime {
					// One-time task should be removed from schedules
					s.mu.Lock()
					defer s.mu.Unlock()
					delete(s.schedules, taskIDCopy)
				}
			}()
		}
	}
}

// enqueueTask adds a task to the execution queue
func (s *Scheduler) enqueueTask(ctx context.Context, task *Task) error {
	// Set task status to pending
	task.Status = StatusPending
	task.UpdatedAt = time.Now()

	// Store task in Redis queue
	queueKey := fmt.Sprintf("queue:tasks:%s", task.Type)

	// Add to sorted set with priority as score
	score := float64(task.Priority)
	if task.CreatedAt.Unix() > 0 {
		// Use creation time as tiebreaker
		score += float64(task.CreatedAt.Unix()) / 1e10
	}

	taskJSON, err := task.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize task: %w", err)
	}

	// Log task being enqueued
	log.Printf("Enqueueing task: ID=%s, Type=%s, Queue=%s", task.ID, task.Type, queueKey)

	// First check if the task already exists in the queue to avoid duplicates
	count, err := s.redisClient.ZScore(ctx, queueKey, string(taskJSON)).Result()
	if err != nil && err != redis.Nil {
		log.Printf("Error checking if task exists in queue: %v", err)
	}

	if count > 0 {
		log.Printf("Task %s already exists in queue %s, skipping enqueue", task.ID, queueKey)
	} else {
		// Add to a sorted set
		if err := s.redisClient.ZAdd(ctx, queueKey, &redis.Z{
			Score:  score,
			Member: taskJSON,
		}).Err(); err != nil {
			log.Printf("Failed to add task %s to queue %s: %v", task.ID, queueKey, err)
			return fmt.Errorf("failed to add task to queue: %w", err)
		}
		log.Printf("Successfully added task %s to queue %s", task.ID, queueKey)
	}

	// Also add to a backup queue with all pending tasks, regardless of type
	backupQueueKey := "queue:tasks:all_pending"
	if err := s.redisClient.ZAdd(ctx, backupQueueKey, &redis.Z{
		Score:  score,
		Member: taskJSON,
	}).Err(); err != nil {
		log.Printf("Failed to add task %s to backup queue: %v", task.ID, err)
		// Don't return error here, as the task is already in the main queue
	}

	// Update the task in storage
	if err := s.saveTask(ctx, task); err != nil {
		return fmt.Errorf("failed to save task: %w", err)
	}

	// Also save a reference to the pending task in a set for easier scanning
	pendingSetKey := "set:pending_tasks"
	if err := s.redisClient.SAdd(ctx, pendingSetKey, task.ID).Err(); err != nil {
		log.Printf("Failed to add task ID to pending set: %v", err)
		// Don't return error here as it's just for optimization
	}

	return nil
}

// saveTask saves a task to Redis
func (s *Scheduler) saveTask(ctx context.Context, task *Task) error {
	taskJSON, err := task.ToJSON()
	if err != nil {
		return err
	}

	key := fmt.Sprintf("task:%s", task.ID)
	return s.redisClient.Set(ctx, key, taskJSON, 0).Err()
}

// saveTaskAndSchedule saves both task and schedule to Redis
func (s *Scheduler) saveTaskAndSchedule(ctx context.Context, task *Task, schedule *Schedule) error {
	if err := s.saveTask(ctx, task); err != nil {
		return err
	}

	log.Printf("Saved task to Redis: key=task:%s", task.ID)

	scheduleJSON, err := serializeSchedule(schedule)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("schedule:%s", task.ID)
	if err := s.redisClient.Set(ctx, key, scheduleJSON, 0).Err(); err != nil {
		return err
	}

	log.Printf("Saved schedule to Redis: key=%s", key)
	return nil
}

// loadTasks loads tasks and schedules from Redis
func (s *Scheduler) loadTasks(ctx context.Context) error {
	// Get all task keys
	taskKeys, err := s.redisClient.Keys(ctx, "task:*").Result()
	if err != nil {
		return err
	}

	for _, key := range taskKeys {
		// Extract task ID from key
		taskID := key[5:] // Remove "task:" prefix

		// Get task data
		taskJSON, err := s.redisClient.Get(ctx, key).Result()
		if err != nil {
			log.Printf("Failed to get task %s: %v", taskID, err)
			continue
		}

		task, err := TaskFromJSON([]byte(taskJSON))
		if err != nil {
			log.Printf("Failed to parse task %s: %v", taskID, err)
			continue
		}

		// Get schedule data
		scheduleKey := fmt.Sprintf("schedule:%s", taskID)
		scheduleJSON, err := s.redisClient.Get(ctx, scheduleKey).Result()
		if err != nil && err != redis.Nil {
			log.Printf("Failed to get schedule for task %s: %v", taskID, err)
			continue
		}

		// Store task in memory
		s.mu.Lock()
		s.tasks[taskID] = task

		// If schedule exists, parse and store it
		if err != redis.Nil && scheduleJSON != "" {
			schedule, err := deserializeSchedule([]byte(scheduleJSON))
			if err != nil {
				log.Printf("Failed to parse schedule for task %s: %v", taskID, err)
				s.mu.Unlock()
				continue
			}

			s.schedules[taskID] = schedule
		}
		s.mu.Unlock()
	}

	return nil
}

// Helper functions

// isDue checks if a scheduled task is due for execution
func isDue(schedule *Schedule, now time.Time) bool {
	switch schedule.Type {
	case ScheduleOneTime:
		return now.After(schedule.StartTime) || now.Equal(schedule.StartTime)

	case ScheduleRecurring:
		if now.Before(schedule.StartTime) {
			return false
		}

		if !schedule.EndTime.IsZero() && now.After(schedule.EndTime) {
			return false
		}

		elapsed := now.Unix() - schedule.StartTime.Unix()
		return elapsed%schedule.Interval == 0

	case ScheduleCron:
		if now.Before(schedule.StartTime) {
			return false
		}

		if !schedule.EndTime.IsZero() && now.After(schedule.EndTime) {
			return false
		}

		expr, err := cronexpr.Parse(schedule.Expression)
		if err != nil {
			log.Printf("Invalid cron expression: %s", schedule.Expression)
			return false
		}

		nextTime := expr.Next(now.Add(-time.Second)) // Look back 1 second to catch exact matches
		return nextTime.Before(now.Add(time.Second)) // Due if next time is within 1 second
	}

	return false
}

// shouldRunNow checks if a task should run immediately after scheduling
func shouldRunNow(schedule *Schedule) bool {
	now := time.Now()

	switch schedule.Type {
	case ScheduleOneTime:
		return now.After(schedule.StartTime) || now.Equal(schedule.StartTime)

	case ScheduleRecurring, ScheduleCron:
		return schedule.StartTime.IsZero() || now.After(schedule.StartTime) || now.Equal(schedule.StartTime)
	}

	return false
}

// serializeSchedule serializes a schedule to JSON
func serializeSchedule(schedule *Schedule) ([]byte, error) {
	return marshalJSON(schedule)
}

// deserializeSchedule deserializes a schedule from JSON
func deserializeSchedule(data []byte) (*Schedule, error) {
	var schedule Schedule
	if err := unmarshalJSON(data, &schedule); err != nil {
		return nil, err
	}
	return &schedule, nil
}
