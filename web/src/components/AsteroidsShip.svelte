<script lang="ts">
  import { RadioTower } from 'lucide-svelte';
  import { onMount } from 'svelte';
  import {
    fetchShipScores,
    shipSocketURL,
    type ShipAsteroid,
    type ShipBullet,
    type ShipCommand,
    type ShipControlZone,
    type ShipInput,
    type ShipPowerUp,
    type ShipScore,
    type ShipSnapshot,
    type ShipState,
    type ShipWelcome
  } from '../lib/feed';
  import { uiText as t } from '../lib/ui_text';

  type ConnectionState = 'connecting' | 'connected' | 'disconnected';
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
    angle: number;
    spin: number;
  };
  type Explosion = { id: number; x: number; y: number };

  const arenaWidth = 1600;
  const arenaHeight = 900;
  const shipWidth = 43.2;
  const shipHeight = 55.2;
  const maxSpeed = 8.5;
  const turnSpeed = 0.105;
  const thrust = 0.18;
  const drag = 0.992;
  const smokeLifetime = 30;
  const resumeTokenKey = 'feed-ai:ship-resume-token';
  const activeVideoEvent = 'feed-ai:video-active';
  const clearActiveVideoEvent = 'feed-ai:video-clear-active';
  const backgroundKeyboardControlEvent = 'feed-ai:background-keyboard-control';
  const gameStartedEvent = 'feed-ai:game-started';
  const gameExitedEvent = 'feed-ai:game-exited';

  let { username = t.common.guest }: { username: string } = $props();

  let socket: WebSocket | undefined;
  let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
  let heartbeatTimer: ReturnType<typeof setInterval> | undefined;
  let reconnectAttempt = 0;
  let connectionGeneration = 0;
  let destroyed = false;
  let seq = 0;
  let playerId = $state('');
  let resumeToken = '';
  let connectionState = $state<ConnectionState>('connecting');
  let viewportWidth = $state(0);
  let viewportHeight = $state(0);
  let localShip = $state<ShipState | undefined>();
  let remoteShips = $state<ShipState[]>([]);
  let bullets = $state<ShipBullet[]>([]);
  let asteroids = $state<ShipAsteroid[]>([]);
  let powerUps = $state<ShipPowerUp[]>([]);
  let controlZone = $state<ShipControlZone | undefined>();
  let particles = $state<Particle[]>([]);
  let explosions = $state<Explosion[]>([]);
  let mode = $state<'idle' | 'solo' | 'multiplayer'>('idle');
  let roundStatus = $state<'idle' | 'playing' | 'finished'>('idle');
  let remainingMs = $state(60_000);
  let leaderboard = $state<ShipScore[]>([]);
  let leaderboardStatus = $state<'idle' | 'loading' | 'ready' | 'error'>('idle');
  let roundSaved = $state(false);
  let winnerId = $state('');
  let winnerName = $state('');
  let gameVisible = $state(false);
  let gameDismissed = false;
  let keyboardControlArmed = false;
  let videoActive = false;
  let animationFrameId: number | undefined;
  let lastFrameAt = 0;
  let nextParticleId = 1;
  let nextExplosionId = 1;
  let audioContext: AudioContext | undefined;
  let smokeAccumulator = 0;
  let lastThrustSoundAt = 0;
  let lastEventId = 0;
  let lastSentInput = '';
  let lastPingEcho = 0;
  let previousControlScores = new Map<string, number>();
  let controlScorePulses = $state<Record<string, number>>({});
  let pingMs = $state<number | undefined>();
  const keys = new Set<string>();
  const pendingCommands: ShipCommand[] = [];

  const scale = $derived(Math.min(viewportWidth / arenaWidth, viewportHeight / arenaHeight));
  const arenaLeft = $derived((viewportWidth - arenaWidth * scale) / 2);
  const arenaTop = $derived((viewportHeight - arenaHeight * scale) / 2);
  const score = $derived(localShip?.score ?? 0);
  const controlScore = $derived(localShip?.controlScore ?? 0);
  const controlTargetScore = $derived(controlZone?.targetScore ?? 100);
  const secondsLeft = $derived(Math.max(0, Math.ceil(remainingMs / 1000)));
  const pingLabel = $derived(pingMs === undefined ? '— ms' : `${Math.round(pingMs)} ms`);
  const controlZoneStateLabel = $derived(
    controlZone?.state === 'controlled'
      ? controlZone.ownerId === playerId
        ? t.game.control.youHold
        : t.game.control.enemyHolds
      : controlZone?.state === 'contested'
        ? t.game.control.contested
        : t.game.control.empty
  );
  const spectatorWorldVisible = $derived(
    remoteShips.some((player) => player.active) ||
      bullets.some((bullet) => bullet.ownerId !== playerId) ||
      asteroids.some((asteroid) => asteroid.ownerId !== playerId)
  );
  const worldVisible = $derived(gameVisible || spectatorWorldVisible);
  const visibleBullets = $derived(gameVisible ? bullets : bullets.filter((bullet) => bullet.ownerId !== playerId));
  const visibleAsteroids = $derived(
    gameVisible ? asteroids : asteroids.filter((asteroid) => asteroid.ownerId !== playerId)
  );
  const activeBoosts = $derived(
    localShip
      ? [
          ...(localShip.shield > 0 ? [{ kind: 'shield', label: `${t.game.boosts.shield} ×${localShip.shield}` }] : []),
          ...(localShip.tripleShot ? [{ kind: 'triple-shot', label: t.game.boosts.tripleShot }] : []),
          ...(localShip.rapidFire ? [{ kind: 'rapid-fire', label: t.game.boosts.rapidFire }] : []),
          ...(localShip.overdrive ? [{ kind: 'overdrive', label: t.game.boosts.overdrive }] : [])
        ]
      : []
  );
  const controlRows = $derived(
    [localShip, ...remoteShips]
      .filter((player): player is ShipState => player !== undefined && player.state !== 'spectator')
      .sort((a, b) => {
        if ((b.controlScore ?? 0) !== (a.controlScore ?? 0)) return (b.controlScore ?? 0) - (a.controlScore ?? 0);
        if (a.id === playerId) return -1;
        if (b.id === playerId) return 1;
        return a.name.localeCompare(b.name);
      })
  );

  function screenX(value: number) {
    return arenaLeft + value * scale;
  }

  function screenY(value: number) {
    return arenaTop + value * scale;
  }

  function screenSize(value: number) {
    return Math.max(1, value * scale);
  }

  function isTextEntryTarget(target: EventTarget | null) {
    return target instanceof HTMLElement && Boolean(target.closest('input, textarea, select, [contenteditable="true"]'));
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

  function currentInput(): ShipInput {
    return {
      left: keys.has('ArrowLeft'),
      right: keys.has('ArrowRight'),
      thrust: keys.has('ArrowUp')
    };
  }

  function sendCommand(command: Omit<ShipCommand, 'seq'>) {
    const next: ShipCommand = { ...command, seq: ++seq };
    pendingCommands.push(next);
    if (pendingCommands.length > 64) pendingCommands.splice(0, pendingCommands.length - 64);
    if (socket?.readyState === WebSocket.OPEN) {
      try {
        socket.send(JSON.stringify(next));
      } catch {
        socket.close();
      }
    }
  }

  function sendHeartbeat() {
    if (socket?.readyState !== WebSocket.OPEN) return;
    sendCommand({ type: 'heartbeat', sentAtMs: Date.now() });
  }

  function sendInput(force = false) {
    const input = currentInput();
    const serialized = JSON.stringify(input);
    if (!force && serialized === lastSentInput) return;
    lastSentInput = serialized;
    sendCommand({ type: 'input', input });
    if (input.left || input.right || input.thrust) showGame(true);
  }

  function showGame(explicitUserAction = false) {
    if (explicitUserAction) {
      gameDismissed = false;
    } else if (gameDismissed) {
      return;
    }
    if (gameVisible) return;
    gameVisible = true;
    window.dispatchEvent(new CustomEvent(gameStartedEvent));
    requestAnimationLoop();
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && gameVisible) {
      exitGame();
      event.preventDefault();
      event.stopImmediatePropagation();
      return;
    }
    if (roundStatus === 'finished' && gameVisible) {
      if (event.code === 'Enter' && !event.repeat) restartGame();
      event.preventDefault();
      event.stopImmediatePropagation();
      return;
    }
    if (roundStatus === 'playing' && mode === 'solo' && event.code === 'Enter' && keyboardControlArmed && !videoActive) {
      sendCommand({ type: 'finish' });
      event.preventDefault();
      return;
    }
    if (!isShipKey(event) || isTextEntryTarget(event.target)) return;
    if (videoActive || !keyboardControlArmed) return;
    if (roundStatus === 'finished') restartGame();
    primeAudio();
    showGame(true);
    if (event.code === 'Space') {
      sendCommand({ type: 'shoot' });
      playShootSound();
    } else {
      keys.add(event.key);
      sendInput();
    }
    event.preventDefault();
  }

  function handleKeyup(event: KeyboardEvent) {
    if (roundStatus === 'finished' && gameVisible) {
      if (event.code !== 'Space') keys.delete(event.key);
      lastSentInput = JSON.stringify(currentInput());
      event.preventDefault();
      event.stopImmediatePropagation();
      return;
    }
    if (!gameVisible || !keyboardControlArmed || videoActive || !isShipKey(event)) return;
    if (event.code !== 'Space') {
      keys.delete(event.key);
      sendInput();
    }
    event.preventDefault();
  }

  function clearInput() {
    keys.clear();
    lastSentInput = '';
    sendInput(true);
  }

  function restartGame() {
    leaderboard = [];
    leaderboardStatus = 'idle';
    roundSaved = false;
    winnerId = '';
    winnerName = '';
    showGame(true);
    sendCommand({ type: 'restart' });
  }

  function exitGame() {
    keys.clear();
    lastSentInput = JSON.stringify(currentInput());
    gameDismissed = true;
    keyboardControlArmed = false;
    sendCommand({ type: 'leave' });
    gameVisible = false;
    particles = [];
    explosions = [];
    window.dispatchEvent(new CustomEvent(gameExitedEvent));
  }

  function connect() {
    if (destroyed) return;
    const generation = ++connectionGeneration;
    clearTimeout(reconnectTimer);
    connectionState = reconnectAttempt === 0 ? 'connecting' : 'disconnected';
    socket?.close();
    socket = new WebSocket(shipSocketURL(resumeToken, username));

    socket.addEventListener('open', () => {
      if (generation !== connectionGeneration) return;
      connectionState = 'connected';
      reconnectAttempt = 0;
      sendHeartbeat();
    });
    socket.addEventListener('message', (event) => {
      if (generation !== connectionGeneration) return;
      try {
        const message = JSON.parse(event.data) as ShipWelcome | ShipSnapshot;
        if (message.type === 'welcome') {
          playerId = message.playerId;
          resumeToken = message.resumeToken;
          try {
            sessionStorage.setItem(resumeTokenKey, resumeToken);
          } catch {
            // Reconnect still works for the lifetime of this page.
          }
          applySnapshot(message.snapshot);
          sendInput(true);
          return;
        }
        applySnapshot(message);
      } catch {
        // A later authoritative snapshot can recover malformed data.
      }
    });
    socket.addEventListener('close', () => {
      if (generation !== connectionGeneration || destroyed) return;
      connectionState = 'disconnected';
      pingMs = undefined;
      lastPingEcho = 0;
      keys.clear();
      lastSentInput = '';
      const delay = Math.min(8000, 400 * 2 ** reconnectAttempt);
      reconnectAttempt += 1;
      reconnectTimer = setTimeout(connect, delay);
    });
    socket.addEventListener('error', () => socket?.close());
  }

  function applySnapshot(snapshot: ShipSnapshot) {
    const previousRoundStatus = roundStatus;
    mode = snapshot.mode;
    roundStatus = snapshot.status;
    remainingMs = snapshot.remainingMs;
    winnerId = snapshot.winnerId ?? '';
    winnerName = snapshot.winnerName ?? '';
    if (snapshot.status === 'finished' && previousRoundStatus !== 'finished') {
      keys.clear();
      lastSentInput = JSON.stringify(currentInput());
    }
    updateControlScorePulses(snapshot.players);
    const authoritativeLocal = snapshot.players.find((player) => player.id === playerId);
    if (authoritativeLocal) {
      if (
        authoritativeLocal.pingEcho &&
        authoritativeLocal.pingEcho !== lastPingEcho &&
        authoritativeLocal.pingEcho <= Date.now()
      ) {
        const measuredPing = Math.min(9999, Math.max(0, Date.now() - authoritativeLocal.pingEcho));
        pingMs = measuredPing;
        lastPingEcho = authoritativeLocal.pingEcho;
      }
      seq = Math.max(seq, authoritativeLocal.ackSeq);
      while (pendingCommands[0]?.seq <= authoritativeLocal.ackSeq) pendingCommands.shift();
      if (!localShip || localShip.active !== authoritativeLocal.active) {
        localShip = { ...authoritativeLocal };
      } else {
        const dx = wrappedDelta(localShip.x, authoritativeLocal.x, arenaWidth);
        const dy = wrappedDelta(localShip.y, authoritativeLocal.y, arenaHeight);
        const distance = Math.hypot(dx, dy);
        localShip = {
          ...authoritativeLocal,
          x: distance > 120 ? authoritativeLocal.x : wrap(localShip.x + dx * 0.45, arenaWidth),
          y: distance > 120 ? authoritativeLocal.y : wrap(localShip.y + dy * 0.45, arenaHeight),
          angle: localShip.angle + angleDelta(localShip.angle, authoritativeLocal.angle) * 0.45
        };
      }
    }
    remoteShips = snapshot.players.filter((player) => player.id !== playerId).map((player) => ({ ...player }));
    bullets = (snapshot.bullets ?? []).map((bullet) => ({ ...bullet }));
    asteroids = (snapshot.asteroids ?? []).map((asteroid) => ({ ...asteroid }));
    powerUps = (snapshot.powerUps ?? []).map((powerUp) => ({ ...powerUp }));
    controlZone = snapshot.controlZone ? { ...snapshot.controlZone } : undefined;
    for (const event of snapshot.events ?? []) {
      if (event.id <= lastEventId) continue;
      lastEventId = event.id;
      handleServerEvent(event);
    }
    requestAnimationLoop();
  }

  function updateControlScorePulses(players: ShipState[]) {
    const nextScores = new Map<string, number>();
    const pulses: Record<string, number> = {};
    for (const player of players) {
      const score = player.controlScore ?? 0;
      const previousScore = previousControlScores.get(player.id);
      nextScores.set(player.id, score);
      if (mode === 'multiplayer' && roundStatus === 'playing' && previousScore !== undefined && score > previousScore) {
        pulses[player.id] = Date.now();
      }
    }
    previousControlScores = nextScores;
    if (Object.keys(pulses).length > 0) {
      controlScorePulses = { ...controlScorePulses, ...pulses };
    }
  }

  function handleServerEvent(event: NonNullable<ShipSnapshot['events']>[number]) {
    if (event.type === 'round-finished' && event.winnerId) {
      winnerId = event.winnerId;
      winnerName = event.winnerName ?? '';
      return;
    }
    if (event.type === 'round-finished' && event.victimId === playerId) {
      roundSaved = Boolean(event.saved);
      void loadLeaderboard();
      return;
    }
    if (event.type === 'power-up-collected') {
      if (event.ownerId === playerId) playPowerUpSound(event.powerUpKind);
      return;
    }
    if (event.type === 'shield-hit') {
      if (event.victimId === playerId) playShieldSound();
      return;
    }
    if (!Number.isFinite(event.x) || !Number.isFinite(event.y)) return;
    const x = event.x ?? 0;
    const y = event.y ?? 0;
    explosions = [...explosions, { id: nextExplosionId++, x, y }].slice(-6);
    window.setTimeout(() => {
      explosions = explosions.filter((item) => item.x !== x || item.y !== y);
    }, 360);
    spawnExplosionParticles(x, y);
    if (event.type === 'asteroid-destroyed') {
      playExplosionSound();
    } else {
      playShipDestroySound();
    }
  }

  async function loadLeaderboard() {
    leaderboardStatus = 'loading';
    try {
      leaderboard = await fetchShipScores();
      leaderboardStatus = 'ready';
    } catch {
      leaderboardStatus = 'error';
    }
  }

  function animate(now: number) {
    animationFrameId = undefined;
    const delta = lastFrameAt ? Math.min((now - lastFrameAt) / 16.67, 2.4) : 1;
    lastFrameAt = now;
    predictLocalShip(delta, now);
    remoteShips = remoteShips.map((player) =>
      player.active ? { ...player, x: wrap(player.x + player.vx * delta, arenaWidth), y: wrap(player.y + player.vy * delta, arenaHeight) } : player
    );
    bullets = bullets.map((bullet) => ({
      ...bullet,
      x: wrap(bullet.x + bullet.vx * delta, arenaWidth),
      y: wrap(bullet.y + bullet.vy * delta, arenaHeight)
    }));
    asteroids = asteroids.map((asteroid) => ({
      ...asteroid,
      x: wrap(asteroid.x + asteroid.vx * delta, arenaWidth),
      y: wrap(asteroid.y + asteroid.vy * delta, arenaHeight),
      angle: asteroid.angle + asteroid.spin * delta
    }));
    particles = particles
      .map((particle) => ({
        ...particle,
        x: particle.x + particle.vx * delta,
        y: particle.y + particle.vy * delta,
        vx: particle.vx * Math.pow(0.985, delta),
        vy: particle.vy * Math.pow(0.985, delta),
        angle: particle.angle + particle.spin * delta,
        age: particle.age + delta
      }))
      .filter((particle) => particle.age < particle.lifetime);
    if (worldVisible || particles.length > 0 || explosions.length > 0) requestAnimationLoop();
    else lastFrameAt = 0;
  }

  function predictLocalShip(delta: number, now: number) {
    if (!localShip?.active || connectionState !== 'connected') return;
    const input = currentInput();
    let angle = localShip.angle;
    let vx = localShip.vx;
    let vy = localShip.vy;
    const predictedThrust = localShip.overdrive ? thrust * 1.65 : thrust;
    const predictedMaxSpeed = localShip.overdrive ? maxSpeed * 1.35 : maxSpeed;
    if (input.left) angle -= turnSpeed * delta;
    if (input.right) angle += turnSpeed * delta;
    if (input.thrust) {
      vx += Math.cos(angle) * predictedThrust * delta;
      vy += Math.sin(angle) * predictedThrust * delta;
      smokeAccumulator += delta;
      while (smokeAccumulator >= 2.4) {
        smokeAccumulator -= 2.4;
        spawnSmokeParticle(localShip, angle);
      }
      if (now - lastThrustSoundAt > 130) {
        lastThrustSoundAt = now;
        playThrustSound();
      }
    } else {
      smokeAccumulator = 0;
    }
    const speed = Math.hypot(vx, vy);
    if (speed > predictedMaxSpeed) {
      vx *= predictedMaxSpeed / speed;
      vy *= predictedMaxSpeed / speed;
    }
    vx *= Math.pow(drag, delta);
    vy *= Math.pow(drag, delta);
    localShip = {
      ...localShip,
      x: wrap(localShip.x + vx * delta, arenaWidth),
      y: wrap(localShip.y + vy * delta, arenaHeight),
      vx,
      vy,
      angle,
      thrusting: input.thrust
    };
  }

  function requestAnimationLoop() {
    if (animationFrameId !== undefined || document.visibilityState === 'hidden') return;
    animationFrameId = requestAnimationFrame(animate);
  }

  function spawnSmokeParticle(ship: ShipState, angle: number) {
    const driftAngle = angle + Math.PI + (Math.random() - 0.5) * 1.15;
    particles = [
      ...particles.slice(-96),
      {
        id: nextParticleId++,
        kind: 'smoke',
        x: ship.x - Math.cos(angle) * 20,
        y: ship.y - Math.sin(angle) * 20,
        vx: ship.vx * 0.28 + Math.cos(driftAngle) * (0.65 + Math.random() * 0.75),
        vy: ship.vy * 0.28 + Math.sin(driftAngle) * (0.65 + Math.random() * 0.75),
        age: 0,
        lifetime: smokeLifetime + Math.random() * 12,
        size: 0.55 + Math.random() * 0.65,
        angle: 0,
        spin: 0
      }
    ];
  }

  function spawnExplosionParticles(x: number, y: number) {
    const burst: Particle[] = [];
    for (let index = 0; index < 19; index += 1) {
      const angle = Math.random() * Math.PI * 2;
      const kind = index < 10 ? 'spark' : index < 14 ? 'debris' : 'smoke';
      const speed = kind === 'spark' ? 1.4 + Math.random() * 3.1 : 0.4 + Math.random() * 1.6;
      burst.push({
        id: nextParticleId++,
        kind,
        x,
        y,
        vx: Math.cos(angle) * speed,
        vy: Math.sin(angle) * speed,
        age: 0,
        lifetime: kind === 'spark' ? 14 + Math.random() * 10 : 26 + Math.random() * 16,
        size: 0.5 + Math.random(),
        angle,
        spin: kind === 'debris' ? (Math.random() - 0.5) * 0.2 : 0
      });
    }
    particles = [...particles.slice(-40), ...burst];
  }

  function wrap(value: number, size: number) {
    return ((value % size) + size) % size;
  }

  function wrappedDelta(from: number, to: number, size: number) {
    let delta = to - from;
    if (delta > size / 2) delta -= size;
    if (delta < -size / 2) delta += size;
    return delta;
  }

  function angleDelta(from: number, to: number) {
    return Math.atan2(Math.sin(to - from), Math.cos(to - from));
  }

  function resize() {
    viewportWidth = window.innerWidth;
    viewportHeight = window.innerHeight;
  }

  function primeAudio() {
    audioContext ??= new AudioContext();
    if (audioContext.state === 'suspended') void audioContext.resume();
  }

  function playTone(type: OscillatorType, from: number, to: number, duration: number, volume: number) {
    const audio = audioContext;
    if (!audio) return;
    const now = audio.currentTime;
    const oscillator = audio.createOscillator();
    const gain = audio.createGain();
    oscillator.type = type;
    oscillator.frequency.setValueAtTime(from, now);
    oscillator.frequency.exponentialRampToValueAtTime(to, now + duration);
    gain.gain.setValueAtTime(volume, now);
    gain.gain.exponentialRampToValueAtTime(0.001, now + duration);
    oscillator.connect(gain).connect(audio.destination);
    oscillator.start(now);
    oscillator.stop(now + duration + 0.01);
  }

  function playShootSound() {
    playTone('square', 920, 180, 0.09, 0.075);
  }

  function playThrustSound() {
    playTone('sawtooth', 72, 54, 0.11, 0.028);
  }

  function playExplosionSound() {
    playTone('triangle', 112, 42, 0.16, 0.075);
    window.setTimeout(() => playTone('triangle', 78, 42, 0.16, 0.1), 80);
  }

  function playShipDestroySound() {
    playTone('triangle', 420, 42, 0.3, 0.12);
  }

  function playPowerUpSound(kind?: ShipPowerUp['kind']) {
    const start = kind === 'nova' ? 260 : 440;
    playTone('sine', start, start * 2.2, 0.18, 0.08);
  }

  function playShieldSound() {
    playTone('sine', 180, 720, 0.22, 0.1);
  }

  function powerUpGlyph(kind: ShipPowerUp['kind']) {
    return {
      shield: 'S',
      'triple-shot': '3',
      'rapid-fire': 'R',
      overdrive: 'F',
      nova: 'N'
    }[kind];
  }

  function powerUpLabel(kind: ShipPowerUp['kind']) {
    return {
      shield: t.game.boosts.shield,
      'triple-shot': t.game.boosts.tripleShot,
      'rapid-fire': t.game.boosts.rapidFire,
      overdrive: t.game.boosts.overdrive,
      nova: t.game.boosts.nova
    }[kind];
  }

  $effect(() => {
    const name = username.trim() || t.common.guest;
    if (connectionState === 'connected' && playerId) sendCommand({ type: 'name', name });
  });

  onMount(() => {
    try {
      resumeToken = sessionStorage.getItem(resumeTokenKey) ?? '';
    } catch {
      resumeToken = '';
    }
    resize();
    connect();
    heartbeatTimer = setInterval(sendHeartbeat, 2000);

    const markVideoActive = () => {
      videoActive = true;
      clearInput();
    };
    const markVideoInactive = () => {
      videoActive = false;
    };
    const updateKeyboardControl = (event: Event) => {
      keyboardControlArmed = Boolean((event as CustomEvent<{ armed?: boolean }>).detail?.armed);
      if (!keyboardControlArmed) clearInput();
    };
    const visibilityChange = () => {
      if (document.visibilityState === 'hidden') {
        if (animationFrameId !== undefined) cancelAnimationFrame(animationFrameId);
        animationFrameId = undefined;
        clearInput();
      } else {
        requestAnimationLoop();
      }
    };

    window.addEventListener('resize', resize);
    window.addEventListener('keydown', handleKeydown, { capture: true });
    window.addEventListener('keyup', handleKeyup);
    window.addEventListener(activeVideoEvent, markVideoActive);
    window.addEventListener(clearActiveVideoEvent, markVideoInactive);
    window.addEventListener(backgroundKeyboardControlEvent, updateKeyboardControl);
    document.addEventListener('visibilitychange', visibilityChange);

    return () => {
      destroyed = true;
      connectionGeneration += 1;
      clearTimeout(reconnectTimer);
      clearInterval(heartbeatTimer);
      if (animationFrameId !== undefined) cancelAnimationFrame(animationFrameId);
      socket?.close();
      void audioContext?.close();
      window.removeEventListener('resize', resize);
      window.removeEventListener('keydown', handleKeydown, { capture: true });
      window.removeEventListener('keyup', handleKeyup);
      window.removeEventListener(activeVideoEvent, markVideoActive);
      window.removeEventListener(clearActiveVideoEvent, markVideoInactive);
      window.removeEventListener(backgroundKeyboardControlEvent, updateKeyboardControl);
      document.removeEventListener('visibilitychange', visibilityChange);
    };
  });
