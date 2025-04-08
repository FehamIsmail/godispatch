# Go Job Scheduling System (GoDispatch)

A robust job scheduling system implemented in Go, designed for distributed task execution with flexible scheduling patterns, fault tolerance, and monitoring capabilities.

## Features

- **Job Management**: Create, read, update, and delete jobs with comprehensive metadata (ID, name, description, priority, status, etc.)
- **Flexible Scheduling**: Support for one-time jobs, recurring jobs, and cron expressions with time zone handling
- **Distributed Execution**: Concurrent processing across multiple workers with queue management and resource allocation
- **Monitoring**: Real-time status tracking, execution history, and performance metrics
- **RESTful API**: Complete API for job management, scheduling, and monitoring
- **Fault Tolerance**: Job recovery, failed job handling, and system restart capabilities
- **Persistence**: Redis-based storage for jobs, schedules, and results

## Architecture

The system consists of several components:

- **Controller**: Coordinates the scheduler and workers
- **Scheduler**: Manages job scheduling and queue management
- **Workers**: Process tasks from queues
- **API Server**: Provides RESTful endpoints for interacting with the system

## Requirements

- Go 1.20 or higher
- Redis 6.0 or higher

## Getting Started

### Installation

```bash
git clone https://github.com/yourusername/godispatch.git
cd godispatch
go mod tidy
```

### Configuration

The system can be configured through environment variables or a JSON configuration file:

```bash
# Using environment variables
export SERVER_PORT=8080
export REDIS_ADDRESS=localhost:6379

# Or using a configuration file
export CONFIG_FILE=/path/to/config.json
```

Example configuration file:

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": "8080",
    "read_timeout": 30000000000,
    "write_timeout": 30000000000
  },
  "redis": {
    "address": "localhost:6379",
    "password": "",
    "db": 0
  },
  "worker": {
    "count": 5,
    "queue_size": 100,
    "poll_interval": 1000000000,
    "max_concurrent": 10
  },
  "scheduler": {
    "max_retries": 3,
    "default_timeout": 600000000000,
    "heartbeat_interval": 5000000000,
    "retry_backoff": 30000000000
  }
}
```

### Running the System

```bash
go run main.go
```

## API Usage

### Creating a Task

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "name": "Send welcome email",
    "description": "Send a welcome email to new users",
    "priority": "high",
    "payload": {
      "to": "user@example.com",
      "subject": "Welcome!",
      "body": "Welcome to our service!"
    },
    "max_retries": 3,
    "timeout": 60,
    "schedule": {
      "type": "one-time",
      "start_time": "2023-04-10T15:00:00Z"
    }
  }'
```

### Getting Task Status

```bash
curl http://localhost:8080/api/v1/tasks/{task_id}
```

### Getting Task Result

```bash
curl http://localhost:8080/api/v1/tasks/{task_id}/result
```

### Canceling a Task

```bash
curl -X DELETE http://localhost:8080/api/v1/tasks/{task_id}
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.
