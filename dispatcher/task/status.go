package task

// Status represents the current state of a task
type Status string

// Task status constants
const (
	StatusPending   Status = "pending"
	StatusScheduled Status = "scheduled"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
	StatusRetrying  Status = "retrying"
	StatusTimeout   Status = "timeout"
)

// Priority represents the priority level of a task
type Priority string

// Task priority constants
const (
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

// PriorityToInt converts a Priority to an integer value for sorting
func PriorityToInt(p Priority) int {
	switch p {
	case PriorityHigh:
		return 3
	case PriorityMedium:
		return 2
	case PriorityLow:
		return 1
	default:
		return 0
	}
}
