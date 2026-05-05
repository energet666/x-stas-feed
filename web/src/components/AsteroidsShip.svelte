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

  type Asteroid = {
    id: number;
    x: number;
    y: number;
    vx: number;
    vy: number;
    radius: number;
    angle: number;
    spin: number;
    path: string;
  };

  type Explosion = {
    id: number;
    x: number;
    y: number;
  };

  const shipWidth = 43.2;
  const shipHeight = 55.2;
  const shipSize = Math.max(shipWidth, shipHeight);
  const shipNoseOffset = 25;
  const shipCollisionRadius = 18;
  const maxSpeed = 8.5;
  const turnSpeed = 0.105;
  const thrust = 0.18;
  const reverseThrust = 0.075;
  const drag = 0.992;
  const bulletSpeed = 11;
  const bulletLifetime = 82;
  const asteroidRespawnMs = 650;
  const headerSafeHeight = 96;
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
  let asteroid = $state<Asteroid | undefined>(undefined);
  let explosion = $state<Explosion | undefined>(undefined);
  let animationFrameID: number | undefined = undefined;
  let asteroidRespawnTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  let lastFrameAt = 0;
  let nextBulletID = 1;
  let nextAsteroidID = 1;
  let nextExplosionID = 1;
  let videoActive = false;
  let shipControlled = false;

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
    if (event.code !== 'Space') {
      startAsteroidGame();
    }
    keys.add(keyID(event));
    if (event.code === 'Space') {
      shoot();
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

  function shoot() {
    startAsteroidGame();
    if (bullets.length > 0) return;

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

    bullets = [bullet];
  }

  function resize() {
    viewportWidth = window.innerWidth;
    viewportHeight = window.innerHeight;
    ship.x = wrap(ship.x, -shipSize, viewportWidth + shipSize);
    ship.y = wrap(ship.y, -shipSize, viewportHeight + shipSize);
    if (asteroid) {
      asteroid.x = wrap(asteroid.x, -asteroid.radius, viewportWidth + asteroid.radius);
      asteroid.y = wrap(asteroid.y, -asteroid.radius, viewportHeight + asteroid.radius);
    }
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
    if (asteroid) {
      asteroid.x = wrap(asteroid.x + asteroid.vx * delta, -asteroid.radius, viewportWidth + asteroid.radius);
      asteroid.y = wrap(asteroid.y + asteroid.vy * delta, -asteroid.radius, viewportHeight + asteroid.radius);
      asteroid.angle += asteroid.spin * delta;
      detectAsteroidHit();
      detectShipCollision();
    }

    animationFrameID = requestAnimationFrame(animate);
  }

  function startAsteroidGame() {
    if (shipControlled) return;
    shipControlled = true;
    spawnAsteroid();
  }

  function spawnAsteroid() {
    if (!viewportWidth || !viewportHeight || asteroid) return;
    const radius = 28 + Math.random() * 16;
    const position = randomAsteroidPosition(radius);
    asteroid = {
      id: nextAsteroidID++,
      x: position.x,
      y: position.y,
      vx: randomSigned(0.45, 1.15),
      vy: randomSigned(0.25, 0.85),
      radius,
      angle: Math.random() * Math.PI * 2,
      spin: randomSigned(0.008, 0.025),
      path: createAsteroidPath(14)
    };
  }

  function randomAsteroidPosition(radius: number) {
    const shipCenterX = ship.x + shipWidth / 2;
    const shipCenterY = ship.y + shipHeight / 2;
    let x = 0;
    let y = 0;

    for (let attempt = 0; attempt < 12; attempt += 1) {
      x = radius + Math.random() * Math.max(1, viewportWidth - radius * 2);
      y = headerSafeHeight + radius + Math.random() * Math.max(1, viewportHeight - headerSafeHeight - radius * 2);
      if (Math.hypot(x - shipCenterX, y - shipCenterY) > 220) {
        return { x, y };
      }
    }

    return { x, y };
  }

  function createAsteroidPath(points: number) {
    const commands: string[] = [];
    for (let i = 0; i < points; i += 1) {
      const angle = (i / points) * Math.PI * 2;
      const radius = 34 + Math.random() * 15;
      const x = 50 + Math.cos(angle) * radius;
      const y = 50 + Math.sin(angle) * radius;
      commands.push(`${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`);
    }

    return `${commands.join(' ')} Z`;
  }

  function randomSigned(min: number, max: number) {
    const value = min + Math.random() * (max - min);
    return Math.random() > 0.5 ? value : -value;
  }

  function detectAsteroidHit() {
    if (!asteroid || bullets.length === 0) return;

    const hit = bullets.find((bullet) => Math.hypot(bullet.x - asteroid!.x, bullet.y - asteroid!.y) <= asteroid!.radius + 5);
    if (!hit) return;

    bullets = bullets.filter((bullet) => bullet.id !== hit.id);
    destroyAsteroid(asteroid.x, asteroid.y);
  }

  function detectShipCollision() {
    if (!asteroid) return;
    const center = shipCenter();
    if (Math.hypot(center.x - asteroid.x, center.y - asteroid.y) > asteroid.radius + shipCollisionRadius) return;

    bullets = [];
    resetShip();
    destroyAsteroid(asteroid.x, asteroid.y);
  }

  function destroyAsteroid(x: number, y: number) {
    explosion = {
      id: nextExplosionID++,
      x,
      y
    };
    asteroid = undefined;
    window.setTimeout(() => {
      explosion = undefined;
    }, 360);
    clearTimeout(asteroidRespawnTimer);
    asteroidRespawnTimer = setTimeout(spawnAsteroid, asteroidRespawnMs);
  }

  function resetShip() {
    keys.clear();
    ship.x = 8;
    ship.y = 4;
    ship.vx = 0;
    ship.vy = 0;
    ship.angle = 0;
    ship.thrusting = false;
  }

  function shipCenter() {
    return {
      x: ship.x + shipWidth / 2,
      y: ship.y + shipHeight / 2
    };
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
      clearTimeout(asteroidRespawnTimer);
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

{#if asteroid}
  <svg
    class="asteroids-rock"
    aria-hidden="true"
    viewBox="0 0 100 100"
    style:width={`${asteroid.radius * 2}px`}
    style:height={`${asteroid.radius * 2}px`}
    style:transform={`translate3d(${asteroid.x}px, ${asteroid.y}px, 0) translate(-50%, -50%) rotate(${asteroid.angle}rad)`}
  >
    <path class="asteroids-rock-fill" d={asteroid.path} />
    <path class="asteroids-rock-line" d={asteroid.path} />
  </svg>
{/if}

{#if explosion}
  <span
    class="asteroids-explosion"
    aria-hidden="true"
    style:transform={`translate3d(${explosion.x}px, ${explosion.y}px, 0) translate(-50%, -50%)`}
  ></span>
{/if}

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

  .asteroids-rock {
    position: fixed;
    top: 0;
    left: 0;
    z-index: 0;
    overflow: visible;
    pointer-events: none;
    filter: drop-shadow(0 0 12px rgb(148 163 184 / 0.28));
    will-change: transform;
  }

  .asteroids-rock-fill {
    fill: rgb(15 23 42 / 0.72);
    stroke: none;
  }

  .asteroids-rock-line {
    fill: none;
    stroke: rgb(226 232 240 / 0.72);
    stroke-linejoin: round;
    stroke-width: 4;
  }

  .asteroids-explosion {
    position: fixed;
    top: 0;
    left: 0;
    z-index: 0;
    width: 4.8rem;
    height: 4.8rem;
    border: 2px solid rgb(253 224 71 / 0.86);
    border-radius: 999px;
    background: radial-gradient(circle, rgb(254 240 138 / 0.65) 0 18%, rgb(251 146 60 / 0.34) 19% 42%, transparent 58%);
    box-shadow:
      0 0 18px rgb(253 224 71 / 0.72),
      0 0 32px rgb(34 211 238 / 0.34);
    pointer-events: none;
    animation: asteroid-pop 360ms ease-out forwards;
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

  @keyframes asteroid-pop {
    from {
      opacity: 0.95;
    }

    to {
      opacity: 0;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .ship-flame {
      animation: none;
    }
  }
</style>
