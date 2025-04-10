<script lang="ts">
    import { onMount } from 'svelte';
    import { api } from '$lib/api';
    import type { Task, CreateTaskRequest } from '$lib/types';
    import TaskForm from '$lib/components/TaskForm.svelte';
    import Table from '$lib/components/Table.svelte';

    let tasks: Task[] = [];
    let loading = true;
    let error: string | null = null;
    let showCreateForm = false;
    let newTask: CreateTaskRequest = {
        type: '',
        name: '',
        description: '',
        priority: 'medium',
        payload: {},
        max_retries: 3,
        timeout: 60,
        schedule: {
            type: 'one-time',
            start_time: new Date().toISOString().slice(0, 16) // Format: YYYY-MM-DDTHH:MM
        }
    };
    let formErrors: Record<string, string> = {};

    // Define table columns
    const columns = [
        { key: 'name', label: 'Name' },
        { key: 'type', label: 'Type' },
        { key: 'status', label: 'Status' },
        { key: 'created_at', label: 'Created' },
        { key: 'actions', label: 'Actions' }
    ];

    // Define custom cell renderers
    const cellRenderers = {
        name: (value: string, row: Task) => ({
            html: `<a href="/tasks/${row.id}" class="text-indigo-600 hover:text-indigo-900">${value}</a>`
        }),
        status: (value: string) => ({
            html: `<span class="px-2 inline-flex text-xs leading-5 font-semibold rounded-full 
                ${value === 'completed' ? 'bg-green-100 text-green-800' :
                 value === 'running' ? 'bg-blue-100 text-blue-800' :
                 value === 'failed' ? 'bg-red-100 text-red-800' :
                 value === 'cancelled' ? 'bg-gray-100 text-gray-800' :
                 'bg-yellow-100 text-yellow-800'}">
                ${value}
            </span>`
        }),
        created_at: (value: string) => new Date(value).toLocaleString(),
        actions: (_: any, row: Task) => {
            if (row.status === 'pending' || row.status === 'running') {
                return {
                    html: `<button
                        data-task-id="${row.id}"
                        class="text-red-600 hover:text-red-900 cancel-task">
                        Cancel
                    </button>`
                };
            }
            return '';
        }
    };

    async function loadTasks() {
        try {
            loading = true;
            const response = await api.listTasks();
            tasks = response.tasks;
        } catch (e) {
            error = e instanceof Error ? e.message : 'Failed to load tasks';
        } finally {
            loading = false;
        }
    }

    async function createTask() {
        try {
            loading = true;
            formErrors = {};
            await api.createTask(newTask);
            showCreateForm = false;
            await loadTasks();
        } catch (e) {
            if (e instanceof Error) {
                error = e.message;
                // Parse validation errors if they exist
                try {
                    const errorData = JSON.parse(e.message);
                    if (errorData.errors) {
                        formErrors = errorData.errors;
                    }
                } catch {
                    // If it's not a JSON error, just use the message as is
                }
            } else {
                error = 'Failed to create task';
            }
        } finally {
            loading = false;
        }
    }

    async function cancelTask(id: string) {
        try {
            await api.cancelTask(id);
            await loadTasks();
        } catch (e) {
            error = e instanceof Error ? e.message : 'Failed to cancel task';
        }
    }

    // Set up event delegation for cancel buttons
    function handleTableClick(e: MouseEvent | KeyboardEvent) {
        const target = e.target as HTMLElement;
        if (target.classList.contains('cancel-task')) {
            const taskId = target.getAttribute('data-task-id');
            if (taskId) {
                cancelTask(taskId);
            }
        }
    }

    onMount(loadTasks);
</script>

<div class="space-y-6">
    <div class="flex justify-between items-center">
        <h1 class="text-2xl font-semibold text-gray-900">Tasks</h1>
        <button
            on:click={() => showCreateForm = !showCreateForm}
            class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
        >
            {showCreateForm ? 'Cancel' : 'Create Task'}
        </button>
    </div>

    {#if error}
        <div class="bg-red-50 border-l-4 border-red-400 p-4">
            <div class="flex">
                <div class="flex-shrink-0">
                    <svg class="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor">
                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
                    </svg>
                </div>
                <div class="ml-3">
                    <p class="text-sm text-red-700">{error}</p>
                </div>
            </div>
        </div>
    {/if}

    {#if showCreateForm}
        <div class="bg-white shadow rounded-lg p-6">
            <h2 class="text-lg font-medium text-gray-900 mb-4">Create New Task</h2>
            <TaskForm
                task={newTask}
                onSubmit={createTask}
                onCancel={() => showCreateForm = false}
                {loading}
                errors={formErrors}
            />
        </div>
    {/if}

    <div 
        on:click={handleTableClick}
        on:keydown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
                handleTableClick(e);
            }
        }}
        role="region"
        aria-label="Tasks table with actions"
    >
        <Table 
            title="All Tasks"
            {columns}
            rows={tasks}
            {cellRenderers}
            loading={loading && !showCreateForm}
        />
    </div>
</div> 