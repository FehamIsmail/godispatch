<script lang="ts">
    export let label: string;
    export let type = 'text';
    export let value: string | number;
    export let required = false;
    export let min: number | undefined;
    export let placeholder = '';
    export let error: string | null = null;
</script>

<div class="mb-4">
    <label for={label.toLowerCase()} class="block text-sm font-medium text-gray-700 mb-1">
        {label}
    </label>
    <input
        type={type}
        id={label.toLowerCase()}
        bind:value
        {required}
        {min}
        {placeholder}
        class="block w-full px-2 py-2 rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
        class:border-red-500={error}
    />
    {#if error}
        <p class="mt-1 text-sm text-red-600">{error}</p>
    {/if}
</div> 