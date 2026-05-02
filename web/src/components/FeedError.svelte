<script lang="ts">
	import { CircleAlert, RefreshCw } from "lucide-svelte";

	let {
		message,
		onRetry,
	}: {
		message: string;
		onRetry: () => void;
	} = $props();
</script>

<div class="glass-panel flex items-start gap-3 p-4 text-danger">
	<CircleAlert
		class="mt-0.5 shrink-0"
		size={20}
	/>
	<div>
		<h2 class="font-semibold">Could not load the feed</h2>
		<p class="text-sm opacity-85">{message}</p>
		<button
			class="glass-button mt-3 gap-2"
			type="button"
			onclick={onRetry}
		>
			<RefreshCw size={16} />
			Try again
		</button>
	</div>
</div>
