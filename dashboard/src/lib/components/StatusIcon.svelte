<script lang="ts">
    export let status: string;
    
    // Background colors for timeline status icons
    const getStatusColor = (status: string): string => {
        switch (status.toLowerCase()) {
            case 'completed':
                return 'bg-green-500';
            case 'running':
                return 'bg-blue-500';
            case 'failed':
                return 'bg-red-500';
            case 'cancelled':
                return 'bg-gray-500';
            case 'timeout':
                return 'bg-orange-500';
            case 'retrying':
                return 'bg-purple-500';
            case 'scheduled':
                return 'bg-indigo-500';
            case 'pending':
                return 'bg-yellow-500';
            default:
                return 'bg-yellow-500';
        }
    };

    const statusClass = getStatusColor(status);
</script>

<span class="h-10 w-10 rounded-full flex items-center justify-center ring-8 ring-white {statusClass}">
    <slot>
        <!-- Default icon if no icon is provided -->
        <svg class="h-6 w-6 text-white" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-8-3a1 1 0 00-.867.5 1 1 0 11-1.731-1A3 3 0 0113 8a3.001 3.001 0 01-2 2.83V11a1 1 0 11-2 0v-1a1 1 0 011-1 1 1 0 100-2zm0 8a1 1 0 100-2 1 1 0 000 2z" clip-rule="evenodd" />
        </svg>
    </slot>
</span> 