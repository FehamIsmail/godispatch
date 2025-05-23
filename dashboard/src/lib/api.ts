import type { Task, CreateTaskRequest, TaskResult, TaskHistory, SystemMetrics, Worker } from './types';

const API_BASE_URL = 'http://localhost:8080/api/v1';

async function fetchApi<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    try {
        const response = await fetch(`${API_BASE_URL}${endpoint}`, {
            ...options,
            headers: {
                'Content-Type': 'application/json',
                ...options.headers,
            },
        });

        // Handle errors from API
        if (!response.ok) {
            const errorText = await response.text();
            let errorMessage = `API error (${response.status}): ${response.statusText}`;
            
            try {
                const errorJson = JSON.parse(errorText);
                errorMessage = errorJson.error || errorMessage;
                throw new Error(errorJson.error ? JSON.stringify(errorJson) : errorMessage);
            } catch (parseError) {
                // Raw error text if not JSON
                throw new Error(errorText || errorMessage);
            }
        }

        return response.json();
    } catch (error) {
        // Log and rethrow errors
        if (error instanceof Error) {
            console.error(`Fetch error for ${endpoint}:`, error);
            throw error;
        }
        throw new Error(`Unknown error when fetching ${endpoint}`);
    }
}

// Convert backend result to frontend format
function transformTaskResult(result: any): TaskResult {
    return {
        task_id: result.task_id,
        status: result.status,
        output: result.output,
        error: result.error || null,
        started_at: result.start_time,
        completed_at: result.end_time,
    };
}

// Format history items for UI
function transformTaskHistory(results: any[]): TaskHistory[] {
    return results.map(result => ({
        task_id: result.task_id,
        status: result.status,
        timestamp: result.start_time,
        message: result.error || `Task ${result.status} by worker ${result.worker_id}`
    }));
}

export const api = {
    // Task endpoints
    createTask: (task: CreateTaskRequest) => 
        fetchApi<{ id: string; message: string }>('/tasks', {
            method: 'POST',
            body: JSON.stringify(task),
        }),

    listTasks: (
        status?: string, 
        limit: number = 10, 
        offset: number = 0,
        sortBy: string = 'created_at',
        sortOrder: 'asc' | 'desc' = 'desc'
    ) => {
        let url = `/tasks?limit=${limit}&offset=${offset}&sort=${sortBy}&order=${sortOrder}`;
        if (status !== undefined && status !== null && status !== 'undefined') {
            url += `&status=${status}`;
        }
        return fetchApi<{ tasks: Task[]; count: number; limit: number; offset: number }>(url);
    },

    getTask: (id: string) =>
        fetchApi<Task>(`/tasks/${id}`),

    cancelTask: (id: string) =>
        fetchApi<{ message: string }>(`/tasks/${id}`, {
            method: 'DELETE',
        }),

    getTaskResult: async (id: string) => {
        const result = await fetchApi<any>(`/tasks/${id}/result`);
        return transformTaskResult(result);
    },

    getTaskHistory: async (id: string) => {
        const response = await fetchApi<{ history: any[]; count: number }>(`/tasks/${id}/history`);
        return {
            history: transformTaskHistory(response.history),
            count: response.count
        };
    },

    // System endpoints
    getSystemMetrics: () =>
        fetchApi<SystemMetrics>('/system/metrics'),

    getWorkers: () =>
        fetchApi<{ workers: Worker[] }>('/system/workers'),
}; 