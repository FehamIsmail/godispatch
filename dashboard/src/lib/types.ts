export interface Task {
    id: string;
    type: string;
    name: string;
    description: string;
    priority: 'low' | 'medium' | 'high';
    status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
    payload: Record<string, any>;
    max_retries: number;
    timeout: number;
    created_at: string;
    updated_at: string;
}

export interface TaskSchedule {
    type: 'one-time' | 'recurring' | 'cron';
    expression?: string;
    interval?: number;
    start_time?: string;
    end_time?: string;
    time_zone?: string;
}

export interface CreateTaskRequest {
    type: string;
    name: string;
    description: string;
    priority: 'low' | 'medium' | 'high';
    payload: Record<string, any>;
    max_retries: number;
    timeout: number;
    schedule: TaskSchedule;
}

export interface TaskResult {
    task_id: string;
    status: string;
    output: any;
    error: string | null;
    started_at: string;
    completed_at: string;
}

export interface TaskHistory {
    task_id: string;
    status: string;
    timestamp: string;
    message: string;
}

export interface SystemMetrics {
    total_tasks: number;
    active_tasks: number;
    completed_tasks: number;
    failed_tasks: number;
    average_execution_time: number;
    worker_count: number;
}

export interface Worker {
    id: string;
    status: string;
    last_heartbeat: string;
    current_task: string | null;
} 