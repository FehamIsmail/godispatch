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
git clone https://github.com/yourusername/GoDispatch.git
cd GoDispatch
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

### Frontend Dashboard

The system includes a web-based dashboard built with Svelte for monitoring and managing jobs.

#### Dashboard Setup

1. Navigate to the dashboard directory:
```bash
cd dashboard
```

2. Install dependencies:
```bash
npm install
# or
yarn install
# or
pnpm install
```

3. Start the development server:
```bash
npm run dev
# or start the server and open the app in a new browser tab
npm run dev -- --open
```

4. For production build:
```bash
npm run build
```

5. Preview the production build:
```bash
npm run preview
```

## Web UI screenshots

### Dashboard overview

![Screenshot 2025-04-11 220913](https://github.com/user-attachments/assets/6d7cc499-8094-4f47-9c3b-0de1c6deaf0a)

### Creating a Task

![Screenshot 2025-04-11 220111](https://github.com/user-attachments/assets/6c22a1d5-dfe4-4d72-bddc-4ebc01940f13)

### Listing all tasks

![Screenshot 2025-04-11 220119](https://github.com/user-attachments/assets/e2634190-0029-4ced-89c2-61ca70e84ec0)

### Task status update `pending` -> `running`

![Screenshot 2025-04-11 220125](https://github.com/user-attachments/assets/00eaff42-8807-4956-af11-b4c5f443f74c)

### Task status update `running` -> `completed`

![Screenshot 2025-04-11 220223](https://github.com/user-attachments/assets/e271f1d3-2816-462b-a291-009aa3da35db)

### Getting task detail information

![Screenshot 2025-04-11 220239](https://github.com/user-attachments/assets/164f0e88-8e85-4b10-88c1-3d1e76ca8937)

### System overview (workers list)

![Screenshot 2025-04-11 220312](https://github.com/user-attachments/assets/896e4dc0-77ea-4100-be3c-aef4cee1bc46)


