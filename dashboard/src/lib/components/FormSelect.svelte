<script lang="ts">
    export let label: string;
    export let value: string;
    export let options: { value: string; label: string }[];
    export let required = false;
    export let error: string | null = null;
</script>

<div class="mb-4">
    <label for={label.toLowerCase()} class="block text-sm font-medium text-gray-700 mb-1">
        {label}
    </label>
    <div class="relative">
        <select
            id={label.toLowerCase()}
            bind:value
            {required}
            class="block w-full px-2 py-2 pr-8 rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm appearance-none"
            class:border-red-500={error}
        >
            {#each options as option}
                <option value={option.value}>{option.label}</option>
            {/each}
        </select>
        <div class="pointer-events-none absolute inset-y-0 right-0 flex items-center px-2 text-gray-700">
            <svg class="h-4 w-4 fill-current" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20">
                <path d="M9.293 12.95l.707.707L15.657 8l-1.414-1.414L10 10.828 5.757 6.586 4.343 8z" />
            </svg>
        </div>
    </div>
    {#if error}
        <p class="mt-1 text-sm text-red-600">{error}</p>
    {/if}
</div> 