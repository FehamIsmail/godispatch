<script lang="ts">
    import FormInput from './FormInput.svelte';
    import FormSelect from './FormSelect.svelte';
    import FormTextarea from './FormTextarea.svelte';
    import type { CreateTaskRequest } from '$lib/types';

    export let task: CreateTaskRequest;
    export let onSubmit: () => void;
    export let onCancel: () => void;
    export let loading = false;
    export let errors: Record<string, string> = {};

    const taskTypeOptions = [
        { value: 'http-request', label: 'HTTP Request' },
        { value: 'email', label: 'Email' },
        { value: 'data-processing', label: 'Data Processing' },
        { value: 'backup', label: 'Backup' },
        { value: 'cleanup', label: 'Cleanup' },
        { value: 'notification', label: 'Notification' },
        { value: 'custom', label: 'Custom' }
    ];

    const priorityOptions = [
        { value: 'low', label: 'Low' },
        { value: 'medium', label: 'Medium' },
        { value: 'high', label: 'High' }
    ];

    const scheduleTypeOptions = [
        { value: 'one-time', label: 'One-time' },
        { value: 'recurring', label: 'Recurring' },
        { value: 'cron', label: 'Cron' }
    ];

    let startTime = task.schedule.start_time || '';

    $: task.schedule.start_time = startTime;
</script>

<form on:submit|preventDefault={onSubmit} class="space-y-6">
    <div class="grid grid-cols-1 gap-6 sm:grid-cols-2">
        <FormSelect
            label="Type"
            bind:value={task.type}
            options={taskTypeOptions}
            required
            error={errors.type}
        />
        <FormInput
            label="Name"
            bind:value={task.name}
            required
            min={undefined}
            error={errors.name}
        />
    </div>

    <FormTextarea
        label="Description"
        bind:value={task.description}
        error={errors.description}
    />

    <div class="grid grid-cols-1 gap-6 sm:grid-cols-3">
        <FormSelect
            label="Priority"
            bind:value={task.priority}
            options={priorityOptions}
            error={errors.priority}
        />
        <FormInput
            label="Max Retries"
            type="number"
            bind:value={task.max_retries}
            min={0}
            error={errors.max_retries}
        />
        <FormInput
            label="Timeout (seconds)"
            type="number"
            bind:value={task.timeout}
            min={1}
            error={errors.timeout}
        />
    </div>

    <div class="grid grid-cols-1 gap-6 sm:grid-cols-2">
        <FormSelect
            label="Schedule Type"
            bind:value={task.schedule.type}
            options={scheduleTypeOptions}
            required
            error={errors['schedule.type']}
        />
        <FormInput
            label="Start Time"
            type="datetime-local"
            bind:value={startTime}
            min={undefined}
            error={errors['schedule.start_time']}
        />
    </div>

    <div class="flex justify-end space-x-3 pt-4">
        <button
            type="button"
            on:click={onCancel}
            class="inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
        >
            Cancel
        </button>
        <button
            type="submit"
            disabled={loading}
            class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:opacity-50"
        >
            {loading ? 'Creating...' : 'Create Task'}
        </button>
    </div>
</form> 