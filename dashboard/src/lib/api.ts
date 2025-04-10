import type { Task, CreateTaskRequest, TaskResult, TaskHistory, SystemMetrics, Worker } from './types';

const API_BASE_URL = 'http://localhost:8080/api/v1';

async function fetchApi<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const response = await fetch(`${API_BASE_URL}${endpoint}`, {
        ...options,
        headers: {
            'Content-Type': 'application/json',
            ...options.headers,
        },
    });

    if (!response.ok) {
        throw new Error(`API error: ${response.statusText}`);
    }

    return response.json();
}

export const api = {
    // Task endpoints
    createTask: (task: CreateTaskRequest) => 
        fetchApi<{ id: string; message: string }>('/tasks', {
            method: 'POST',
            body: JSON.stringify(task),
        }),

    listTasks: (status?: string, limit: number = 10, offset: number = 0) =>
        fetchApi<{ tasks: Task[]; count: number; limit: number; offset: number }>(
            `/tasks?status=${status}&limit=${limit}&offset=${offset}`
        ),

    getTask: (id: string) =>
        fetchApi<Task>(`/tasks/${id}`),

    cancelTask: (id: string) =>
        fetchApi<{ message: string }>(`/tasks/${id}`, {
            method: 'DELETE',
        }),

    getTaskResult: (id: string) =>
        fetchApi<TaskResult>(`/tasks/${id}/result`),

    getTaskHistory: (id: string) =>
        fetchApi<{ history: TaskHistory[]; count: number }>(`/tasks/${id}/history`),

    // System endpoints
    getSystemMetrics: () =>
        fetchApi<SystemMetrics>('/system/metrics'),

    getWorkers: () =>
        fetchApi<{ workers: Worker[] }>('/system/workers'),
}; 