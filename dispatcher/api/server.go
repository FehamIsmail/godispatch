// Server serves as the API server for the scheduler
package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"GoDispatch/dispatcher/config"
	"GoDispatch/dispatcher/controller"
	"GoDispatch/dispatcher/task"

	"encoding/json"

	"github.com/gin-gonic/gin"
)

// Server represents the API server
type Server struct {
	config     *config.Config
	controller *controller.Controller
	httpServer *http.Server
	router     *gin.Engine
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, ctrl *controller.Controller) *Server {
	router := gin.Default()
	server := &Server{
		config:     cfg,
		controller: ctrl,
		router:     router,
	}

	// Set up routes
	server.setupRoutes()

	return server
}

// Start starts the API server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%s", s.config.Server.Host, s.config.Server.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.config.Server.ReadTimeout,
		WriteTimeout: s.config.Server.WriteTimeout,
	}

	return s.httpServer.ListenAndServe()
}

// Stop stops the API server
func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// setupRoutes sets up the API routes
func (s *Server) setupRoutes() {
	v1 := s.router.Group("/api/v1")
	{
		v1.GET("/health", s.healthCheck)

		// Task routes
		tasks := v1.Group("/tasks")
		{
			tasks.POST("", s.createTask)
			tasks.GET("", s.listTasks)
			tasks.GET("/:id", s.getTask)
			tasks.PUT("/:id", s.updateTask)
			tasks.DELETE("/:id", s.cancelTask)
			tasks.GET("/:id/result", s.getTaskResult)
			tasks.GET("/:id/history", s.getTaskHistory)
		}

		// Schedule routes
		schedules := v1.Group("/schedules")
		{
			schedules.POST("", s.createSchedule)
			schedules.GET("", s.listSchedules)
			schedules.GET("/:id", s.getSchedule)
			schedules.PUT("/:id", s.updateSchedule)
			schedules.DELETE("/:id", s.deleteSchedule)
		}

		// System routes
		system := v1.Group("/system")
		{
			system.GET("/metrics", s.getSystemMetrics)
			system.GET("/workers", s.getWorkers)
		}
	}
}

// healthCheck handles health check requests
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// createTask handles task creation requests
func (s *Server) createTask(c *gin.Context) {
	var req struct {
		Type        string                 `json:"type" binding:"required"`
		Name        string                 `json:"name" binding:"required"`
		Description string                 `json:"description"`
		Priority    string                 `json:"priority"`
		Payload     map[string]interface{} `json:"payload"`
		MaxRetries  int                    `json:"max_retries"`
		Timeout     int64                  `json:"timeout"` // in seconds
		Schedule    struct {
			Type       string    `json:"type" binding:"required"`
			Expression string    `json:"expression,omitempty"`
			Interval   int64     `json:"interval,omitempty"`
			StartTime  time.Time `json:"start_time,omitempty"`
			EndTime    time.Time `json:"end_time,omitempty"`
			TimeZone   string    `json:"time_zone,omitempty"`
		} `json:"schedule" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert payload to JSON
	payloadJSON, err := json.Marshal(req.Payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	// Convert priority string to int
	priorityInt := 1 // Default to medium
	if req.Priority == "high" {
		priorityInt = 2
	} else if req.Priority == "low" {
		priorityInt = 0
	}

	// Create task
	t := &task.Task{
		Type:       req.Type,
		Payload:    payloadJSON,
		Priority:   priorityInt,
		MaxRetries: req.MaxRetries,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Set timeout if provided
	if req.Timeout > 0 {
		deadline := time.Now().Add(time.Duration(req.Timeout) * time.Second)
		t.Deadline = &deadline
	}

	// Create schedule
	schedule := &task.Schedule{
		Type:       task.ScheduleType(req.Schedule.Type),
		Expression: req.Schedule.Expression,
		Interval:   req.Schedule.Interval,
		StartTime:  req.Schedule.StartTime,
		EndTime:    req.Schedule.EndTime,
		TimeZone:   req.Schedule.TimeZone,
	}

	// Validate schedule
	if schedule.Type == task.ScheduleCron && schedule.Expression == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cron expression is required for cron schedule"})
		return
	}

	if schedule.Type == task.ScheduleRecurring && schedule.Interval <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Interval must be positive for recurring schedule"})
		return
	}

	// Schedule task
	taskID, err := s.controller.CreateTask(c.Request.Context(), t, schedule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":      taskID,
		"message": "Task scheduled successfully",
	})
}

// listTasks handles task listing requests
func (s *Server) listTasks(c *gin.Context) {
	statusStr := c.Query("status")
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit"})
		return
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid offset"})
		return
	}

	var status task.Status
	if statusStr != "" {
		status = task.Status(statusStr)
	}

	tasks, err := s.controller.ListTasks(c.Request.Context(), status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks":  tasks,
		"count":  len(tasks),
		"limit":  limit,
		"offset": offset,
	})
}

// getTask handles task retrieval requests
func (s *Server) getTask(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task ID is required"})
		return
	}

	task, err := s.controller.GetTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// updateTask handles task update requests
func (s *Server) updateTask(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task ID is required"})
		return
	}

	// For now, we only support cancellation via PUT
	if err := s.controller.CancelTask(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task cancelled successfully"})
}

// cancelTask handles task cancellation requests
func (s *Server) cancelTask(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task ID is required"})
		return
	}

	if err := s.controller.CancelTask(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task cancelled successfully"})
}

// getTaskResult handles task result retrieval requests
func (s *Server) getTaskResult(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task ID is required"})
		return
	}

	result, err := s.controller.GetTaskResult(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// getTaskHistory handles task history retrieval requests
func (s *Server) getTaskHistory(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task ID is required"})
		return
	}

	history, err := s.controller.GetTaskHistory(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"history": history,
		"count":   len(history),
	})
}

// Below are stub implementations for schedule-related endpoints

func (s *Server) createSchedule(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented"})
}

func (s *Server) listSchedules(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented"})
}

func (s *Server) getSchedule(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented"})
}

func (s *Server) updateSchedule(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented"})
}

func (s *Server) deleteSchedule(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented"})
}

// Below are stub implementations for system-related endpoints

func (s *Server) getSystemMetrics(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented"})
}

func (s *Server) getWorkers(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented"})
}
