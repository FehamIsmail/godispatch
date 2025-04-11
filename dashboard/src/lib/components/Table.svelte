<script lang="ts">
    export let title: string = '';
    export let columns: {
        key: string;
        label: string;
        class?: string;
    }[] = [];
    export let rows: any[] | null = [];
    export let loading: boolean = false;

    // Allow passing in custom cell renderers
    export let cellRenderers: Record<string, (value: any, row: any) => string | { html: string } | null> = {};

    // Ensure rows is never null for rendering
    $: safeRows = rows || [];

    // Helper function to determine if we're on the last row
    function isLastRow(index: number): boolean {
        return index === safeRows.length - 1;
    }

    // Helper function to determine if we're on the first column
    function isFirstColumn(index: number): boolean {
        return index === 0;
    }

    // Helper function to determine if we're on the last column
    function isLastColumn(index: number): boolean {
        return index === columns.length - 1;
    }
</script>

<div class="bg-white shadow rounded-lg overflow-hidden">
    {#if title}
        <div class="px-4 py-5 sm:px-6">
            <h3 class="text-lg leading-6 font-medium text-gray-900">{title}</h3>
        </div>
    {/if}
    <div class="border-t border-gray-200">
        {#if loading}
            <div class="flex justify-center py-6">
                <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900"></div>
            </div>
        {:else if safeRows.length === 0}
            <div class="px-6 py-4 text-sm text-gray-500 text-center">
                No data available
            </div>
        {:else}
            <div class="overflow-hidden">
                <table class="min-w-full divide-y divide-gray-200">
                    <thead class="bg-gray-50">
                        <tr>
                            {#each columns as column}
                                <th 
                                    scope="col" 
                                    class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider {column.class || ''}"
                                >
                                    {column.label}
                                </th>
                            {/each}
                        </tr>
                    </thead>
                    <tbody class="bg-white divide-y divide-gray-200">
                        {#each safeRows as row, rowIndex}
                            <tr>
                                {#each columns as column, colIndex}
                                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 {column.class || ''} 
                                        {isLastRow(rowIndex) && isFirstColumn(colIndex) ? 'rounded-bl-lg' : ''} 
                                        {isLastRow(rowIndex) && isLastColumn(colIndex) ? 'rounded-br-lg' : ''}"
                                    >
                                        {#if cellRenderers[column.key] && row[column.key] !== undefined}
                                            {@const rendered = cellRenderers[column.key](row[column.key], row)}
                                            {#if rendered && typeof rendered === 'object' && 'html' in rendered}
                                                {@html rendered.html}
                                            {:else}
                                                {rendered}
                                            {/if}
                                        {:else}
                                            {row[column.key] !== undefined ? row[column.key] : '-'}
                                        {/if}
                                    </td>
                                {/each}
                            </tr>
                        {/each}
                    </tbody>
                </table>
            </div>
        {/if}
    </div>
</div> 