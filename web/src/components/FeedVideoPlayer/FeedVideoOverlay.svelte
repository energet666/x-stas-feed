<script lang="ts">
  import { Play } from 'lucide-svelte';

  let {
    paused,
    playBlocked,
    seekFeedbackSide,
    seekFeedbackAmount,
    seekFeedbackTick,
    showSpeedIndicator,
    userPlaybackRate,
    onTogglePlay
  }: {
    paused: boolean;
    playBlocked: boolean;
    seekFeedbackSide: 'left' | 'right' | null;
    seekFeedbackAmount: number;
    seekFeedbackTick: number;
    showSpeedIndicator: boolean;
    userPlaybackRate: number;
    onTogglePlay: () => void;
  } = $props();
</script>

{#if seekFeedbackSide}
  {#key `${seekFeedbackSide}-${seekFeedbackTick}`}
    <div
      class="video-seek-feedback absolute flex items-center"
      class:video-seek-feedback-left={seekFeedbackSide === 'left'}
      class:video-seek-feedback-right={seekFeedbackSide === 'right'}
    >
      <span>{seekFeedbackSide === 'left' ? `-${seekFeedbackAmount}s` : `+${seekFeedbackAmount}s`}</span>
    </div>
  {/key}
{/if}

{#if showSpeedIndicator}
  <div class="video-speed-indicator absolute right-4 rounded-full text-center font-extrabold">{userPlaybackRate}x</div>
{/if}

{#if paused}
  <button class="video-play-overlay absolute grid place-items-center rounded-full" type="button" aria-label="Play video" onclick={onTogglePlay}>
    <Play size={26} fill="currentColor" />
  </button>
{/if}

{#if playBlocked}
  <div class="video-play-message absolute rounded-full text-xs font-bold">Tap play to start video</div>
{/if}

<style>
  .video-speed-indicator {
    top: 4.8rem;
    z-index: 6;
    min-width: 3.1rem;
    padding: 0.45rem 0.7rem;
    border: 1px solid var(--color-glass-border);
    background: var(--background-image-glass);
    box-shadow: var(--shadow-popover);
    color: var(--color-secondary);
    font-size: 0.8rem;
    backdrop-filter: blur(24px) saturate(170%);
    -webkit-backdrop-filter: blur(24px) saturate(170%);
  }

  .video-seek-feedback {
    inset-block: 0;
    z-index: 6;
    width: 50%;
    pointer-events: none;
  }

  .video-seek-feedback-left {
    left: 0;
    justify-content: flex-start;
  }

  .video-seek-feedback-right {
    right: 0;
    justify-content: flex-end;
  }

  .video-seek-feedback span {
    border-radius: 999px;
    font-weight: 800;
    text-align: center;
    margin: 0 2rem;
    min-width: 4.1rem;
    padding: 0.7rem 0.9rem;
    border: 1px solid var(--color-glass-border);
    background: var(--background-image-glass-strong);
    box-shadow: var(--shadow-seek);
    color: var(--color-primary);
    font-size: 0.92rem;
    animation: video-seek-feedback-pop 420ms ease-out both;
    backdrop-filter: blur(22px) saturate(170%);
    -webkit-backdrop-filter: blur(22px) saturate(170%);
  }

  @keyframes video-seek-feedback-pop {
    0% {
      opacity: 0;
      transform: scale(0.88);
    }
    22% {
      opacity: 1;
      transform: scale(1);
    }
    100% {
      opacity: 0;
      transform: scale(1.04);
    }
  }

  .video-play-overlay {
    top: 50%;
    left: 50%;
    z-index: 4;
    height: 3.35rem;
    width: 3.35rem;
    border: 1px solid var(--color-glass-border);
    background: var(--background-image-glass-play);
    box-shadow: var(--shadow-play);
    color: var(--color-secondary);
    transform: translate(-50%, -50%);
    backdrop-filter: blur(18px) saturate(150%);
    -webkit-backdrop-filter: blur(18px) saturate(150%);
  }

  .video-play-message {
    top: calc(50% + 3.25rem);
    left: 50%;
    z-index: 4;
    width: max-content;
    max-width: calc(100% - 2rem);
    padding: 0.45rem 0.75rem;
    border: 1px solid var(--color-glass-border);
    background: var(--background-image-glass);
    box-shadow: var(--shadow-popover);
    color: var(--color-secondary);
    transform: translateX(-50%);
    backdrop-filter: blur(24px) saturate(170%);
    -webkit-backdrop-filter: blur(24px) saturate(170%);
  }
</style>
