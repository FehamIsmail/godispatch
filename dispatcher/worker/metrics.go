// We wish to track metrics, we will have a metricsCollector
// we will track time, queue wait time, memory usage, cpu usage, active workers, idle workers
package worker

import (
	"runtime"
	"sync"
	"time"

	"GoDispatch/dispatcher/task"
)

// TaskMetricsStart holds metrics at the start of task execution
type TaskMetricsStart struct {
	StartTime time.Time
	MemAlloc  uint64
	CPUTime   float64
}

// MetricsStats holds aggregated metrics statistics
type MetricsStats struct {
	ActiveTasks       int            `json:"active_tasks"`
	CompletedTasks    int            `json:"completed_tasks"`
	FailedTasks       int            `json:"failed_tasks"`
	TotalTasks        int            `json:"total_tasks"`
	AvgProcessingTime time.Duration  `json:"avg_processing_time"`
	AvgQueueWaitTime  time.Duration  `json:"avg_queue_wait_time"`
	AvgMemoryUsage    uint64         `json:"avg_memory_usage"`
	AvgCPUTime        float64        `json:"avg_cpu_time"`
	TasksByStatus     map[string]int `json:"tasks_by_status"`
	LastUpdated       time.Time      `json:"last_updated"`
}

// MetricsCollector collects and tracks worker metrics
type MetricsCollector struct {
	mu               sync.Mutex
	activeTasks      map[string]TaskMetricsStart
	completedTasks   int
	failedTasks      int
	totalTasks       int
	processingTimes  []time.Duration
	queueWaitTimes   []time.Duration
	memoryUsage      []uint64
	cpuTime          []float64
	taskStatusCounts map[string]int
	lastUpdated      time.Time
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		activeTasks:      make(map[string]TaskMetricsStart),
		processingTimes:  make([]time.Duration, 0, 100),
		queueWaitTimes:   make([]time.Duration, 0, 100),
		memoryUsage:      make([]uint64, 0, 100),
		cpuTime:          make([]float64, 0, 100),
		taskStatusCounts: make(map[string]int),
		lastUpdated:      time.Now(),
	}
}

// StartTask records the start of a task execution
func (m *MetricsCollector) StartTask(taskID string) TaskMetricsStart {
	m.mu.Lock()
	defer m.mu.Unlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metrics := TaskMetricsStart{
		StartTime: time.Now(),
		MemAlloc:  memStats.Alloc,
		CPUTime:   getCPUTime(),
	}

	m.activeTasks[taskID] = metrics
	m.totalTasks++

	return metrics
}

// EndTask records the end of a task execution
func (m *MetricsCollector) EndTask(taskID string, startMetrics TaskMetricsStart) *task.TaskMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.activeTasks, taskID)

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	endTime := time.Now()
	processingTime := endTime.Sub(startMetrics.StartTime)
	memoryUsage := memStats.Alloc - startMetrics.MemAlloc
	cpuTime := getCPUTime() - startMetrics.CPUTime

	// Store metrics for aggregation
	m.processingTimes = append(m.processingTimes, processingTime)
	m.memoryUsage = append(m.memoryUsage, memoryUsage)
	m.cpuTime = append(m.cpuTime, cpuTime)

	// Limit the size of metrics arrays
	if len(m.processingTimes) > 1000 {
		m.processingTimes = m.processingTimes[len(m.processingTimes)-1000:]
		m.memoryUsage = m.memoryUsage[len(m.memoryUsage)-1000:]
		m.cpuTime = m.cpuTime[len(m.cpuTime)-1000:]
	}

	// Update the last updated time
	m.lastUpdated = time.Now()

	return &task.TaskMetrics{
		ProcessingTime: processingTime,
		MemoryUsage:    memoryUsage,
		CPUTime:        cpuTime,
	}
}

// RecordTaskStatus records a task status for metrics
func (m *MetricsCollector) RecordTaskStatus(status task.Status) {
	m.mu.Lock()
	defer m.mu.Unlock()

	statusStr := string(status)
	m.taskStatusCounts[statusStr]++

	if status == task.StatusCompleted {
		m.completedTasks++
	} else if status == task.StatusFailed {
		m.failedTasks++
	}
}

// GetStats returns current metrics statistics
func (m *MetricsCollector) GetStats() *MetricsStats {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := &MetricsStats{
		ActiveTasks:    len(m.activeTasks),
		CompletedTasks: m.completedTasks,
		FailedTasks:    m.failedTasks,
		TotalTasks:     m.totalTasks,
		TasksByStatus:  make(map[string]int),
		LastUpdated:    m.lastUpdated,
	}

	// Copy the task status counts
	for status, count := range m.taskStatusCounts {
		stats.TasksByStatus[status] = count
	}

	// Calculate averages
	if len(m.processingTimes) > 0 {
		var totalProcessingTime time.Duration
		var totalMemoryUsage uint64
		var totalCPUTime float64

		for _, t := range m.processingTimes {
			totalProcessingTime += t
		}

		for _, m := range m.memoryUsage {
			totalMemoryUsage += m
		}

		for _, c := range m.cpuTime {
			totalCPUTime += c
		}

		stats.AvgProcessingTime = totalProcessingTime / time.Duration(len(m.processingTimes))
		stats.AvgMemoryUsage = totalMemoryUsage / uint64(len(m.memoryUsage))
		stats.AvgCPUTime = totalCPUTime / float64(len(m.cpuTime))
	}

	return stats
}

// getCPUTime returns a simple approximation of CPU time
// In a real system, you might use a more sophisticated method
func getCPUTime() float64 {
	// This is a placeholder for a real CPU time measurement
	// In a production system, you would use OS-specific APIs
	// or a library like gopsutil to get actual CPU usage
	return float64(time.Now().UnixNano())
}
