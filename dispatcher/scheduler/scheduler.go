// ScheduleTask ensures a task is saved and properly queued for execution
func (s *Scheduler) ScheduleTask(ctx context.Context, task *task.Task, schedule *task.Schedule) error {
	// Log what we're scheduling
	log.Printf("Scheduling task: %s (ID: %s)", task.Name, task.ID)

	// Set task status
	if task.Status == "" {
		task.Status = task.StatusPending
	}

	// Save the task and schedule to Redis
	if err := s.saveTaskAndSchedule(ctx, task, schedule); err != nil {
		log.Printf("Failed to save task: %v", err)
		return err
	}

	// Check if we need to queue the task immediately
	queueNow := false
	if schedule.Type == task.ScheduleOneTime {
		if schedule.StartTime.IsZero() || schedule.StartTime.Before(time.Now()) {
			queueNow = true
		}
	}

	if queueNow {
		// Enqueue the task immediately
		if err := s.enqueueTask(ctx, task); err != nil {
			log.Printf("Failed to enqueue task: %v", err)
			return err
		}
	}

	return nil
} 