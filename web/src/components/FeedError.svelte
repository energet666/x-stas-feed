<script lang="ts">
	import { CircleAlert, RefreshCw } from "lucide-svelte";
	import { uiText as t } from '../lib/ui_text';

	let {
		message,
		onRetry,
	}: {
		message: string;
		onRetry: () => void;
	} = $props();
</script>

<div class="ui-panel flex items-start gap-3 p-4 text-danger">
	<CircleAlert
		class="mt-0.5 shrink-0"
		size={20}
	/>
	<div>
		<h2 class="font-semibold">{t.feed.couldNotLoad}</h2>
		<p class="text-sm opacity-85">{message}</p>
		<button
			class="ui-button mt-3 gap-2"
			type="button"
			onclick={onRetry}
		>
			<RefreshCw size={16} />
			{t.common.tryAgain}
		</button>
	</div>
</div>
