<script lang="ts">
    import { onMount } from 'svelte';
    import { mount } from 'svelte';
    import { api } from '$lib/api';
    import type { Task, CreateTaskRequest } from '$lib/types';
    import TaskForm from '$lib/components/TaskForm.svelte';
    import Table from '$lib/components/Table.svelte';
    import StatusBadge from '$lib/components/StatusBadge.svelte';

    let tasks: Task[] = [];
    let loading = true;
    let error: string | null = null;
    let dataFetched = false;
    let showCreateForm = false;
    let sortBy = 'created_at';
    let sortOrder: 'asc' | 'desc' = 'desc';
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
        status: (value: string) => {
            const statusHTML = document.createElement('div');
            // Use mount instead of new for Svelte 5
            mount(StatusBadge, {
                target: statusHTML,
                props: { status: value }
            });
            return { html: statusHTML.innerHTML };
        },
        created_at: (value: string) => value ? new Date(value).toLocaleString() : 'Unknown',
        actions: (_: any, row: Task) => {
            if (row.status === 'pending' || row.status === 'running' || row.status === 'retrying' || row.status === 'scheduled') {
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
            const response = await api.listTasks(undefined, 10, 0, sortBy, sortOrder);
            console.log('Tasks loaded:', response);
            tasks = response.tasks || [];
            dataFetched = true;
        } catch (e) {
            console.error('Error loading tasks:', e);
            error = e instanceof Error ? e.message : 'Failed to load tasks';
            dataFetched = true;
        } finally {
            loading = false;
        }
    }

    function changeSort(field: string) {
        if (sortBy === field) {
            // Toggle order if same field
            sortOrder = sortOrder === 'asc' ? 'desc' : 'asc';
        } else {
            // New field, default to descending
            sortBy = field;
            sortOrder = 'desc';
        }
        loadTasks();
    }

    async function createTask() {
        try {
            loading = true;
            formErrors = {};
            const response = await api.createTask(newTask);
            console.log('Task created:', response);
            showCreateForm = false;
            
            // Reset the form for next use
            newTask = {
                type: '',
                name: '',
                description: '',
                priority: 'medium',
                payload: {},
                max_retries: 3,
                timeout: 60,
                schedule: {
                    type: 'one-time',
                    start_time: new Date().toISOString().slice(0, 16) 
                }
            };
            
            // Reload tasks
            await loadTasks();
        } catch (e) {
            console.error('Error creating task:', e);
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
        <div class="flex space-x-2">
            <button
                on:click={loadTasks}
                class="inline-flex items-center px-3 py-2 border border-gray-300 text-sm font-medium rounded-md shadow-sm text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
            >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
                Refresh
            </button>
            <button
                on:click={() => showCreateForm = !showCreateForm}
                class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
            >
                {showCreateForm ? 'Cancel' : 'Create Task'}
            </button>
        </div>
    </div>

    <div class="flex items-center space-x-4 mb-4">
        <div class="text-sm font-medium text-gray-500">Sort by:</div>
        <div class="flex space-x-2">
            {#each [
                { field: 'created_at', label: 'Created Date' },
                { field: 'updated_at', label: 'Updated Date' },
                { field: 'name', label: 'Name' },
                { field: 'status', label: 'Status' }
            ] as sortOption}
                <button 
                    class="px-3 py-1 text-sm border rounded-md {sortBy === sortOption.field ? 'bg-indigo-50 border-indigo-200 text-indigo-700 font-medium' : 'border-gray-200 text-gray-600'}"
                    on:click={() => changeSort(sortOption.field)}
                >
                    {sortOption.label}
                    {#if sortBy === sortOption.field}
                        <span class="ml-1">{sortOrder === 'asc' ? '↑' : '↓'}</span>
                    {/if}
                </button>
            {/each}
        </div>
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
            loading={loading && (!dataFetched || showCreateForm)}
        />
    </div>
</div> 