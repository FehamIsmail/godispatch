<script lang="ts">
    import { onMount } from 'svelte';
    import { page } from '$app/stores';
    import { api } from '$lib/api';
    import type { Task, TaskResult, TaskHistory } from '$lib/types';

    let task: Task | null = null;
    let result: TaskResult | null = null;
    let history: TaskHistory[] = [];
    let loading = true;
    let error: string | null = null;

    async function loadTaskData() {
        try {
            loading = true;
            const taskId = $page.params.id;
            const [taskResponse, resultResponse, historyResponse] = await Promise.all([
                api.getTask(taskId),
                api.getTaskResult(taskId),
                api.getTaskHistory(taskId)
            ]);
            task = taskResponse;
            result = resultResponse;
            history = historyResponse.history;
        } catch (e) {
            error = e instanceof Error ? e.message : 'Failed to load task data';
        } finally {
            loading = false;
        }
    }

    async function cancelTask() {
        if (!task) return;
        try {
            await api.cancelTask(task.id);
            await loadTaskData();
        } catch (e) {
            error = e instanceof Error ? e.message : 'Failed to cancel task';
        }
    }

    onMount(loadTaskData);
</script>

<div class="space-y-6">
    <div class="flex justify-between items-center">
        <h1 class="text-2xl font-semibold text-gray-900">Task Details</h1>
        {#if task && (task.status === 'pending' || task.status === 'running')}
            <button
                on:click={cancelTask}
                class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-red-600 hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500"
            >
                Cancel Task
            </button>
        {/if}
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

    {#if loading}
        <div class="flex justify-center">
            <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900"></div>
        </div>
    {:else if task}
        <div class="bg-white shadow rounded-lg">
            <div class="px-4 py-5 sm:px-6">
                <h3 class="text-lg leading-6 font-medium text-gray-900">Task Information</h3>
            </div>
            <div class="border-t border-gray-200">
                <dl class="divide-y divide-gray-200">
                    <div class="px-4 py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
                        <dt class="text-sm font-medium text-gray-500">Name</dt>
                        <dd class="mt-1 text-sm text-gray-900 sm:mt-0 sm:col-span-2">{task.name}</dd>
                    </div>
                    <div class="px-4 py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
                        <dt class="text-sm font-medium text-gray-500">Type</dt>
                        <dd class="mt-1 text-sm text-gray-900 sm:mt-0 sm:col-span-2">{task.type}</dd>
                    </div>
                    <div class="px-4 py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
                        <dt class="text-sm font-medium text-gray-500">Description</dt>
                        <dd class="mt-1 text-sm text-gray-900 sm:mt-0 sm:col-span-2">{task.description}</dd>
                    </div>
                    <div class="px-4 py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
                        <dt class="text-sm font-medium text-gray-500">Status</dt>
                        <dd class="mt-1 text-sm text-gray-900 sm:mt-0 sm:col-span-2">
                            <span class="px-2 inline-flex text-xs leading-5 font-semibold rounded-full 
                                {task.status === 'completed' ? 'bg-green-100 text-green-800' :
                                 task.status === 'running' ? 'bg-blue-100 text-blue-800' :
                                 task.status === 'failed' ? 'bg-red-100 text-red-800' :
                                 task.status === 'cancelled' ? 'bg-gray-100 text-gray-800' :
                                 'bg-yellow-100 text-yellow-800'}">
                                {task.status}
                            </span>
                        </dd>
                    </div>
                    <div class="px-4 py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
                        <dt class="text-sm font-medium text-gray-500">Priority</dt>
                        <dd class="mt-1 text-sm text-gray-900 sm:mt-0 sm:col-span-2">{task.priority}</dd>
                    </div>
                    <div class="px-4 py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
                        <dt class="text-sm font-medium text-gray-500">Created</dt>
                        <dd class="mt-1 text-sm text-gray-900 sm:mt-0 sm:col-span-2">
                            {new Date(task.created_at).toLocaleString()}
                        </dd>
                    </div>
                    <div class="px-4 py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
                        <dt class="text-sm font-medium text-gray-500">Updated</dt>
                        <dd class="mt-1 text-sm text-gray-900 sm:mt-0 sm:col-span-2">
                            {new Date(task.updated_at).toLocaleString()}
                        </dd>
                    </div>
                </dl>
            </div>
        </div>

        {#if result}
            <div class="bg-white shadow rounded-lg">
                <div class="px-4 py-5 sm:px-6">
                    <h3 class="text-lg leading-6 font-medium text-gray-900">Task Result</h3>
                </div>
                <div class="border-t border-gray-200">
                    <dl class="divide-y divide-gray-200">
                        <div class="px-4 py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
                            <dt class="text-sm font-medium text-gray-500">Status</dt>
                            <dd class="mt-1 text-sm text-gray-900 sm:mt-0 sm:col-span-2">{result.status}</dd>
                        </div>
                        <div class="px-4 py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
                            <dt class="text-sm font-medium text-gray-500">Started</dt>
                            <dd class="mt-1 text-sm text-gray-900 sm:mt-0 sm:col-span-2">
                                {new Date(result.started_at).toLocaleString()}
                            </dd>
                        </div>
                        <div class="px-4 py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
                            <dt class="text-sm font-medium text-gray-500">Completed</dt>
                            <dd class="mt-1 text-sm text-gray-900 sm:mt-0 sm:col-span-2">
                                {new Date(result.completed_at).toLocaleString()}
                            </dd>
                        </div>
                        {#if result.error}
                            <div class="px-4 py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
                                <dt class="text-sm font-medium text-gray-500">Error</dt>
                                <dd class="mt-1 text-sm text-red-900 sm:mt-0 sm:col-span-2">{result.error}</dd>
                            </div>
                        {/if}
                        <div class="px-4 py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
                            <dt class="text-sm font-medium text-gray-500">Output</dt>
                            <dd class="mt-1 text-sm text-gray-900 sm:mt-0 sm:col-span-2">
                                <pre class="bg-gray-50 p-4 rounded-md overflow-x-auto">
                                    {JSON.stringify(result.output, null, 2)}
                                </pre>
                            </dd>
                        </div>
                    </dl>
                </div>
            </div>
        {/if}

        <div class="bg-white shadow rounded-lg">
            <div class="px-4 py-5 sm:px-6">
                <h3 class="text-lg leading-6 font-medium text-gray-900">Task History</h3>
            </div>
            <div class="border-t border-gray-200">
                <div class="flow-root">
                    <ul class="-mb-8">
                        {#each history as event, i}
                            <li>
                                <div class="relative pb-8">
                                    {#if i !== history.length - 1}
                                        <span class="absolute top-4 left-4 -ml-px h-full w-0.5 bg-gray-200" aria-hidden="true"></span>
                                    {/if}
                                    <div class="relative flex space-x-3">
                                        <div>
                                            <span class="h-8 w-8 rounded-full flex items-center justify-center ring-8 ring-white
                                                {event.status === 'completed' ? 'bg-green-500' :
                                                 event.status === 'running' ? 'bg-blue-500' :
                                                 event.status === 'failed' ? 'bg-red-500' :
                                                 event.status === 'cancelled' ? 'bg-gray-500' :
                                                 'bg-yellow-500'}">
                                                <svg class="h-5 w-5 text-white" viewBox="0 0 20 20" fill="currentColor">
                                                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                                                </svg>
                                            </span>
                                        </div>
                                        <div class="min-w-0 flex-1 pt-1.5 flex justify-between space-x-4">
                                            <div>
                                                <p class="text-sm text-gray-500">{event.message}</p>
                                            </div>
                                            <div class="text-right text-sm whitespace-nowrap text-gray-500">
                                                <time datetime={event.timestamp}>
                                                    {new Date(event.timestamp).toLocaleString()}
                                                </time>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </li>
                        {/each}
                    </ul>
                </div>
            </div>
        </div>
    {/if}
</div> 