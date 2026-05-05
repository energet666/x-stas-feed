<script lang="ts">
  import { onMount } from 'svelte';

  type ShipState = {
    x: number;
    y: number;
    vx: number;
    vy: number;
    angle: number;
    thrusting: boolean;
  };

  type Bullet = {
    id: number;
    x: number;
    y: number;
    vx: number;
    vy: number;
    age: number;
  };

  const shipWidth = 43.2;
  const shipHeight = 55.2;
  const shipSize = Math.max(shipWidth, shipHeight);
  const shipNoseOffset = 25;
  const maxSpeed = 8.5;
  const turnSpeed = 0.105;
  const thrust = 0.18;
  const reverseThrust = 0.075;
  const drag = 0.992;
  const bulletSpeed = 11;
  const bulletLifetime = 82;
  const fireCooldownMs = 135;
  const maxBullets = 24;
  const activeVideoEvent = 'feed-ai:video-active';
  const clearActiveVideoEvent = 'feed-ai:video-clear-active';
  const keys = new Set<string>();

  let ship = $state<ShipState>({
    x: 8,
    y: 4,
    vx: 0,
    vy: 0,
    angle: 0,
    thrusting: false
  });
  let viewportWidth = $state(0);
  let viewportHeight = $state(0);
  let bullets = $state<Bullet[]>([]);
  let animationFrameID: number | undefined = undefined;
  let lastFrameAt = 0;
  let nextBulletID = 1;
  let lastShotAt = 0;
  let videoActive = false;

  const shipTransform = $derived(
    `translate3d(${ship.x}px, ${ship.y}px, 0) rotate(${ship.angle + Math.PI / 2}rad)`
  );

  function isTextEntryTarget(target: EventTarget | null) {
    if (!(target instanceof HTMLElement)) return false;
    return Boolean(target.closest('input, textarea, select, [contenteditable="true"]'));
  }

  function handleKeydown(event: KeyboardEvent) {
    if (videoActive && isShipKey(event)) {
      keys.delete(keyID(event));
      ship.thrusting = false;
      return;
    }
    if (!isShipKey(event) || isTextEntryTarget(event.target)) return;
    keys.add(keyID(event));
    if (event.code === 'Space') {
      shoot(event.timeStamp || performance.now());
    }
    event.preventDefault();
  }

  function handleKeyup(event: KeyboardEvent) {
    if (videoActive && isShipKey(event)) {
      keys.delete(keyID(event));
      ship.thrusting = false;
      return;
    }
    if (!isShipKey(event)) return;
    keys.delete(keyID(event));
    event.preventDefault();
  }

  function isShipKey(event: KeyboardEvent) {
    return (
      event.key === 'ArrowUp' ||
      event.key === 'ArrowDown' ||
      event.key === 'ArrowLeft' ||
      event.key === 'ArrowRight' ||
      event.code === 'Space'
    );
  }

  function keyID(event: KeyboardEvent) {
    return event.code === 'Space' ? 'Space' : event.key;
  }

  function shoot(now: number) {
    if (now - lastShotAt < fireCooldownMs) return;
    lastShotAt = now;

    const centerX = ship.x + shipWidth / 2;
    const centerY = ship.y + shipHeight / 2;
    const noseX = centerX + Math.cos(ship.angle) * shipNoseOffset;
    const noseY = centerY + Math.sin(ship.angle) * shipNoseOffset;
    const bullet = {
      id: nextBulletID++,
      x: noseX,
      y: noseY,
      vx: ship.vx + Math.cos(ship.angle) * bulletSpeed,
      vy: ship.vy + Math.sin(ship.angle) * bulletSpeed,
      age: 0
    };

    bullets = [...bullets.slice(-(maxBullets - 1)), bullet];
  }

  function resize() {
    viewportWidth = window.innerWidth;
    viewportHeight = window.innerHeight;
    ship.x = wrap(ship.x, -shipSize, viewportWidth + shipSize);
    ship.y = wrap(ship.y, -shipSize, viewportHeight + shipSize);
  }

  function animate(now: number) {
    const delta = lastFrameAt > 0 ? Math.min((now - lastFrameAt) / 16.67, 2.4) : 1;
    lastFrameAt = now;

    if (keys.has('ArrowLeft')) {
      ship.angle -= turnSpeed * delta;
    }
    if (keys.has('ArrowRight')) {
      ship.angle += turnSpeed * delta;
    }

    ship.thrusting = keys.has('ArrowUp');
    if (ship.thrusting || keys.has('ArrowDown')) {
      const force = ship.thrusting ? thrust : -reverseThrust;
      ship.vx += Math.cos(ship.angle) * force * delta;
      ship.vy += Math.sin(ship.angle) * force * delta;
    }

    const speed = Math.hypot(ship.vx, ship.vy);
    if (speed > maxSpeed) {
      const scale = maxSpeed / speed;
      ship.vx *= scale;
      ship.vy *= scale;
    }

    ship.vx *= Math.pow(drag, delta);
    ship.vy *= Math.pow(drag, delta);
    ship.x = wrap(ship.x + ship.vx * delta, -shipSize, viewportWidth + shipSize);
    ship.y = wrap(ship.y + ship.vy * delta, -shipSize, viewportHeight + shipSize);
    bullets = bullets
      .map((bullet) => ({
        ...bullet,
        x: wrap(bullet.x + bullet.vx * delta, -8, viewportWidth + 8),
        y: wrap(bullet.y + bullet.vy * delta, -8, viewportHeight + 8),
        age: bullet.age + delta
      }))
      .filter((bullet) => bullet.age < bulletLifetime);

    animationFrameID = requestAnimationFrame(animate);
  }

  function wrap(value: number, min: number, max: number) {
    const range = max - min;
    if (range <= 0) return value;
    return ((((value - min) % range) + range) % range) + min;
  }

  onMount(() => {
    const markVideoActive = () => {
      videoActive = true;
      keys.clear();
      ship.thrusting = false;
    };
    const markVideoInactive = () => {
      videoActive = false;
    };

    resize();
    window.addEventListener('resize', resize);
    window.addEventListener('keydown', handleKeydown);
    window.addEventListener('keyup', handleKeyup);
    window.addEventListener(activeVideoEvent, markVideoActive);
    window.addEventListener(clearActiveVideoEvent, markVideoInactive);
    animationFrameID = requestAnimationFrame(animate);

    return () => {
      keys.clear();
      window.removeEventListener('resize', resize);
      window.removeEventListener('keydown', handleKeydown);
      window.removeEventListener('keyup', handleKeyup);
      window.removeEventListener(activeVideoEvent, markVideoActive);
      window.removeEventListener(clearActiveVideoEvent, markVideoInactive);
      if (animationFrameID !== undefined) {
        cancelAnimationFrame(animationFrameID);
      }
    };
  });