</script>

{#if gameVisible && connectionState !== 'connected'}
  <div class="asteroids-connection" aria-live="polite">
    {connectionState === 'connecting' ? 'Подключение к игре...' : 'Связь потеряна, переподключаемся...'}
  </div>
{/if}

{#if gameVisible && roundStatus === 'playing'}
  {#if mode === 'multiplayer'}
    <div class="asteroids-hud asteroids-killboard" aria-live="polite">
      <div class="asteroids-hud-heading">
        <span class="asteroids-killboard-title">{t.game.control.title}</span>
        <span class="asteroids-ping"><RadioTower size={11} strokeWidth={1.8} />{pingLabel}</span>
      </div>
      <div class="asteroids-control-status">
        <span>{controlZoneStateLabel}</span>
        <strong>{controlScore}/{controlTargetScore}</strong>
      </div>
      <ol>
        {#each controlRows as player (player.id)}
          {#key controlScorePulses[player.id] ?? 0}
            <li
              class="asteroids-control-progress-row"
              class:asteroids-local-kill-row={player.id === playerId}
              class:asteroids-control-score-pulse={controlScorePulses[player.id]}
              style:--control-progress={`${Math.min(100, Math.max(0, ((player.controlScore ?? 0) / controlTargetScore) * 100))}%`}
              style:--control-progress-alpha={Math.min(1, Math.max(0.1, (player.controlScore ?? 0) / controlTargetScore))}
            >
              <span class="asteroids-control-player-name">{player.name}</span>
              <div class="asteroids-control-row-score">
                <strong>{player.controlScore ?? 0}</strong>
              </div>
            </li>
          {/key}
        {/each}
      </ol>
      {#if activeBoosts.length > 0}
        <div class="asteroids-active-boosts">
          {#each activeBoosts as boost (boost.kind)}
            <span class={`asteroids-boost-chip asteroids-boost-${boost.kind}`}>{boost.label}</span>
          {/each}
        </div>
      {/if}
    </div>
  {:else}
    <div class="asteroids-hud" aria-live="polite">
      <span>{secondsLeft}s</span>
      <strong>{score}</strong>
      <span class="asteroids-ping"><RadioTower size={11} strokeWidth={1.8} />{pingLabel}</span>
      {#if activeBoosts.length > 0}
        <div class="asteroids-active-boosts">
          {#each activeBoosts as boost (boost.kind)}
            <span class={`asteroids-boost-chip asteroids-boost-${boost.kind}`}>{boost.label}</span>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
{/if}

{#if gameVisible && roundStatus === 'finished'}
  {#if mode === 'multiplayer'}
    <section class="asteroids-leaderboard asteroids-multiplayer-result" aria-live="polite">
      <div class="asteroids-leaderboard-header">
        <span>{t.game.control.winner}</span>
        <strong class="asteroids-winner-name">{winnerId === playerId ? t.game.control.youWon : winnerName || t.common.guest}</strong>
      </div>
      <div class="asteroids-leaderboard-title">{t.game.control.final}</div>
      <ol>
        {#each controlRows as player (player.id)}
          <li class:currentScore={player.id === winnerId}>
            <span class="asteroids-result-player-name">{player.name}</span><strong>{player.controlScore ?? 0}</strong>
          </li>
        {/each}
      </ol>
      <div class="asteroids-restart">{t.game.enterToRestart}</div>
    </section>
  {:else}
    <section class="asteroids-leaderboard" aria-live="polite">
      <div class="asteroids-leaderboard-header">
        <span>{t.game.time}</span>
        <strong>{score}</strong>
      </div>
      <div class="asteroids-leaderboard-title">{t.game.leaders}</div>
      {#if !roundSaved}<p>{t.game.resultNotSaved}</p>{/if}
      {#if leaderboardStatus === 'loading'}
        <p>{t.game.loadingLeaders}</p>
      {:else if leaderboardStatus === 'error'}
        <p>{t.game.leadersUnavailable}</p>
      {:else if leaderboard.length === 0}
        <p>{t.game.noScores}</p>
      {:else}
        <ol>
          {#each leaderboard as item, index (`${item.createdAt}-${index}`)}
            <li class:currentScore={roundSaved && item.name === (username.trim() || t.common.guest) && item.score === score}>
              <span>{index + 1}. {item.name}</span><strong>{item.score}</strong>
            </li>
          {/each}
        </ol>
      {/if}
      <div class="asteroids-restart">{t.game.enterToRestart}</div>
    </section>
  {/if}
{/if}

{#if worldVisible}
  <div
    class="asteroids-arena-boundary"
    class:asteroids-arena-boundary-local={gameVisible}
    aria-hidden="true"
    style:left={`${arenaLeft}px`}
    style:top={`${arenaTop}px`}
    style:width={`${arenaWidth * scale}px`}
    style:height={`${arenaHeight * scale}px`}
  >
    <span class="asteroids-arena-corner asteroids-arena-corner-tl"></span>
    <span class="asteroids-arena-corner asteroids-arena-corner-tr"></span>
    <span class="asteroids-arena-corner asteroids-arena-corner-bl"></span>
    <span class="asteroids-arena-corner asteroids-arena-corner-br"></span>
  </div>

  {#if controlZone}
    <div
      class="asteroids-control-zone"
      class:asteroids-control-zone-held={controlZone.state === 'controlled'}
      class:asteroids-control-zone-local={controlZone.ownerId === playerId}
      class:asteroids-control-zone-contested={controlZone.state === 'contested'}
      aria-hidden="true"
      style:width={`${screenSize(controlZone.radius * 2)}px`}
      style:height={`${screenSize(controlZone.radius * 2)}px`}
      style:transform={`translate3d(${screenX(controlZone.x)}px, ${screenY(controlZone.y)}px, 0) translate(-50%, -50%)`}
    ></div>
  {/if}

  {#each visibleAsteroids as asteroid (asteroid.id)}
    <svg
      class="asteroids-rock"
      aria-hidden="true"
      viewBox="0 0 100 100"
      style:width={`${screenSize(asteroid.radius * 2)}px`}
      style:height={`${screenSize(asteroid.radius * 2)}px`}
      style:transform={`translate3d(${screenX(asteroid.x)}px, ${screenY(asteroid.y)}px, 0) translate(-50%, -50%) rotate(${asteroid.angle}rad)`}
    >
      <path class="asteroids-rock-fill" d={asteroid.path} />
      <path class="asteroids-rock-line" d={asteroid.path} />
    </svg>
  {/each}

  {#each visibleBullets as bullet (bullet.id)}
    <span
      class="asteroids-bullet"
      class:asteroids-remote-bullet={bullet.ownerId !== playerId}
      aria-hidden="true"
      style:transform={`translate3d(${screenX(bullet.x)}px, ${screenY(bullet.y)}px, 0) translate(-50%, -50%)`}
    ></span>
  {/each}

  {#each powerUps as powerUp (powerUp.id)}
    <div
      class={`asteroids-power-up asteroids-power-up-${powerUp.kind}`}
      aria-label={powerUpLabel(powerUp.kind)}
      title={powerUpLabel(powerUp.kind)}
      style:width={`${screenSize(42)}px`}
      style:height={`${screenSize(42)}px`}
      style:transform={`translate3d(${screenX(powerUp.x)}px, ${screenY(powerUp.y)}px, 0) translate(-50%, -50%)`}
    >
      <span>{powerUpGlyph(powerUp.kind)}</span>
    </div>
  {/each}

  {#each remoteShips as remote (remote.id)}
    {#if remote.active}
      <div
        class="asteroids-remote-ship"
        class:asteroids-ship-shielded={remote.shield > 0}
        aria-hidden="true"
        style:width={`${screenSize(shipWidth)}px`}
        style:height={`${screenSize(shipHeight)}px`}
        style:transform={`translate3d(${screenX(remote.x) - screenSize(shipWidth) / 2}px, ${screenY(remote.y) - screenSize(shipHeight) / 2}px, 0) rotate(${remote.angle + Math.PI / 2}rad)`}
      >
        <svg viewBox="0 0 42 54">
          <path class="remote-ship-glow" d="M21 3 39 49 21 39 3 49 21 3Z" />
          <path class="remote-ship-outline" d="M21 3 39 49 21 39 3 49 21 3Z" />
          <path class="remote-ship-window" d="M21 15 27 31 21 27 15 31 21 15Z" />
          <path class="remote-ship-flame" class:remote-ship-flame-active={remote.thrusting} d="M21 42 27 55 21 50 15 55 21 42Z" />
        </svg>
      </div>
      <span
        class="asteroids-remote-name"
        aria-hidden="true"
        style:transform={`translate3d(${screenX(remote.x)}px, ${screenY(remote.y) + screenSize(shipHeight) / 2 + 8}px, 0) translate(-50%, 0)`}
      >{remote.name}</span>
    {/if}
  {/each}

  {#if gameVisible && localShip?.active}
    <div
      class="asteroids-ship"
      class:asteroids-ship-thrusting={localShip.thrusting}
      class:asteroids-ship-shielded={localShip.shield > 0}
      aria-hidden="true"
      style:width={`${screenSize(shipWidth)}px`}
      style:height={`${screenSize(shipHeight)}px`}
      style:transform={`translate3d(${screenX(localShip.x) - screenSize(shipWidth) / 2}px, ${screenY(localShip.y) - screenSize(shipHeight) / 2}px, 0) rotate(${localShip.angle + Math.PI / 2}rad)`}
    >
      <svg viewBox="0 0 42 54">
        <path class="ship-glow" d="M21 3 39 49 21 39 3 49 21 3Z" />
        <path class="ship-outline" d="M21 3 39 49 21 39 3 49 21 3Z" />
        <path class="ship-window" d="M21 15 27 31 21 27 15 31 21 15Z" />
        <path class="ship-flame" d="M21 42 27 55 21 50 15 55 21 42Z" />
      </svg>
    </div>
  {/if}

  {#each particles as particle (particle.id)}
    <span
      class={`asteroids-particle asteroids-${particle.kind}`}
      aria-hidden="true"
      style:--particle-alpha={1 - particle.age / particle.lifetime}
      style:--particle-scale={particle.size + (particle.age / particle.lifetime) * (particle.kind === 'spark' ? 0.2 : 1.4)}
      style:transform={`translate3d(${screenX(particle.x)}px, ${screenY(particle.y)}px, 0) translate(-50%, -50%) rotate(${particle.angle}rad) scale(var(--particle-scale))`}
    ></span>
  {/each}

  {#each explosions as explosion (explosion.id)}
    <span
      class="asteroids-explosion"
      aria-hidden="true"
      style:transform={`translate3d(${screenX(explosion.x)}px, ${screenY(explosion.y)}px, 0) translate(-50%, -50%)`}
    ></span>
  {/each}
{/if}

<style>
  .asteroids-connection,
  .asteroids-hud {
    position: fixed;
    top: 1rem;
    left: 50%;
    z-index: 20;
    transform: translateX(-50%);
    border: 1px solid rgb(226 232 240 / 0.18);
    border-radius: 0.5rem;
    background: rgb(2 6 23 / 0.78);
    padding: 0.55rem 0.8rem;
    color: rgb(226 232 240);
    font-size: 0.9rem;
    pointer-events: none;
  }

  .asteroids-active-boosts {
    position: absolute;
    top: calc(100% + 0.45rem);
    left: 50%;
    display: flex;
    width: max-content;
    max-width: min(32rem, calc(100vw - 2rem));
    flex-wrap: wrap;
    justify-content: center;
    gap: 0.3rem;
    transform: translateX(-50%);
  }

  .asteroids-boost-chip {
    border: 1px solid rgb(125 211 252 / 0.28);
    border-radius: 999px;
    background: rgb(2 6 23 / 0.78);
    padding: 0.2rem 0.48rem;
    color: rgb(186 230 253);
    font-size: 0.65rem;
    white-space: nowrap;
  }

  .asteroids-hud {
    display: flex;
    min-width: 9rem;
    align-items: center;
    justify-content: space-between;
    gap: 1.25rem;
  }

  .asteroids-hud strong,
  .asteroids-leaderboard strong {
    color: rgb(253 224 71);
  }

  .asteroids-killboard {
    min-width: min(16.5rem, calc(100vw - 2rem));
    flex-direction: column;
    align-items: stretch;
    gap: 0.28rem;
    padding: 0.42rem 0.58rem;
  }

  .asteroids-killboard-title,
  .asteroids-leaderboard-title,
  .asteroids-restart,
  .asteroids-leaderboard p {
    color: rgb(148 163 184);
    font-size: 0.82rem;
  }

  .asteroids-hud-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
  }

  .asteroids-killboard .asteroids-hud-heading {
    gap: 0.7rem;
    line-height: 1;
  }

  .asteroids-killboard .asteroids-killboard-title {
    font-size: 0.74rem;
  }

  .asteroids-killboard .asteroids-ping {
    font-size: 0.62rem;
  }

  .asteroids-control-status {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    border-radius: 0.35rem;
    background: rgb(15 23 42 / 0.72);
    padding: 0.26rem 0.42rem;
    color: rgb(203 213 225);
    font-size: 0.72rem;
  }

  .asteroids-control-status strong {
    color: rgb(253 224 71);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  .asteroids-ping {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    color: rgb(148 163 184 / 0.88);
    font-size: 0.68rem;
    font-variant-numeric: tabular-nums;
    letter-spacing: 0.02em;
    white-space: nowrap;
  }

  .asteroids-killboard ol,
  .asteroids-leaderboard ol {
    display: grid;
    gap: 0.3rem;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .asteroids-killboard ol {
    gap: 0.22rem;
  }

  .asteroids-killboard li,
  .asteroids-leaderboard li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    border-radius: 0.35rem;
    background: rgb(15 23 42 / 0.65);
    padding: 0.4rem 0.5rem;
  }

  .asteroids-killboard li {
    min-height: 1.7rem;
    gap: 0.65rem;
    padding: 0.24rem 0.42rem;
    font-size: 0.78rem;
  }

  .asteroids-control-progress-row {
    position: relative;
    overflow: hidden;
    background:
      linear-gradient(90deg, rgb(20 184 166 / 0.34) 0 var(--control-progress), transparent var(--control-progress) 100%),
      rgb(15 23 42 / 0.65) !important;
  }

  .asteroids-control-progress-row::before {
    position: absolute;
    inset: 0;
    border-radius: inherit;
    background: linear-gradient(90deg, rgb(45 212 191 / 0.16), rgb(253 224 71 / 0.13));
    opacity: var(--control-progress-alpha);
    content: '';
    pointer-events: none;
  }

  .asteroids-control-player-name,
  .asteroids-control-row-score {
    position: relative;
    z-index: 1;
  }

  .asteroids-control-player-name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .asteroids-control-row-score {
    display: flex;
    align-items: center;
    font-variant-numeric: tabular-nums;
  }

  .asteroids-control-row-score strong {
    color: rgb(253 224 71);
    font-size: 0.9rem;
    text-align: right;
    min-width: 2.35rem;
  }

  .asteroids-control-score-pulse {
    animation: control-score-pulse 520ms ease-out;
  }

  .asteroids-local-kill-row {
    color: rgb(153 246 228);
    box-shadow: inset 0 0 0 1px rgb(45 212 191 / 0.22);
  }

  .asteroids-leaderboard li.currentScore {
    background: rgb(20 184 166 / 0.16) !important;
    color: rgb(153 246 228);
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
    background: rgb(2 6 23 / 0.9);
    padding: 1rem;
    color: rgb(226 232 240);
    box-shadow: 0 18px 48px rgb(0 0 0 / 0.32);
  }

  .asteroids-leaderboard-header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    border-bottom: 1px solid rgb(226 232 240 / 0.14);
    padding-bottom: 0.75rem;
  }

  .asteroids-leaderboard-header strong {
    font-size: 2rem;
  }

  .asteroids-multiplayer-result .asteroids-leaderboard-header {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 0.2rem;
  }

  .asteroids-multiplayer-result .asteroids-leaderboard-header span {
    color: rgb(203 213 225);
    font-size: 1rem;
  }

  .asteroids-winner-name {
    display: block;
    overflow-wrap: anywhere;
    color: rgb(253 224 71);
    font-size: clamp(1.5rem, 7vw, 2rem) !important;
    line-height: 1.08;
  }

  .asteroids-result-player-name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .asteroids-multiplayer-result li {
    min-width: 0;
  }

  .asteroids-multiplayer-result li strong {
    flex: 0 0 auto;
    min-width: 2.4rem;
    text-align: right;
  }

  .asteroids-leaderboard-title {
    margin: 0.9rem 0 0.65rem;
    text-transform: uppercase;
  }

  .asteroids-restart {
    margin-top: 0.85rem;
    text-align: center;
  }

  .asteroids-ship,
  .asteroids-remote-ship,
  .asteroids-arena-boundary,
  .asteroids-control-zone,
  .asteroids-rock,
  .asteroids-bullet,
  .asteroids-power-up,
  .asteroids-particle,
  .asteroids-explosion,
  .asteroids-remote-name {
    position: fixed;
    top: 0;
    left: 0;
    z-index: 0;
    pointer-events: none;
    will-change: transform;
  }

  .asteroids-arena-boundary {
    box-sizing: border-box;
    border: 1px solid rgb(125 211 252 / 0.12);
    box-shadow:
      inset 0 0 2rem rgb(34 211 238 / 0.025),
      0 0 0.75rem rgb(125 211 252 / 0.035);
    opacity: 0.62;
  }

  .asteroids-arena-boundary-local {
    border-color: rgb(125 211 252 / 0.2);
    box-shadow:
      inset 0 0 2.5rem rgb(34 211 238 / 0.04),
      0 0 1rem rgb(125 211 252 / 0.055);
    opacity: 1;
  }

  .asteroids-control-zone {
    box-sizing: border-box;
    border: 2px solid rgb(148 163 184 / 0.22);
    border-radius: 999px;
    background:
      radial-gradient(circle, rgb(148 163 184 / 0.06) 0 48%, transparent 49%),
      repeating-radial-gradient(circle, transparent 0 1.2rem, rgb(148 163 184 / 0.08) 1.25rem 1.32rem);
    box-shadow: inset 0 0 2.5rem rgb(148 163 184 / 0.06), 0 0 1.5rem rgb(148 163 184 / 0.08);
  }

  .asteroids-control-zone::before {
    position: absolute;
    inset: -7%;
    border: 2px solid rgb(148 163 184 / 0.16);
    border-radius: inherit;
    box-shadow: 0 0 1.4rem rgb(148 163 184 / 0.12);
    content: '';
    animation: control-zone-breathe 2.8s ease-in-out infinite;
  }

  .asteroids-control-zone-held {
    border-color: rgb(253 224 71 / 0.52);
    background:
      radial-gradient(circle, rgb(253 224 71 / 0.12) 0 48%, transparent 49%),
      repeating-radial-gradient(circle, transparent 0 1.2rem, rgb(253 224 71 / 0.1) 1.25rem 1.32rem);
    box-shadow: inset 0 0 3rem rgb(253 224 71 / 0.1), 0 0 1.8rem rgb(253 224 71 / 0.14);
  }

  .asteroids-control-zone-held::before {
    border-color: rgb(253 224 71 / 0.3);
    box-shadow: 0 0 1.8rem rgb(253 224 71 / 0.2);
    animation-duration: 1.7s;
  }

  .asteroids-control-zone-local {
    border-color: rgb(45 212 191 / 0.7);
    box-shadow: inset 0 0 3rem rgb(20 184 166 / 0.14), 0 0 1.9rem rgb(45 212 191 / 0.2);
  }

  .asteroids-control-zone-local::before {
    border-color: rgb(45 212 191 / 0.38);
    box-shadow: 0 0 2rem rgb(45 212 191 / 0.26);
  }

  .asteroids-control-zone-contested {
    border-color: rgb(244 114 182 / 0.66);
    background:
      radial-gradient(circle, rgb(244 114 182 / 0.12) 0 48%, transparent 49%),
      repeating-radial-gradient(circle, transparent 0 1.2rem, rgb(244 114 182 / 0.1) 1.25rem 1.32rem);
    box-shadow: inset 0 0 3rem rgb(244 114 182 / 0.1), 0 0 1.8rem rgb(244 114 182 / 0.16);
  }

  .asteroids-control-zone-contested::before {
    border-color: rgb(244 114 182 / 0.42);
    box-shadow: 0 0 2.2rem rgb(244 114 182 / 0.26);
    animation: control-zone-contested 760ms ease-in-out infinite;
  }

  .asteroids-arena-corner {
    position: absolute;
    width: 1rem;
    height: 1rem;
    border-color: rgb(165 243 252 / 0.42);
    pointer-events: none;
  }

  .asteroids-arena-corner-tl {
    top: -1px;
    left: -1px;
    border-top: 2px solid;
    border-left: 2px solid;
  }

  .asteroids-arena-corner-tr {
    top: -1px;
    right: -1px;
    border-top: 2px solid;
    border-right: 2px solid;
  }

  .asteroids-arena-corner-bl {
    bottom: -1px;
    left: -1px;
    border-bottom: 2px solid;
    border-left: 2px solid;
  }

  .asteroids-arena-corner-br {
    right: -1px;
    bottom: -1px;
    border-right: 2px solid;
    border-bottom: 2px solid;
  }

  .asteroids-ship,
  .asteroids-remote-ship {
    transform-origin: 50% 50%;
    filter: drop-shadow(0 0 10px rgb(125 211 252 / 0.5));
  }

  .asteroids-ship-shielded::before {
    position: absolute;
    inset: -28%;
    border: 2px solid rgb(96 165 250 / 0.72);
    border-radius: 999px;
    box-shadow: inset 0 0 1rem rgb(59 130 246 / 0.18), 0 0 1rem rgb(96 165 250 / 0.38);
    content: '';
    animation: shield-pulse 1.2s ease-in-out infinite alternate;
  }

  .asteroids-remote-ship {
    filter: drop-shadow(0 0 10px rgb(244 114 182 / 0.42));
    opacity: 0.86;
  }

  .asteroids-ship svg,
  .asteroids-remote-ship svg {
    display: block;
    width: 100%;
    height: 100%;
    overflow: visible;
  }

  .ship-glow,
  .remote-ship-glow {
    fill: rgb(125 211 252 / 0.12);
    stroke: rgb(125 211 252 / 0.26);
    stroke-width: 7;
  }

  .remote-ship-glow {
    fill: rgb(244 114 182 / 0.12);
    stroke: rgb(244 114 182 / 0.26);
  }

  .ship-outline,
  .remote-ship-outline {
    fill: rgb(3 7 18 / 0.5);
    stroke: rgb(255 255 255 / 0.88);
    stroke-linejoin: round;
    stroke-width: 2.25;
  }

  .remote-ship-outline {
    stroke: rgb(251 207 232 / 0.88);
  }

  .ship-window,
  .remote-ship-window {
    fill: rgb(34 211 238 / 0.28);
    stroke: rgb(165 243 252 / 0.74);
    stroke-linejoin: round;
    stroke-width: 1.4;
  }

  .remote-ship-window {
    fill: rgb(244 114 182 / 0.26);
    stroke: rgb(251 207 232 / 0.72);
  }

  .ship-flame,
  .remote-ship-flame {
    fill: transparent;
    stroke: transparent;
    transform-origin: 50% 84%;
  }

  .asteroids-ship-thrusting .ship-flame,
  .remote-ship-flame-active {
    animation: ship-thrust 120ms steps(2, end) infinite;
    fill: rgb(251 146 60 / 0.84);
    stroke: rgb(254 240 138 / 0.8);
  }

  .asteroids-rock {
    overflow: visible;
    filter: drop-shadow(0 0 12px rgb(148 163 184 / 0.28));
  }

  .asteroids-rock-fill {
    fill: rgb(15 23 42 / 0.72);
  }

  .asteroids-rock-line {
    fill: none;
    stroke: rgb(226 232 240 / 0.72);
    stroke-linejoin: round;
    stroke-width: 4;
  }

  .asteroids-bullet {
    width: 0.42rem;
    height: 0.42rem;
    border-radius: 999px;
    background: rgb(253 224 71 / 0.96);
    box-shadow: 0 0 6px rgb(253 224 71 / 0.92), 0 0 14px rgb(34 211 238 / 0.5);
  }

  .asteroids-remote-bullet {
    background: rgb(244 114 182 / 0.96);
  }

  .asteroids-power-up {
    display: grid;
    place-items: center;
    border: 2px solid rgb(255 255 255 / 0.82);
    border-radius: 0.7rem;
    background: rgb(2 6 23 / 0.86);
    color: white;
    font-size: clamp(0.65rem, 1.2vw, 0.95rem);
    font-weight: 800;
    line-height: 1;
    animation: power-up-pulse 1.1s ease-in-out infinite alternate;
  }

  .asteroids-power-up-shield {
    border-color: rgb(96 165 250);
    box-shadow: 0 0 1rem rgb(59 130 246 / 0.62);
    color: rgb(191 219 254);
  }

  .asteroids-power-up-triple-shot {
    border-color: rgb(192 132 252);
    box-shadow: 0 0 1rem rgb(168 85 247 / 0.62);
    color: rgb(233 213 255);
  }

  .asteroids-power-up-rapid-fire {
    border-color: rgb(250 204 21);
    box-shadow: 0 0 1rem rgb(234 179 8 / 0.62);
    color: rgb(254 240 138);
  }

  .asteroids-power-up-overdrive {
    border-color: rgb(251 146 60);
    box-shadow: 0 0 1rem rgb(249 115 22 / 0.62);
    color: rgb(254 215 170);
  }

  .asteroids-power-up-nova {
    border-color: rgb(45 212 191);
    box-shadow: 0 0 1.1rem rgb(20 184 166 / 0.72);
    color: rgb(153 246 228);
  }

  .asteroids-remote-name {
    max-width: 9rem;
    overflow: hidden;
    border-radius: 999px;
    background: rgb(0 0 0 / 0.35);
    padding: 0.12rem 0.42rem;
    color: rgb(251 207 232);
    font-size: 0.62rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .asteroids-particle {
    opacity: var(--particle-alpha);
  }

  .asteroids-smoke {
    width: 0.8rem;
    height: 0.8rem;
    border-radius: 999px;
    background: rgb(148 163 184 / 0.28);
  }

  .asteroids-spark {
    width: 0.45rem;
    height: 0.45rem;
    border-radius: 999px;
    background: rgb(253 224 71);
    box-shadow: 0 0 8px rgb(253 224 71 / 0.62);
  }

  .asteroids-debris {
    width: 0.75rem;
    height: 0.36rem;
    border-radius: 0.16rem;
    background: rgb(226 232 240 / 0.62);
  }

  .asteroids-explosion {
    width: 4.8rem;
    height: 4.8rem;
    border: 2px solid rgb(253 224 71 / 0.86);
    border-radius: 999px;
    background: radial-gradient(circle, rgb(254 240 138 / 0.65) 0 18%, rgb(251 146 60 / 0.34) 19% 42%, transparent 58%);
    animation: asteroid-pop 260ms ease-out forwards;
  }

  @keyframes ship-thrust {
    from { transform: scaleY(0.72); opacity: 0.74; }
    to { transform: scaleY(1.08); opacity: 1; }
  }

  @keyframes asteroid-pop {
    from { opacity: 0.95; }
    to { opacity: 0; }
  }

  @keyframes power-up-pulse {
    from { scale: 0.92; opacity: 0.78; }
    to { scale: 1.06; opacity: 1; }
  }

  @keyframes control-zone-breathe {
    0%, 100% {
      opacity: 0.52;
      transform: scale(0.96);
    }
    50% {
      opacity: 0.98;
      transform: scale(1.04);
    }
  }

  @keyframes control-zone-contested {
    0%, 100% {
      opacity: 0.58;
      transform: scale(0.98);
    }
    50% {
      opacity: 1;
      transform: scale(1.08);
    }
  }

  @keyframes control-score-pulse {
    0% {
      background: rgb(20 184 166 / 0.32);
      box-shadow: 0 0 0 rgb(45 212 191 / 0);
      transform: scale(1);
    }
    38% {
      background: rgb(20 184 166 / 0.34);
      box-shadow: 0 0 1.1rem rgb(45 212 191 / 0.34);
      transform: scale(1.035);
    }
    100% {
      box-shadow: 0 0 0 rgb(45 212 191 / 0);
      transform: scale(1);
    }
  }

  @keyframes shield-pulse {
    from { opacity: 0.5; scale: 0.94; }
    to { opacity: 0.9; scale: 1.04; }
  }

  @media (prefers-reduced-motion: reduce) {
    .ship-flame,
    .remote-ship-flame {
      animation: none;
    }

    .asteroids-power-up,
    .asteroids-ship-shielded::before,
    .asteroids-control-zone::before,
    .asteroids-control-score-pulse {
      animation: none;
    }
  }
</style>
