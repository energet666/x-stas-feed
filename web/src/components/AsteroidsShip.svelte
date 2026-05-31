<script lang="ts">
  import { onMount } from 'svelte';
  import {
    createShipScore,
    fetchShipScores,
    shipSocketURL,
    type ShipScore,
    type ShipSnapshot,
    type ShipState as NetworkShipState
  } from '../lib/feed';
  import { uiText as t } from '../lib/ui_text';

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

  type Particle = {
    id: number;
    kind: 'smoke' | 'spark' | 'debris';
    x: number;
    y: number;
    vx: number;
    vy: number;
    age: number;
    lifetime: number;
    size: number;
    angle?: number;
    spin?: number;
  };

  type RoundStatus = 'idle' | 'playing' | 'finished';

  const shipWidth = 43.2;
  const shipHeight = 55.2;
  const shipSize = Math.max(shipWidth, shipHeight);
  const shipNoseOffset = 25;
  const shipCollisionRadius = 18;
  const maxSpeed = 8.5;
  const turnSpeed = 0.105;
  const thrust = 0.18;
  const drag = 0.992;
  const bulletSpeed = 11;
  const bulletLifetime = 82;
  const smokeLifetime = 30;
  const smokeSpawnInterval = 2.4;
  const explosionSparkCount = 10;
  const explosionDebrisCount = 4;
  const explosionSmokeCount = 5;
  const roundDurationMs = 60_000;
  const asteroidHitScore = 100;
  const collisionPenalty = 200;
  const asteroidRespawnMs = 650;
  const shipSpawnGraceMs = 1200;
  const headerSafeHeight = 96;
  const activeVideoEvent = 'feed-ai:video-active';
  const clearActiveVideoEvent = 'feed-ai:video-clear-active';
  const backgroundKeyboardFocusEvent = 'feed-ai:background-keyboard-focus';
  const gameStartedEvent = 'feed-ai:game-started';
  const gameExitedEvent = 'feed-ai:game-exited';
  const shipPostIntervalMs = 16;
  const shipSessionStorageKey = 'feed-ai:ship-session-id';
  const keys = new Set<string>();

  let { username = t.common.guest }: { username: string } = $props();

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
  let particles = $state<Particle[]>([]);
  let asteroid = $state<Asteroid | undefined>(undefined);
  let explosion = $state<Explosion | undefined>(undefined);
  let animationFrameID: number | undefined = undefined;
  let asteroidRespawnTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  let lastFrameAt = 0;
  let nextBulletID = 1;
  let nextSmokeID = 1;
  let nextAsteroidID = 1;
  let nextExplosionID = 1;
  let videoActive = false;
  let backgroundKeyboardFocused = false;
  let shipControlled = $state(false);
  let sessionID = '';
  let lastShipPostAt = 0;
  let remoteShips = $state<NetworkShipState[]>([]);
  let roundStatus = $state<RoundStatus>('idle');
  let roundEndsAt = 0;
  let roundRemainingMs = $state(roundDurationMs);
  let score = $state(0);
  let leaderboard = $state<ShipScore[]>([]);
  let scoreSubmitStatus = $state<'idle' | 'saving' | 'saved' | 'error'>('idle');
  let roundSaved = $state(false);
  let scoreSubmissionID = 0;
  let shipSocket: WebSocket | undefined = undefined;
  let audioContext: AudioContext | undefined = undefined;
  let lastThrustSoundAt = 0;
  let smokeSpawnAccumulator = 0;
  let shipCollisionGraceUntil = 0;
  const pendingAsteroidHits = new Set<string>();

  const shipTransform = $derived(
    `translate3d(${ship.x}px, ${ship.y}px, 0) rotate(${ship.angle + Math.PI / 2}rad)`
  );
  const roundSecondsLeft = $derived(Math.max(0, Math.ceil(roundRemainingMs / 1000)));

  function isTextEntryTarget(target: EventTarget | null) {
    if (!(target instanceof HTMLElement)) return false;
    return Boolean(target.closest('input, textarea, select, [contenteditable="true"]'));
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && roundStatus !== 'idle' && !isTextEntryTarget(event.target)) {
      exitGameMode();
      event.preventDefault();
      return;
    }
    if (roundStatus === 'finished') {
      if (event.code === 'Enter' && backgroundKeyboardFocused && !videoActive && !isTextEntryTarget(event.target)) {
        restartRound();
        event.preventDefault();
      }
      if (backgroundKeyboardFocused && isShipKey(event)) {
        event.preventDefault();
      }
      return;
    }
    if (
      roundStatus === 'playing' &&
      event.code === 'Enter' &&
      backgroundKeyboardFocused &&
      !videoActive &&
      !isTextEntryTarget(event.target)
    ) {
      finishRound(false);
      event.preventDefault();
      return;
    }
    if ((videoActive || !backgroundKeyboardFocused) && isShipKey(event)) {
      keys.delete(keyID(event));
      ship.thrusting = false;
      return;
    }
    if (!isShipKey(event) || isTextEntryTarget(event.target)) return;
    primeAudio();
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
    if (roundStatus === 'finished' && backgroundKeyboardFocused && isShipKey(event)) {
      keys.delete(keyID(event));
      event.preventDefault();
      return;
    }
    if ((videoActive || !backgroundKeyboardFocused) && isShipKey(event)) {
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
    if (roundStatus !== 'playing') return;
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
    playShootSound();
  }

  function resize() {
    viewportWidth = window.innerWidth;
    viewportHeight = window.innerHeight;
    if (!shipControlled) {
      placeShipAtSpawn();
      return;
    }
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
    updateRoundTimer(now);

    if (keys.has('ArrowLeft')) {
      ship.angle -= turnSpeed * delta;
    }
    if (keys.has('ArrowRight')) {
      ship.angle += turnSpeed * delta;
    }

    ship.thrusting = keys.has('ArrowUp');
    if (ship.thrusting && now - lastThrustSoundAt > 130) {
      lastThrustSoundAt = now;
      playThrustSound();
    }
    if (ship.thrusting) {
      smokeSpawnAccumulator += delta;
      while (smokeSpawnAccumulator >= smokeSpawnInterval) {
        smokeSpawnAccumulator -= smokeSpawnInterval;
        spawnSmokeParticle();
      }
    } else {
      smokeSpawnAccumulator = 0;
    }
    if (ship.thrusting) {
      ship.vx += Math.cos(ship.angle) * thrust * delta;
      ship.vy += Math.sin(ship.angle) * thrust * delta;
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
    particles = particles
      .map((particle) => ({
        ...particle,
        x: particle.x + particle.vx * delta,
        y: particle.y + particle.vy * delta,
        vx: particle.vx * Math.pow(0.985, delta),
        vy: particle.vy * Math.pow(0.985, delta),
        angle: (particle.angle ?? 0) + (particle.spin ?? 0) * delta,
        age: particle.age + delta
      }))
      .filter((particle) => particle.age < particle.lifetime);
    if (asteroid) {
      asteroid.x = wrap(asteroid.x + asteroid.vx * delta, -asteroid.radius, viewportWidth + asteroid.radius);
      asteroid.y = wrap(asteroid.y + asteroid.vy * delta, -asteroid.radius, viewportHeight + asteroid.radius);
      asteroid.angle += asteroid.spin * delta;
      detectAsteroidHit();
      detectShipCollision();
    }
    if (shipControlled) {
      detectRemoteShipCollision();
      detectRemoteAsteroidHit();
      detectRemoteBulletCollision();
      detectRemoteAsteroidCollision();
      publishShipThrottled(now);
    }

    animationFrameID = requestAnimationFrame(animate);
  }

  function startAsteroidGame() {
    if (roundStatus === 'finished') return;
    if (roundStatus === 'idle') {
      startRound();
    }
    if (shipControlled) return;
    shipControlled = true;
    shipCollisionGraceUntil = performance.now() + shipSpawnGraceMs;
    window.dispatchEvent(new CustomEvent(gameStartedEvent));
    spawnAsteroid();
  }

  function startRound() {
    roundStatus = 'playing';
    roundEndsAt = performance.now() + roundDurationMs;
    roundRemainingMs = roundDurationMs;
    score = 0;
    leaderboard = [];
    scoreSubmitStatus = 'idle';
    roundSaved = false;
    scoreSubmissionID += 1;
  }

  function restartRound() {
    resetShip();
    bullets = [];
    particles = [];
    asteroid = undefined;
    explosion = undefined;
    remoteShips = [];
    pendingAsteroidHits.clear();
    clearTimeout(asteroidRespawnTimer);
    nextBulletID = 1;
    nextAsteroidID = 1;
    roundStatus = 'idle';
    roundEndsAt = 0;
    roundRemainingMs = roundDurationMs;
    score = 0;
    leaderboard = [];
    scoreSubmitStatus = 'idle';
    roundSaved = false;
    scoreSubmissionID += 1;
    startAsteroidGame();
  }

  function exitGameMode() {
    resetShip();
    bullets = [];
    particles = [];
    asteroid = undefined;
    explosion = undefined;
    pendingAsteroidHits.clear();
    clearTimeout(asteroidRespawnTimer);
    roundStatus = 'idle';
    roundEndsAt = 0;
    roundRemainingMs = roundDurationMs;
    score = 0;
    leaderboard = [];
    scoreSubmitStatus = 'idle';
    roundSaved = false;
    scoreSubmissionID += 1;
    void publishShip();
    window.dispatchEvent(new CustomEvent(gameExitedEvent));
  }

  function updateRoundTimer(now: number) {
    if (roundStatus !== 'playing') return;
    roundRemainingMs = Math.max(0, roundEndsAt - now);
    if (roundRemainingMs <= 0) {
      finishRound(true);
    }
  }

  function finishRound(shouldSubmitScore: boolean) {
    if (roundStatus !== 'playing') return;
    roundStatus = 'finished';
    roundSaved = shouldSubmitScore;
    roundRemainingMs = 0;
    keys.clear();
    bullets = [];
    asteroid = undefined;
    ship.thrusting = false;
    shipControlled = false;
    clearTimeout(asteroidRespawnTimer);
    scoreSubmissionID += 1;
    if (shouldSubmitScore) {
      void submitScore(scoreSubmissionID, score);
    } else {
      void loadLeaderboard(scoreSubmissionID);
    }
    void publishShip();
  }

  async function submitScore(submissionID: number, finalScore: number) {
    scoreSubmitStatus = 'saving';
    try {
      const scores = await createShipScore(username.trim() || t.common.guest, finalScore);
      if (submissionID !== scoreSubmissionID || roundStatus !== 'finished') return;
      leaderboard = scores;
      scoreSubmitStatus = 'saved';
    } catch {
      if (submissionID !== scoreSubmissionID || roundStatus !== 'finished') return;
      scoreSubmitStatus = 'error';
    }
  }

  async function loadLeaderboard(submissionID: number) {
    scoreSubmitStatus = 'saving';
    try {
      const scores = await fetchShipScores();
      if (submissionID !== scoreSubmissionID || roundStatus !== 'finished') return;
      leaderboard = scores;
      scoreSubmitStatus = 'saved';
    } catch {
      if (submissionID !== scoreSubmissionID || roundStatus !== 'finished') return;
      scoreSubmitStatus = 'error';
    }
  }

  function addScore(delta: number) {
    if (roundStatus !== 'playing') return;
    score += delta;
  }

  function spawnAsteroid() {
    if (roundStatus !== 'playing' || !viewportWidth || !viewportHeight || asteroid) return;
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

  function spawnSmokeParticle() {
    const center = shipCenter();
    const tailX = center.x - Math.cos(ship.angle) * 20;
    const tailY = center.y - Math.sin(ship.angle) * 20;
    const spread = (Math.random() - 0.5) * 1.15;
    const driftAngle = ship.angle + Math.PI + spread;
    particles = [
      ...particles.slice(-96),
      {
        id: nextSmokeID++,
        kind: 'smoke',
        x: tailX + (Math.random() - 0.5) * 6,
        y: tailY + (Math.random() - 0.5) * 6,
        vx: ship.vx * 0.28 + Math.cos(driftAngle) * (0.65 + Math.random() * 0.75),
        vy: ship.vy * 0.28 + Math.sin(driftAngle) * (0.65 + Math.random() * 0.75),
        age: 0,
        lifetime: smokeLifetime + Math.random() * 12,
        size: 0.55 + Math.random() * 0.65
      }
    ];
  }

  function spawnExplosionParticles(x: number, y: number) {
    const burst: Particle[] = [];

    for (let i = 0; i < explosionSparkCount; i += 1) {
      const angle = Math.random() * Math.PI * 2;
      const speed = 1.4 + Math.random() * 3.1;
      burst.push({
        id: nextSmokeID++,
        kind: 'spark',
        x,
        y,
        vx: Math.cos(angle) * speed,
        vy: Math.sin(angle) * speed,
        age: 0,
        lifetime: 14 + Math.random() * 10,
        size: 0.45 + Math.random() * 0.75
      });
    }

    for (let i = 0; i < explosionDebrisCount; i += 1) {
      const angle = Math.random() * Math.PI * 2;
      const speed = 0.6 + Math.random() * 1.7;
      burst.push({
        id: nextSmokeID++,
        kind: 'debris',
        x: x + (Math.random() - 0.5) * 10,
        y: y + (Math.random() - 0.5) * 10,
        vx: Math.cos(angle) * speed,
        vy: Math.sin(angle) * speed,
        age: 0,
        lifetime: 24 + Math.random() * 14,
        size: 0.65 + Math.random() * 0.85,
        angle: Math.random() * Math.PI * 2,
        spin: (Math.random() - 0.5) * 0.2
      });
    }

    for (let i = 0; i < explosionSmokeCount; i += 1) {
      const angle = Math.random() * Math.PI * 2;
      const speed = 0.3 + Math.random() * 1;
      burst.push({
        id: nextSmokeID++,
        kind: 'smoke',
        x: x + (Math.random() - 0.5) * 12,
        y: y + (Math.random() - 0.5) * 12,
        vx: Math.cos(angle) * speed,
        vy: Math.sin(angle) * speed,
        age: 0,
        lifetime: 28 + Math.random() * 16,
        size: 0.8 + Math.random() * 1.1
      });
    }

    particles = [...particles.slice(-40), ...burst];
  }

  function detectAsteroidHit() {
    if (!asteroid || bullets.length === 0) return;

    const hit = bullets.find((bullet) => Math.hypot(bullet.x - asteroid!.x, bullet.y - asteroid!.y) <= asteroid!.radius + 5);
    if (!hit) return;

    bullets = bullets.filter((bullet) => bullet.id !== hit.id);
    addScore(asteroidHitScore);
    destroyAsteroid(asteroid.x, asteroid.y);
  }

  function detectShipCollision() {
    if (!asteroid) return;
    const center = shipCenter();
    if (Math.hypot(center.x - asteroid.x, center.y - asteroid.y) > asteroid.radius + shipCollisionRadius) return;

    bullets = [];
    addScore(-collisionPenalty);
    resetShip();
    destroyAsteroid(asteroid.x, asteroid.y);
    playShipDestroySound();
    void publishShip();
  }

  function detectRemoteShipCollision() {
    if (performance.now() < shipCollisionGraceUntil) return;
    const center = shipCenter();
    const hit = remoteShips.find(
      (remoteShip) =>
        isRemoteShipActive(remoteShip) &&
        Math.hypot(center.x - remoteShip.x, center.y - remoteShip.y) <= shipCollisionRadius * 2
    );
    if (!hit) return;

    bullets = [];
    addScore(-collisionPenalty);
    resetShip();
    explosion = {
      id: nextExplosionID++,
      x: (center.x + hit.x) / 2,
      y: (center.y + hit.y) / 2
    };
    window.setTimeout(() => {
      explosion = undefined;
    }, 360);
    playShipDestroySound();
    void publishShip();
  }

  function detectRemoteAsteroidHit() {
    if (bullets.length === 0) return;

    for (const remoteShip of remoteShips) {
      if (!isRemoteShipActive(remoteShip)) continue;
      const remoteAsteroid = remoteShip.asteroid;
      if (!remoteAsteroid) continue;
      const hitKey = `${remoteShip.id}:${remoteAsteroid.id}`;
      if (pendingAsteroidHits.has(hitKey)) continue;

      const hit = bullets.find(
        (bullet) => Math.hypot(bullet.x - remoteAsteroid.x, bullet.y - remoteAsteroid.y) <= remoteAsteroid.radius + 5
      );
      if (!hit) continue;

      bullets = bullets.filter((bullet) => bullet.id !== hit.id);
      pendingAsteroidHits.add(hitKey);
      sendAsteroidHit(remoteShip.id, remoteAsteroid.id, hit.x, hit.y);
      addScore(asteroidHitScore);
      void publishShip();
      return;
    }
  }

  function detectRemoteBulletCollision() {
    const center = shipCenter();
    for (const remoteShip of remoteShips) {
      if (!isRemoteShipActive(remoteShip)) continue;
      const hit = remoteShip.bullets?.find(
        (bullet) => Math.hypot(center.x - bullet.x, center.y - bullet.y) <= shipCollisionRadius + 5
      );
      if (!hit) continue;

      resetAfterRemoteHit((center.x + hit.x) / 2, (center.y + hit.y) / 2);
      return;
    }
  }

  function detectRemoteAsteroidCollision() {
    const center = shipCenter();
    const hit = remoteShips.find((remoteShip) => {
      if (!isRemoteShipActive(remoteShip)) return false;
      const remoteAsteroid = remoteShip.asteroid;
      return (
        remoteAsteroid &&
        Math.hypot(center.x - remoteAsteroid.x, center.y - remoteAsteroid.y) <= remoteAsteroid.radius + shipCollisionRadius
      );
    });
    if (!hit?.asteroid) return;

    resetAfterRemoteHit(hit.asteroid.x, hit.asteroid.y);
  }

  function resetAfterRemoteHit(x: number, y: number) {
    bullets = [];
    addScore(-collisionPenalty);
    resetShip();
    explosion = {
      id: nextExplosionID++,
      x,
      y
    };
    window.setTimeout(() => {
      explosion = undefined;
    }, 360);
    playShipDestroySound();
    void publishShip();
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
    spawnExplosionParticles(x, y);
    playExplosionSound();
    clearTimeout(asteroidRespawnTimer);
    asteroidRespawnTimer = setTimeout(spawnAsteroid, asteroidRespawnMs);
  }

  function resetShip() {
    keys.clear();
    placeShipAtSpawn();
    ship.vx = 0;
    ship.vy = 0;
    ship.angle = 0;
    ship.thrusting = false;
    shipControlled = false;
    shipCollisionGraceUntil = 0;
    particles = [];
    smokeSpawnAccumulator = 0;
  }

  function placeShipAtSpawn() {
    const point = selectSpawnPoint();
    ship.x = point.x;
    ship.y = point.y;
  }

  function selectSpawnPoint() {
    const points = spawnPoints();
    const start = sessionHash() % points.length;

    for (let offset = 0; offset < points.length; offset += 1) {
      const point = points[(start + offset) % points.length];
      const centerX = point.x + shipWidth / 2;
      const centerY = point.y + shipHeight / 2;
      const blocked = remoteShips.some(
        (remoteShip) =>
          isRemoteShipActive(remoteShip) &&
          Math.hypot(centerX - remoteShip.x, centerY - remoteShip.y) < shipCollisionRadius * 4
      );
      if (!blocked) return point;
    }

    return points[start];
  }

  function spawnPoints() {
    const margin = 18;
    const maxX = Math.max(margin, viewportWidth - shipWidth - margin);
    const maxY = Math.max(margin, viewportHeight - shipHeight - margin);
    const midX = Math.max(margin, viewportWidth / 2 - shipWidth / 2);
    const midY = Math.max(margin, viewportHeight / 2 - shipHeight / 2);

    return [
      { x: margin, y: margin },
      { x: maxX, y: margin },
      { x: margin, y: maxY },
      { x: maxX, y: maxY },
      { x: midX, y: margin },
      { x: midX, y: maxY },
      { x: margin, y: midY },
      { x: maxX, y: midY }
    ];
  }

  function sessionHash() {
    let hash = 0;
    for (let i = 0; i < sessionID.length; i += 1) {
      hash = (hash * 31 + sessionID.charCodeAt(i)) >>> 0;
    }
    return hash;
  }

  function isRemoteShipActive(remoteShip: NetworkShipState) {
    return remoteShip.active !== false;
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

  function connectShipSocket() {
    shipSocket?.close();
    shipSocket = new WebSocket(shipSocketURL());
    shipSocket.addEventListener('message', (event) => {
      try {
        const snapshot = JSON.parse(event.data) as ShipSnapshot;
        remoteShips = snapshot.ships.filter((remoteShip) => remoteShip.id !== sessionID);
        handleShipEvents(snapshot);
      } catch {
        // Ignore malformed game messages; the next snapshot can recover state.
      }
    });
    shipSocket.addEventListener('open', () => {
      if (shipControlled) {
        publishShip();
      }
    });
  }

  function publishShipThrottled(now: number) {
    if (now - lastShipPostAt < shipPostIntervalMs) return;
    lastShipPostAt = now;
    void publishShip();
  }

  function publishShip() {
    if (!sessionID || shipSocket?.readyState !== WebSocket.OPEN) return;
    try {
      const center = shipCenter();
      shipSocket.send(
        JSON.stringify({
          type: 'state',
          ship: {
            id: sessionID,
            name: username.trim() || t.common.guest,
            x: center.x,
            y: center.y,
            angle: ship.angle,
            thrusting: ship.thrusting,
            active: shipControlled && roundStatus === 'playing',
            bullets: shipControlled ? bullets.map((bullet) => ({ x: bullet.x, y: bullet.y })) : [],
            asteroid: shipControlled && asteroid
              ? {
                  id: asteroid.id,
                  x: asteroid.x,
                  y: asteroid.y,
                  radius: asteroid.radius,
                  angle: asteroid.angle,
                  path: asteroid.path
                }
              : undefined
          }
        })
      );
    } catch {
      // Multiplayer state is decorative; local ship controls should remain responsive.
    }
  }

  function sendAsteroidHit(ownerId: string, asteroidId: number, x: number, y: number) {
    if (!sessionID || shipSocket?.readyState !== WebSocket.OPEN) return;
    try {
      shipSocket.send(
        JSON.stringify({
          type: 'asteroid-hit',
          ownerId,
          asteroidId,
          x,
          y
        })
      );
    } catch {
      pendingAsteroidHits.delete(`${ownerId}:${asteroidId}`);
    }
  }

  function handleShipEvents(snapshot: ShipSnapshot) {
    for (const event of snapshot.events ?? []) {
      if (event.type !== 'asteroid-destroyed' || !event.ownerId || !event.asteroidId) continue;
      pendingAsteroidHits.delete(`${event.ownerId}:${event.asteroidId}`);
      if (event.ownerId === sessionID && asteroid?.id === event.asteroidId) {
        asteroid = undefined;
        clearTimeout(asteroidRespawnTimer);
        asteroidRespawnTimer = setTimeout(spawnAsteroid, asteroidRespawnMs);
      }
      if (Number.isFinite(event.x) && Number.isFinite(event.y)) {
        explosion = {
          id: nextExplosionID++,
          x: event.x ?? 0,
          y: event.y ?? 0
        };
        window.setTimeout(() => {
          explosion = undefined;
        }, 360);
        spawnExplosionParticles(event.x ?? 0, event.y ?? 0);
        playExplosionSound();
      }
    }
  }

  function primeAudio() {
    audioContext ??= new AudioContext();
    if (audioContext.state === 'suspended') {
      void audioContext.resume();
    }
  }

  function playShootSound() {
    const audio = audioContext;
    if (!audio) return;
    const now = audio.currentTime;
    const oscillator = audio.createOscillator();
    const gain = audio.createGain();

    oscillator.type = 'square';
    oscillator.frequency.setValueAtTime(920, now);
    oscillator.frequency.exponentialRampToValueAtTime(180, now + 0.09);
    gain.gain.setValueAtTime(0.075, now);
    gain.gain.exponentialRampToValueAtTime(0.001, now + 0.1);

    oscillator.connect(gain).connect(audio.destination);
    oscillator.start(now);
    oscillator.stop(now + 0.11);
  }

  function playThrustSound() {
    const audio = audioContext;
    if (!audio) return;
    const now = audio.currentTime;
    const oscillator = audio.createOscillator();
    const gain = audio.createGain();

    oscillator.type = 'sawtooth';
    oscillator.frequency.setValueAtTime(72, now);
    oscillator.frequency.linearRampToValueAtTime(54, now + 0.08);
    gain.gain.setValueAtTime(0.028, now);
    gain.gain.exponentialRampToValueAtTime(0.001, now + 0.11);

    oscillator.connect(gain).connect(audio.destination);
    oscillator.start(now);
    oscillator.stop(now + 0.12);
  }

  function playExplosionSound() {
    const audio = audioContext;
    if (!audio) return;
    const now = audio.currentTime;
    playExplosionThump(now, 112, 0.075);
    playExplosionThump(now + 0.082, 78, 0.11);
    playExplosionHiss(now + 0.12);
  }

  function playExplosionThump(startAt: number, startFrequency: number, volume: number) {
    const audio = audioContext;
    if (!audio) return;
    const oscillator = audio.createOscillator();
    const gain = audio.createGain();
    const filter = audio.createBiquadFilter();

    oscillator.type = 'triangle';
    oscillator.frequency.setValueAtTime(startFrequency, startAt);
    oscillator.frequency.exponentialRampToValueAtTime(42, startAt + 0.13);
    filter.type = 'lowpass';
    filter.frequency.setValueAtTime(520, startAt);
    filter.frequency.exponentialRampToValueAtTime(180, startAt + 0.12);
    gain.gain.setValueAtTime(volume, startAt);
    gain.gain.exponentialRampToValueAtTime(0.001, startAt + 0.15);

    oscillator.connect(filter).connect(gain).connect(audio.destination);
    oscillator.start(startAt);
    oscillator.stop(startAt + 0.16);
  }

  function playExplosionHiss(startAt: number) {
    const audio = audioContext;
    if (!audio) return;
    const noise = audio.createBufferSource();
    const buffer = audio.createBuffer(1, Math.floor(audio.sampleRate * 0.16), audio.sampleRate);
    const data = buffer.getChannelData(0);
    const filter = audio.createBiquadFilter();
    const gain = audio.createGain();

    for (let i = 0; i < data.length; i += 1) {
      data[i] = (Math.random() * 2 - 1) * Math.pow(1 - i / data.length, 2.2);
    }
    noise.buffer = buffer;
    filter.type = 'lowpass';
    filter.frequency.setValueAtTime(950, startAt);
    filter.frequency.exponentialRampToValueAtTime(420, startAt + 0.13);
    gain.gain.setValueAtTime(0.035, startAt);
    gain.gain.exponentialRampToValueAtTime(0.001, startAt + 0.16);

    noise.connect(filter).connect(gain).connect(audio.destination);
    noise.start(startAt);
    noise.stop(startAt + 0.17);
  }

  function playShipDestroySound() {
    const audio = audioContext;
    if (!audio) return;
    const now = audio.currentTime;
    const fall = audio.createOscillator();
    const fallGain = audio.createGain();
    const noise = audio.createBufferSource();
    const buffer = audio.createBuffer(1, Math.floor(audio.sampleRate * 0.16), audio.sampleRate);
    const data = buffer.getChannelData(0);
    const noiseGain = audio.createGain();

    for (let i = 0; i < data.length; i += 1) {
      data[i] = (Math.random() * 2 - 1) * (1 - i / data.length);
    }
    noise.buffer = buffer;
    fall.type = 'triangle';
    fall.frequency.setValueAtTime(420, now);
    fall.frequency.exponentialRampToValueAtTime(42, now + 0.28);
    fallGain.gain.setValueAtTime(0.12, now);
    fallGain.gain.exponentialRampToValueAtTime(0.001, now + 0.3);
    noiseGain.gain.setValueAtTime(0.055, now);
    noiseGain.gain.exponentialRampToValueAtTime(0.001, now + 0.14);

    fall.connect(fallGain).connect(audio.destination);
    noise.connect(noiseGain).connect(audio.destination);
    fall.start(now);
    noise.start(now);
    noise.stop(now + 0.16);
    fall.stop(now + 0.31);
  }

  function readShipSessionID() {
    try {
      const existing = window.sessionStorage.getItem(shipSessionStorageKey);
      if (existing) return existing;
      const nextID = crypto.randomUUID();
      window.sessionStorage.setItem(shipSessionStorageKey, nextID);
      return nextID;
    } catch {
      return `ship-${Date.now()}-${Math.random().toString(16).slice(2)}`;
    }
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
    const markBackgroundKeyboardFocus = (event: Event) => {
      backgroundKeyboardFocused = Boolean((event as CustomEvent<{ focused?: boolean }>).detail?.focused);
      if (!backgroundKeyboardFocused) {
        keys.clear();
        ship.thrusting = false;
      }
    };

    sessionID = readShipSessionID();
    resize();
    connectShipSocket();
    window.addEventListener('resize', resize);
    window.addEventListener('keydown', handleKeydown);
    window.addEventListener('keyup', handleKeyup);
    window.addEventListener(activeVideoEvent, markVideoActive);
    window.addEventListener(clearActiveVideoEvent, markVideoInactive);
    window.addEventListener(backgroundKeyboardFocusEvent, markBackgroundKeyboardFocus);
    animationFrameID = requestAnimationFrame(animate);

    return () => {
      keys.clear();
      window.removeEventListener('resize', resize);
      window.removeEventListener('keydown', handleKeydown);
      window.removeEventListener('keyup', handleKeyup);
      window.removeEventListener(activeVideoEvent, markVideoActive);
      window.removeEventListener(clearActiveVideoEvent, markVideoInactive);
      window.removeEventListener(backgroundKeyboardFocusEvent, markBackgroundKeyboardFocus);
      shipSocket?.close();
      void audioContext?.close();
      clearTimeout(asteroidRespawnTimer);
      if (animationFrameID !== undefined) {
        cancelAnimationFrame(animationFrameID);
      }
    };
  });
</script>

{#if roundStatus === 'playing'}
  <div class="asteroids-hud" aria-live="polite">
    <span>{roundSecondsLeft}s</span>
    <strong>{score}</strong>
  </div>
{/if}

{#if roundStatus === 'finished'}
  <section class="asteroids-leaderboard" aria-live="polite">
    <div class="asteroids-leaderboard-header">
      <span>{t.game.time}</span>
      <strong>{score}</strong>
    </div>
    <div class="asteroids-leaderboard-title">{t.game.leaders}</div>
    {#if !roundSaved}
      <p>{t.game.resultNotSaved}</p>
    {/if}
    {#if scoreSubmitStatus === 'saving'}
      <p>{roundSaved ? t.game.savingScore : t.game.loadingLeaders}</p>
    {:else if scoreSubmitStatus === 'error'}
      <p>{roundSaved ? t.game.scoreSaveFailed : t.game.leadersUnavailable}</p>
    {:else if leaderboard.length === 0}
      <p>{t.game.noScores}</p>
    {:else}
      <ol>
        {#each leaderboard as item, index (`${item.createdAt}-${index}`)}
          <li class:currentScore={roundSaved && item.name === (username.trim() || t.common.guest) && item.score === score}>
            <span>{index + 1}. {item.name}</span>
            <strong>{item.score}</strong>
          </li>
        {/each}
      </ol>
    {/if}
    <div class="asteroids-restart">{t.game.enterToRestart}</div>
  </section>
{/if}

{#each bullets as bullet (bullet.id)}
  <span
    class="asteroids-bullet"
    aria-hidden="true"
    style:transform={`translate3d(${bullet.x}px, ${bullet.y}px, 0) translate(-50%, -50%)`}
  ></span>
{/each}

{#each particles as particle (particle.id)}
  <span
    class={`asteroids-particle asteroids-${particle.kind}`}
    aria-hidden="true"
    style:--particle-alpha={1 - particle.age / particle.lifetime}
    style:--particle-scale={particle.size + (particle.age / particle.lifetime) * (particle.kind === 'spark' ? 0.2 : 1.4)}
    style:transform={`translate3d(${particle.x}px, ${particle.y}px, 0) translate(-50%, -50%) rotate(${particle.angle ?? 0}rad) scale(var(--particle-scale))`}
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

{#each remoteShips as remoteShip (remoteShip.id)}
  {#if isRemoteShipActive(remoteShip)}
    {#if remoteShip.asteroid}
      <svg
        class="asteroids-rock asteroids-remote-rock"
        aria-hidden="true"
        viewBox="0 0 100 100"
        style:width={`${remoteShip.asteroid.radius * 2}px`}
        style:height={`${remoteShip.asteroid.radius * 2}px`}
        style:transform={`translate3d(${remoteShip.asteroid.x}px, ${remoteShip.asteroid.y}px, 0) translate(-50%, -50%) rotate(${remoteShip.asteroid.angle}rad)`}
      >
        <path class="remote-rock-fill" d={remoteShip.asteroid.path} />
        <path class="remote-rock-line" d={remoteShip.asteroid.path} />
      </svg>
    {/if}
    {#each remoteShip.bullets ?? [] as bullet, index (`${remoteShip.id}-${index}`)}
      <span
        class="asteroids-bullet asteroids-remote-bullet"
        aria-hidden="true"
        style:transform={`translate3d(${bullet.x}px, ${bullet.y}px, 0) translate(-50%, -50%)`}
      ></span>
    {/each}
    <div
      class="asteroids-remote-ship"
      aria-hidden="true"
      style:transform={`translate3d(${remoteShip.x - shipWidth / 2}px, ${remoteShip.y - shipHeight / 2}px, 0) rotate(${remoteShip.angle + Math.PI / 2}rad)`}
    >
      <svg viewBox="0 0 42 54" role="img">
        <path class="remote-ship-glow" d="M21 3 39 49 21 39 3 49 21 3Z" />
        <path class="remote-ship-outline" d="M21 3 39 49 21 39 3 49 21 3Z" />
        <path class="remote-ship-window" d="M21 15 27 31 21 27 15 31 21 15Z" />
        <path class="remote-ship-flame" class:remote-ship-flame-active={remoteShip.thrusting} d="M21 42 27 55 21 50 15 55 21 42Z" />
      </svg>
    </div>
    <span
      class="asteroids-remote-name"
      aria-hidden="true"
      style:transform={`translate3d(${remoteShip.x}px, ${remoteShip.y + shipHeight / 2 + 8}px, 0) translate(-50%, 0)`}
    >
      {remoteShip.name}
    </span>
  {/if}
{/each}

{#if shipControlled}
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
{/if}

<style>
  .asteroids-hud {
    position: fixed;
    top: 1rem;
    left: 50%;
    z-index: 20;
    display: flex;
    min-width: 9rem;
    transform: translateX(-50%);
    align-items: center;
    justify-content: space-between;
    gap: 1.25rem;
    border: 1px solid rgb(226 232 240 / 0.18);
    border-radius: 0.5rem;
    background: rgb(2 6 23 / 0.72);
    padding: 0.55rem 0.8rem;
    color: rgb(226 232 240);
    font-size: 0.9rem;
    line-height: 1;
    pointer-events: none;
  }

  .asteroids-hud strong {
    color: rgb(253 224 71);
    font-size: 1.1rem;
    font-weight: 800;
  }

  .asteroids-leaderboard {
    position: fixed;
    top: 50%;
    left: 50%;
    z-index: 30;
    width: min(24rem, calc(100vw - 2rem));
    transform: translate(-50%, -50%);
    border: 1px solid rgb(226 232 240 / 0.18);
    border-radius: 0.5rem;
    background: rgb(2 6 23 / 0.88);
    padding: 1rem;
    color: rgb(226 232 240);
    box-shadow: 0 18px 48px rgb(0 0 0 / 0.32);
  }

  .asteroids-leaderboard-header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 1rem;
    border-bottom: 1px solid rgb(226 232 240 / 0.14);
    padding-bottom: 0.75rem;
  }

  .asteroids-leaderboard-header span,
  .asteroids-leaderboard-title,
  .asteroids-restart,
  .asteroids-leaderboard p {
    color: rgb(148 163 184);
    font-size: 0.82rem;
  }

  .asteroids-leaderboard-header strong {
    color: rgb(253 224 71);
    font-size: 2rem;
    line-height: 1;
  }

  .asteroids-leaderboard-title {
    margin-top: 0.9rem;
    text-transform: uppercase;
  }

  .asteroids-leaderboard ol {
    display: grid;
    gap: 0.35rem;
    margin: 0.65rem 0 0;
    padding: 0;
    list-style: none;
  }

  .asteroids-leaderboard li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    border-radius: 0.35rem;
    padding: 0.45rem 0.55rem;
    background: rgb(15 23 42 / 0.72);
    font-size: 0.92rem;
  }

  .asteroids-leaderboard li.currentScore {
    background: rgb(253 224 71 / 0.14);
    color: rgb(254 240 138);
  }

  .asteroids-leaderboard li span {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .asteroids-leaderboard li strong {
    color: rgb(253 224 71);
  }

  .asteroids-restart {
    margin-top: 0.85rem;
    text-align: center;
  }

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

  .asteroids-remote-ship {
    position: fixed;
    top: 0;
    left: 0;
    z-index: 0;
    width: 2.7rem;
    height: 3.45rem;
    pointer-events: none;
    transform-origin: 50% 50%;
    filter: drop-shadow(0 0 10px rgb(244 114 182 / 0.42));
    opacity: 0.86;
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

  .asteroids-particle {
    position: fixed;
    top: 0;
    left: 0;
    z-index: 0;
    pointer-events: none;
    will-change: transform, opacity;
  }

  .asteroids-smoke {
    width: 0.8rem;
    height: 0.8rem;
    border-radius: 999px;
    background: rgb(148 163 184 / calc(var(--particle-alpha) * 0.28));
    opacity: var(--particle-alpha);
  }

  .asteroids-spark {
    width: 0.45rem;
    height: 0.45rem;
    border-radius: 999px;
    background: rgb(253 224 71 / calc(var(--particle-alpha) * 0.96));
    box-shadow: 0 0 8px rgb(253 224 71 / calc(var(--particle-alpha) * 0.62));
    opacity: var(--particle-alpha);
  }

  .asteroids-debris {
    width: 0.75rem;
    height: 0.36rem;
    border-radius: 0.16rem;
    background: rgb(226 232 240 / calc(var(--particle-alpha) * 0.62));
    opacity: var(--particle-alpha);
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

  .asteroids-remote-rock {
    opacity: 0.76;
    filter: drop-shadow(0 0 12px rgb(244 114 182 / 0.22));
  }

  .remote-rock-fill {
    fill: rgb(39 7 24 / 0.48);
    stroke: none;
  }

  .remote-rock-line {
    fill: none;
    stroke: rgb(251 207 232 / 0.62);
    stroke-linejoin: round;
    stroke-width: 4;
  }

  .asteroids-remote-bullet {
    background: rgb(244 114 182 / 0.96);
    box-shadow:
      0 0 6px rgb(244 114 182 / 0.86),
      0 0 14px rgb(125 211 252 / 0.42);
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
    pointer-events: none;
    animation: asteroid-pop 260ms ease-out forwards;
  }

  .asteroids-ship svg {
    display: block;
    width: 100%;
    height: 100%;
    overflow: visible;
  }

  .asteroids-remote-ship svg {
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

  .remote-ship-glow {
    fill: rgb(244 114 182 / 0.12);
    stroke: rgb(244 114 182 / 0.26);
    stroke-width: 7;
  }

  .remote-ship-outline {
    fill: rgb(3 7 18 / 0.48);
    stroke: rgb(251 207 232 / 0.88);
    stroke-linejoin: round;
    stroke-width: 2.25;
  }

  .remote-ship-window {
    fill: rgb(244 114 182 / 0.26);
    stroke: rgb(251 207 232 / 0.72);
    stroke-linejoin: round;
    stroke-width: 1.4;
  }

  .remote-ship-flame {
    fill: rgb(251 146 60 / 0);
    stroke: rgb(251 191 36 / 0);
    stroke-linejoin: round;
    stroke-width: 1.7;
    transform-origin: 50% 84%;
  }

  .remote-ship-flame-active {
    animation: ship-thrust 120ms steps(2, end) infinite;
    fill: rgb(251 146 60 / 0.78);
    stroke: rgb(254 240 138 / 0.74);
  }

  .asteroids-remote-name {
    position: fixed;
    top: 0;
    left: 0;
    z-index: 0;
    max-width: 9rem;
    overflow: hidden;
    border: 1px solid rgb(251 207 232 / 0.24);
    border-radius: 999px;
    background: rgb(0 0 0 / 0.3);
    padding: 0.12rem 0.42rem;
    color: rgb(251 207 232 / 0.92);
    font-size: 0.62rem;
    font-weight: 800;
    line-height: 1.15;
    text-overflow: ellipsis;
    text-shadow: 0 1px 6px rgb(0 0 0 / 0.6);
    white-space: nowrap;
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