</script>

{#each bullets as bullet (bullet.id)}
  <span
    class="asteroids-bullet"
    aria-hidden="true"
    style:transform={`translate3d(${bullet.x}px, ${bullet.y}px, 0) translate(-50%, -50%)`}
  ></span>
{/each}

<div
  class="asteroids-ship"
  class:asteroids-ship-thrusting={ship.thrusting}
  aria-hidden="true"
  style:transform={shipTransform}
>
  <svg viewBox="0 0 42 54" role="img">
    <path class="ship-glow" d="M21 3 39 49 21 39 3 49 21 3Z" />
    <path class="ship-outline" d="M21 3 39 49 21 39 3 49 21 3Z" />
    <path class="ship-window" d="M21 15 27 31 21 27 15 31 21 15Z" />
    <path class="ship-flame" d="M21 42 27 55 21 50 15 55 21 42Z" />
  </svg>
</div>

<style>
  .asteroids-ship {
    position: fixed;
    top: 0;
    left: 0;
    z-index: 0;
    width: 2.7rem;
    height: 3.45rem;
    pointer-events: none;
    transform-origin: 50% 50%;
    filter: drop-shadow(0 0 10px rgb(125 211 252 / 0.5));
    will-change: transform;
  }

  .asteroids-bullet {
    position: fixed;
    top: 0;
    left: 0;
    z-index: 0;
    width: 0.42rem;
    height: 0.42rem;
    border-radius: 999px;
    background: rgb(253 224 71 / 0.96);
    box-shadow:
      0 0 6px rgb(253 224 71 / 0.92),
      0 0 14px rgb(34 211 238 / 0.5);
    pointer-events: none;
    transform-origin: 50% 50%;
    will-change: transform;
  }

  .asteroids-ship svg {
    display: block;
    width: 100%;
    height: 100%;
    overflow: visible;
  }

  .ship-glow {
    fill: rgb(125 211 252 / 0.12);
    stroke: rgb(125 211 252 / 0.26);
    stroke-width: 7;
  }

  .ship-outline {
    fill: rgb(3 7 18 / 0.5);
    stroke: rgb(255 255 255 / 0.88);
    stroke-linejoin: round;
    stroke-width: 2.25;
  }

  .ship-window {
    fill: rgb(34 211 238 / 0.28);
    stroke: rgb(165 243 252 / 0.74);
    stroke-linejoin: round;
    stroke-width: 1.4;
  }

  .ship-flame {
    fill: rgb(251 146 60 / 0);
    stroke: rgb(251 191 36 / 0);
    stroke-linejoin: round;
    stroke-width: 1.7;
    transform-origin: 50% 84%;
  }

  .asteroids-ship-thrusting .ship-flame {
    animation: ship-thrust 120ms steps(2, end) infinite;
    fill: rgb(251 146 60 / 0.84);
    stroke: rgb(254 240 138 / 0.8);
  }

  @keyframes ship-thrust {
    from {
      transform: scaleY(0.72);
      opacity: 0.74;
    }

    to {
      transform: scaleY(1.08);
      opacity: 1;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .ship-flame {
      animation: none;
    }
  }
</style>
