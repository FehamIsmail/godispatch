<script lang="ts">
    import { onMount } from 'svelte';
    import { page } from '$app/stores';
    import { api } from '$lib/api';
    import type { Task, TaskResult, TaskHistory } from '$lib/types';
    import StatusBadge from '$lib/components/StatusBadge.svelte';
    import StatusIcon from '$lib/components/StatusIcon.svelte';

    let task: Task | null = null;
    let result: TaskResult | null = null;
    let history: TaskHistory[] = [];
    let loading = true;
    let error: string | null = null;
    let dataFetched = false;

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
            dataFetched = true;
        } catch (e) {
            error = e instanceof Error ? e.message : 'Failed to load task data';
            dataFetched = true;
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
        {#if task && (task.status === 'pending' || task.status === 'running' || task.status === 'retrying' || task.status === 'scheduled')}
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

    {#if loading && !dataFetched}
        <div class="flex justify-center">
            <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900"></div>
        </div>
    {:else if task}
        <div class="bg-white shadow rounded-lg">
            <div class="px-4 py-5 sm:px-6 border-b border-gray-200">
                <h3 class="text-lg leading-6 font-medium text-gray-900">Task Information</h3>
                <p class="mt-1 text-sm text-gray-500">Details about this task</p>
            </div>
            <div class="px-4 py-5 sm:px-6">
                <dl class="grid grid-cols-1 gap-x-4 gap-y-6 sm:grid-cols-2">
                    <div class="sm:col-span-1">
                        <dt class="text-sm font-medium text-gray-500">Name</dt>
                        <dd class="mt-1 text-sm text-gray-900">{task.name}</dd>
                    </div>
                    <div class="sm:col-span-1">
                        <dt class="text-sm font-medium text-gray-500">Type</dt>
                        <dd class="mt-1 text-sm text-gray-900">{task.type}</dd>
                    </div>
                    <div class="sm:col-span-2">
                        <dt class="text-sm font-medium text-gray-500">Description</dt>
                        <dd class="mt-1 text-sm text-gray-900">{task.description || 'No description provided'}</dd>
                    </div>
                    <div class="sm:col-span-1">
                        <dt class="text-sm font-medium text-gray-500">Status</dt>
                        <dd class="mt-1 text-sm text-gray-900">
                            <StatusBadge status={task.status} />
                        </dd>
                    </div>
                    <div class="sm:col-span-1">
                        <dt class="text-sm font-medium text-gray-500">Priority</dt>
                        <dd class="mt-1 text-sm text-gray-900">
                            <span class="px-2 py-1 inline-flex text-xs leading-5 font-semibold rounded-full
                                {task.priority === 'high' ? 'bg-red-100 text-red-800' :
                                 task.priority === 'medium' ? 'bg-yellow-100 text-yellow-800' :
                                 'bg-green-100 text-green-800'}">
                                {task.priority}
                            </span>
                        </dd>
                    </div>
                    <div class="sm:col-span-1">
                        <dt class="text-sm font-medium text-gray-500">Created</dt>
                        <dd class="mt-1 text-sm text-gray-900">
                            {new Date(task.created_at).toLocaleString()}
                        </dd>
                    </div>
                    <div class="sm:col-span-1">
                        <dt class="text-sm font-medium text-gray-500">Updated</dt>
                        <dd class="mt-1 text-sm text-gray-900">
                            {new Date(task.updated_at).toLocaleString()}
                        </dd>
                    </div>
                </dl>
            </div>
        </div>

        {#if result}
            <div class="bg-white shadow rounded-lg">
                <div class="px-4 py-5 sm:px-6 border-b border-gray-200">
                    <h3 class="text-lg leading-6 font-medium text-gray-900">Task Result</h3>
                    <p class="mt-1 text-sm text-gray-500">Execution results and output data</p>
                </div>
                <div class="px-4 py-5 sm:px-6">
                    <dl class="grid grid-cols-1 gap-x-4 gap-y-6 sm:grid-cols-2">
                        <div class="sm:col-span-1">
                            <dt class="text-sm font-medium text-gray-500">Status</dt>
                            <dd class="mt-1 text-sm text-gray-900">
                                <StatusBadge status={result.status} />
                            </dd>
                        </div>
                        <div class="sm:col-span-1">
                            <dt class="text-sm font-medium text-gray-500">Started</dt>
                            <dd class="mt-1 text-sm text-gray-900">
                                {#if result.started_at}
                                    {new Date(result.started_at).toLocaleString()}
                                {:else}
                                    <span class="text-gray-400">Not started</span>
                                {/if}
                            </dd>
                        </div>
                        <div class="sm:col-span-1">
                            <dt class="text-sm font-medium text-gray-500">Completed</dt>
                            <dd class="mt-1 text-sm text-gray-900">
                                {#if result.completed_at && result.status !== 'running' && result.status !== 'pending'}
                                    {new Date(result.completed_at).toLocaleString()}
                                {:else}
                                    <span class="text-gray-400">Not completed</span>
                                {/if}
                            </dd>
                        </div>
                        
                        {#if result.error}
                            <div class="sm:col-span-2">
                                <dt class="text-sm font-medium text-gray-500">Error</dt>
                                <dd class="mt-1 text-sm text-red-600 bg-red-50 p-3 rounded-md border border-red-100">
                                    {result.error}
                                </dd>
                            </div>
                        {/if}
                        
                        <div class="sm:col-span-2">
                            <dt class="text-sm font-medium text-gray-500">Output</dt>
                            <dd class="mt-1 text-sm text-gray-900">
                                <pre class="bg-gray-50 p-4 rounded-md overflow-x-auto border border-gray-100 max-h-64 overflow-y-auto">
                                    {#if result.output}
                                        {#if typeof result.output === 'object'}
                                            {JSON.stringify(result.output, null, 2)}
                                        {:else if typeof result.output === 'string'}
                                            {result.output}
                                        {:else}
                                            {String(result.output)}
                                        {/if}
                                    {:else}
                                        <span class="text-gray-400">No output</span>
                                    {/if}
                                </pre>
                            </dd>
                        </div>
                    </dl>
                </div>
            </div>
        {/if}

        <div class="bg-white shadow rounded-lg">
            <div class="px-4 py-5 sm:px-6 border-b border-gray-200">
                <h3 class="text-lg leading-6 font-medium text-gray-900">Task History</h3>
                <p class="mt-1 text-sm text-gray-500">Timeline of task execution events</p>
            </div>
            <div class="py-4 px-2 sm:px-6">
                <div class="flow-root">
                    {#if history.length === 0}
                        <div class="py-10 text-center text-gray-500">
                            <p>No history available for this task</p>
                        </div>
                    {:else}
                        <ul class="-mb-8">
                            {#each history as event, i}
                                <li>
                                    <div class="relative pb-8">
                                        {#if i !== history.length - 1}
                                            <span class="absolute top-5 left-5 -ml-px h-full w-0.5 bg-gray-200" aria-hidden="true"></span>
                                        {/if}
                                        <div class="relative flex items-start space-x-4">
                                            <div class="flex-shrink-0">
                                                <StatusIcon status={event.status}>
                                                    {#if event.status === 'completed'}
                                                    <svg class="h-6 w-6 text-white" viewBox="0 0 20 20" fill="currentColor">
                                                        <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                                                    </svg>
                                                    {:else if event.status === 'running'}
                                                    <svg class="h-6 w-6 text-white" viewBox="0 0 20 20" fill="currentColor">
                                                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z" clip-rule="evenodd" />
                                                    </svg>
                                                    {:else if event.status === 'failed' || event.status === 'timeout'}
                                                    <svg class="h-6 w-6 text-white" viewBox="0 0 20 20" fill="currentColor">
                                                        <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" />
                                                    </svg>
                                                    {:else if event.status === 'cancelled'}
                                                    <svg class="h-6 w-6 text-white" viewBox="0 0 20 20" fill="currentColor">
                                                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
                                                    </svg>
                                                    {:else if event.status === 'retrying'}
                                                    <svg class="h-6 w-6 text-white" viewBox="0 0 20 20" fill="currentColor">
                                                        <path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clip-rule="evenodd" />
                                                    </svg>
                                                    {:else}
                                                    <svg class="h-6 w-6 text-white" viewBox="0 0 20 20" fill="currentColor">
                                                        <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-8-3a1 1 0 00-.867.5 1 1 0 11-1.731-1A3 3 0 0113 8a3.001 3.001 0 01-2 2.83V11a1 1 0 11-2 0v-1a1 1 0 011-1 1 1 0 100-2zm0 8a1 1 0 100-2 1 1 0 000 2z" clip-rule="evenodd" />
                                                    </svg>
                                                    {/if}
                                                </StatusIcon>
                                            </div>
                                            <div class="min-w-0 flex-1 py-1.5">
                                                <div class="bg-gray-50 rounded-lg p-3 shadow-sm border border-gray-100">
                                                    <div class="flex justify-between items-center mb-1">
                                                        <span class="text-sm font-medium text-gray-900">
                                                            {event.status.charAt(0).toUpperCase() + event.status.slice(1)}
                                                        </span>
                                                        <time class="text-xs text-gray-500 whitespace-nowrap" datetime={event.timestamp}>
                                                            {#if event.timestamp}
                                                                {new Date(event.timestamp).toLocaleString()}
                                                            {:else}
                                                                Unknown time
                                                            {/if}
                                                        </time>
                                                    </div>
                                                    <p class="text-sm text-gray-600">{event.message}</p>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                </li>
                            {/each}
                        </ul>
                    {/if}
                </div>
            </div>
        </div>
    {/if}
</div> 