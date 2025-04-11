<script lang="ts">
    export let status: string;
    
    // Define color mappings for different statuses
    const getStatusColors = (status: string): string => {
        switch (status.toLowerCase()) {
            case 'completed':
                return 'bg-green-100 text-green-800';
            case 'running':
                return 'bg-blue-100 text-blue-800';
            case 'failed':
                return 'bg-red-100 text-red-800';
            case 'cancelled':
                return 'bg-gray-100 text-gray-800';
            case 'timeout':
                return 'bg-orange-100 text-orange-800';
            case 'retrying':
                return 'bg-purple-100 text-purple-800';
            case 'scheduled':
                return 'bg-indigo-100 text-indigo-800';
            case 'pending':
                return 'bg-yellow-100 text-yellow-800';
            default:
                return 'bg-yellow-100 text-yellow-800';
        }
    };

    // Format status for display (capitalize first letter)
    const formatStatus = (status: string): string => {
        return status.charAt(0).toUpperCase() + status.slice(1).toLowerCase();
    };

    const statusClass = getStatusColors(status);
    const displayText = formatStatus(status);
</script>

<span class="px-2 py-1 inline-flex text-xs leading-5 font-semibold rounded-full {statusClass}">
    {displayText}
</span> 