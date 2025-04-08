// Config serves as the configuration for the scheduler
// It contains the configuration for the scheduler
// Contains structs types
package config

import (
	"encoding/json"
	"os"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	Server    ServerConfig    `json:"server"`
	Redis     RedisConfig     `json:"redis"`
	Worker    WorkerConfig    `json:"worker"`
	Scheduler SchedulerConfig `json:"scheduler"`
}

// ServerConfig holds API server configuration
type ServerConfig struct {
	Host         string        `json:"host"`
	Port         string        `json:"port"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Address  string `json:"address"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

// WorkerConfig holds worker configuration
type WorkerConfig struct {
	Count         int           `json:"count"`
	QueueSize     int           `json:"queue_size"`
	PollInterval  time.Duration `json:"poll_interval"`
	MaxConcurrent int           `json:"max_concurrent"`
}

// SchedulerConfig holds scheduler configuration
type SchedulerConfig struct {
	MaxRetries        int           `json:"max_retries"`
	DefaultTimeout    time.Duration `json:"default_timeout"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	RetryBackoff      time.Duration `json:"retry_backoff"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         "8080",
			ReadTimeout:  time.Second * 30,
			WriteTimeout: time.Second * 30,
		},
		Redis: RedisConfig{
			Address:  "localhost:6379",
			Password: "",
			DB:       0,
		},
		Worker: WorkerConfig{
			Count:         5,
			QueueSize:     100,
			PollInterval:  time.Second * 1,
			MaxConcurrent: 10,
		},
		Scheduler: SchedulerConfig{
			MaxRetries:        3,
			DefaultTimeout:    time.Minute * 10,
			HeartbeatInterval: time.Second * 5,
			RetryBackoff:      time.Second * 30,
		},
	}
}

// Load loads configuration from file or environment variables
func Load() (*Config, error) {
	config := DefaultConfig()

	// Try to load from config file if exists
	if configFile := os.Getenv("CONFIG_FILE"); configFile != "" {
		file, err := os.Open(configFile)
		if err == nil {
			defer file.Close()
			decoder := json.NewDecoder(file)
			if err := decoder.Decode(config); err != nil {
				return nil, err
			}
		}
	}

	// Override with environment variables if set
	// This is a simplified version - you might want to expand this
	if port := os.Getenv("SERVER_PORT"); port != "" {
		config.Server.Port = port
	}
	if redisAddr := os.Getenv("REDIS_ADDRESS"); redisAddr != "" {
		config.Redis.Address = redisAddr
	}

	return config, nil
}
