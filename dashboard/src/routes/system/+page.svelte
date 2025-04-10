<script lang="ts">
    import { onMount } from 'svelte';
    import { api } from '$lib/api';
    import type { SystemMetrics, Worker } from '$lib/types';
    import Table from '$lib/components/Table.svelte';

    let metrics: SystemMetrics | null = null;
    let workers: Worker[] = [];
    let loading = true;
    let error: string | null = null;

    // Define table columns for workers
    const workerColumns = [
        { key: 'id', label: 'ID' },
        { key: 'status', label: 'Status' },
        { key: 'current_task', label: 'Current Task' },
        { key: 'last_heartbeat', label: 'Last Heartbeat' }
    ];

    // Define custom cell renderers
    const workerCellRenderers = {
        status: (value: string) => ({
            html: `<span class="px-2 inline-flex text-xs leading-5 font-semibold rounded-full 
                ${value === 'active' ? 'bg-green-100 text-green-800' :
                 value === 'idle' ? 'bg-gray-100 text-gray-800' :
                 'bg-yellow-100 text-yellow-800'}">
                ${value}
            </span>`
        }),
        current_task: (value: string | null) => {
            if (value) {
                return {
                    html: `<a href="/tasks/${value}" class="text-indigo-600 hover:text-indigo-900">${value}</a>`
                };
            }
            return '-';
        },
        last_heartbeat: (value: string) => new Date(value).toLocaleString()
    };

    async function loadSystemData() {
        try {
            loading = true;
            const [metricsResponse, workersResponse] = await Promise.all([
                api.getSystemMetrics(),
                api.getWorkers()
            ]);
            metrics = metricsResponse;
            workers = workersResponse.workers;
        } catch (e) {
            error = e instanceof Error ? e.message : 'Failed to load system data';
        } finally {
            loading = false;
        }
    }

    onMount(() => {
        loadSystemData();
        const interval = setInterval(loadSystemData, 5000);
        return () => clearInterval(interval);
    });
</script>

<div class="space-y-6">
    <h1 class="text-2xl font-semibold text-gray-900">System Overview</h1>

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

    {#if loading && !metrics}
        <div class="flex justify-center">
            <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900"></div>
        </div>
    {:else if metrics}
        <div class="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
            <div class="bg-white overflow-hidden shadow rounded-lg">
                <div class="p-5">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <svg class="h-6 w-6 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                            </svg>
                        </div>
                        <div class="ml-5 w-0 flex-1">
                            <dl>
                                <dt class="text-sm font-medium text-gray-500 truncate">Total Tasks</dt>
                                <dd class="text-lg font-semibold text-gray-900">{metrics.total_tasks}</dd>
                            </dl>
                        </div>
                    </div>
                </div>
            </div>

            <div class="bg-white overflow-hidden shadow rounded-lg">
                <div class="p-5">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <svg class="h-6 w-6 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                            </svg>
                        </div>
                        <div class="ml-5 w-0 flex-1">
                            <dl>
                                <dt class="text-sm font-medium text-gray-500 truncate">Active Tasks</dt>
                                <dd class="text-lg font-semibold text-gray-900">{metrics.active_tasks}</dd>
                            </dl>
                        </div>
                    </div>
                </div>
            </div>

            <div class="bg-white overflow-hidden shadow rounded-lg">
                <div class="p-5">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <svg class="h-6 w-6 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                            </svg>
                        </div>
                        <div class="ml-5 w-0 flex-1">
                            <dl>
                                <dt class="text-sm font-medium text-gray-500 truncate">Completed Tasks</dt>
                                <dd class="text-lg font-semibold text-gray-900">{metrics.completed_tasks}</dd>
                            </dl>
                        </div>
                    </div>
                </div>
            </div>

            <div class="bg-white overflow-hidden shadow rounded-lg">
                <div class="p-5">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <svg class="h-6 w-6 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                            </svg>
                        </div>
                        <div class="ml-5 w-0 flex-1">
                            <dl>
                                <dt class="text-sm font-medium text-gray-500 truncate">Avg. Execution Time</dt>
                                <dd class="text-lg font-semibold text-gray-900">{metrics.average_execution_time.toFixed(2)}s</dd>
                            </dl>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <Table
            title="Workers"
            columns={workerColumns}
            rows={workers}
            cellRenderers={workerCellRenderers}
            loading={loading}
        />
    {/if}
</div> 