<script lang="ts">
  import { onMount } from "svelte";

  let {
    mode = "cosmos",
    animated = true
  }: {
    mode?: "cosmos" | "daylight" | "toxic" | "aurora";
    animated?: boolean;
  } = $props();

  const idleFrameMs = 1000 / 24;
  const scrollFrameMs = 1000 / 60;
  const scrollBoostMs = 220;
  const maxCanvasPixels = 1_000_000;
  const particleDensityPixels = 16000;
  const maxParticles = 72;
  const maxStreaks = 2;

  let canvas: HTMLCanvasElement;
  let ctx: CanvasRenderingContext2D | null;

  type Particle = {
    x: number;
    y: number;
    radius: number;
    vx: number;
    vy: number;
    alpha: number;
    pulseSpeed: number;
    twinkleDepth: number;
    glow: boolean;
    phase: number;
  };

  type Streak = {
    x: number;
    y: number;
    vx: number;
    vy: number;
    age: number;
    lifetime: number;
    length: number;
    alpha: number;
  };

  let particles: Particle[] = [];
  let streaks: Streak[] = [];
  let animationFrameId: number | undefined = undefined;
  let frameTimerId: number | undefined = undefined;
  let width = 0;
  let height = 0;
  let renderScale = 1;
  let time = 0;
  let lastFrameAt = 0;
  let lastScrollY = 0;
  let scrollOffset = 0;
  let targetScrollOffset = 0;
  let scrollBoostUntil = 0;
  let reducedMotion = false;
  let motionMediaQuery: MediaQueryList | undefined = undefined;

  function resize() {
    if (!canvas) return;
    width = window.innerWidth;
    height = window.innerHeight;
    renderScale = selectRenderScale(width, height);
    canvas.width = Math.max(1, Math.round(width * renderScale));
    canvas.height = Math.max(1, Math.round(height * renderScale));
    ctx?.setTransform(renderScale, 0, 0, renderScale, 0, 0);
    initParticles();
    drawParticles();
  }

  function selectRenderScale(nextWidth: number, nextHeight: number) {
    const pixels = nextWidth * nextHeight;
    if (pixels <= maxCanvasPixels) return 1;
    return Math.sqrt(maxCanvasPixels / pixels);
  }

  $effect(() => {
    mode;
    if (!canvas || width === 0 || height === 0) return;
    initParticles();
    drawParticles();
  });

  $effect(() => {
    animated;
    if (!canvas || width === 0 || height === 0) return;
    if (!animated) {
      cancelScheduledFrame();
      drawParticles();
      return;
    }

    lastFrameAt = 0;
    lastScrollY = window.scrollY;
    requestNextFrame();
  });

  function handleScroll() {
    if (!animated || reducedMotion) return;
    const currentScrollY = window.scrollY;
    const delta = currentScrollY - lastScrollY;
    lastScrollY = currentScrollY;
    targetScrollOffset += Math.max(-22, Math.min(22, -delta * 0.07));
    scrollBoostUntil = performance.now() + scrollBoostMs;
    requestImmediateFrame();
  }

  function initParticles() {
    particles = [];
    streaks = [];
    if (mode !== "cosmos") return;

    const numParticles = Math.min(Math.floor((width * height) / particleDensityPixels), maxParticles);
    for (let i = 0; i < numParticles; i++) {
      particles.push(createParticle(true));
    }
  }

  function createParticle(randomY = false): Particle {
    return {
      x: Math.random() * width,
      y: randomY ? Math.random() * height : height + 10,
      radius: Math.random() * 2 + 1, // Increased radius
      vx: (Math.random() - 0.5) * 0.18,
      vy: -(Math.random() * 0.28 + 0.1),
      alpha: Math.random() * 0.5 + 0.2, // Increased alpha
      pulseSpeed: Math.random() * 0.035 + 0.012,
      twinkleDepth: Math.random() * 0.32 + 0.18,
      glow: Math.random() > 0.8,
      phase: Math.random() * Math.PI * 2
    };
  }

  function createStreak(): Streak {
    return {
      x: Math.random() * width,
      y: Math.random() * height * 0.62,
      vx: -(Math.random() * 5.5 + 3.4),
      vy: Math.random() * 2.2 + 1.2,
      age: 0,
      lifetime: Math.random() * 18 + 20,
      length: Math.random() * 72 + 56,
      alpha: Math.random() * 0.22 + 0.18
    };
  }

  function drawParticles() {
    if (!ctx) return;
    if (mode === "daylight") {
      drawDaylightBase();
      drawDaylightGeometry();
      return;
    }

    if (mode === "toxic") {
      drawToxicBase();
      drawToxicGeometry();
      return;
    }

    if (mode === "aurora") {
      drawAuroraBase();
      drawAuroraBands();
      return;
    }

    ctx.fillStyle = "rgb(0, 0, 0)";
    ctx.fillRect(0, 0, width, height);

    for (let i = 0; i < particles.length; i++) {
      const p = particles[i];
      const twinkle = 1 - p.twinkleDepth + ((Math.sin(time * p.pulseSpeed + p.phase) + 1) / 2) * p.twinkleDepth;
      const alphaClamped = Math.max(0.05, Math.min(0.86, p.alpha * twinkle));
      const radius = p.radius * (0.82 + twinkle * 0.28);
      const drawY = wrapParticleY(p.y + scrollOffset);

      ctx.beginPath();
      ctx.arc(p.x, drawY, radius, 0, Math.PI * 2);

      // Removed expensive shadowBlur/shadowColor
      if (p.glow) {
        ctx.fillStyle = `rgba(255, 255, 255, ${alphaClamped})`;
      } else {
        ctx.fillStyle = `rgba(255, 255, 255, ${alphaClamped * 0.6})`;
      }

      ctx.fill();

    }

    drawStreaks();
  }

  function drawStreaks() {
    if (!ctx || streaks.length === 0) return;
    ctx.save();
    ctx.lineCap = "round";
    for (let i = 0; i < streaks.length; i++) {
      const streak = streaks[i];
      const progress = streak.age / streak.lifetime;
      const alpha = streak.alpha * Math.sin(Math.PI * Math.min(1, progress));
      const speed = Math.hypot(streak.vx, streak.vy) || 1;
      const tailX = streak.x - (streak.vx / speed) * streak.length;
      const drawY = streak.y + scrollOffset;
      const tailY = drawY - (streak.vy / speed) * streak.length;
      const gradient = ctx.createLinearGradient(streak.x, drawY, tailX, tailY);
      gradient.addColorStop(0, `rgba(255, 255, 255, ${alpha})`);
      gradient.addColorStop(0.35, `rgba(168, 202, 255, ${alpha * 0.45})`);
      gradient.addColorStop(1, "rgba(168, 202, 255, 0)");
      ctx.strokeStyle = gradient;
      ctx.lineWidth = 1.4;
      ctx.beginPath();
      ctx.moveTo(streak.x, drawY);
      ctx.lineTo(tailX, tailY);
      ctx.stroke();
    }
    ctx.restore();
  }

  function drawDaylightBase() {
    if (!ctx) return;
    const drift = time * 0.0025;
    const sky = ctx.createLinearGradient(0, 0, width, height);
    sky.addColorStop(0, "rgb(82, 89, 96)");
    sky.addColorStop(0.45, "rgb(65, 72, 79)");
    sky.addColorStop(1, "rgb(48, 55, 63)");
    ctx.fillStyle = sky;
    ctx.fillRect(0, 0, width, height);

    drawGlow(width * (0.16 + Math.sin(drift) * 0.04), height * 0.14, Math.max(width, height) * 0.5, "rgba(100, 116, 139, 0.32)");
    drawGlow(width * (0.86 + Math.cos(drift * 0.8) * 0.03), height * 0.22, Math.max(width, height) * 0.44, "rgba(30, 41, 59, 0.34)");
    drawGlow(width * 0.62, height * (0.88 + Math.sin(drift * 1.2) * 0.025), Math.max(width, height) * 0.52, "rgba(148, 163, 184, 0.18)");

    ctx.save();
    ctx.globalAlpha = 0.2;
    ctx.strokeStyle = "rgba(203, 213, 225, 0.12)";
    ctx.lineWidth = 1;
    for (let i = 0; i < 5; i++) {
      const y = height * (0.22 + i * 0.16) + Math.sin(time * 0.006 + i) * 10;
      ctx.beginPath();
      ctx.moveTo(-40, y);
      ctx.bezierCurveTo(width * 0.25, y - 34, width * 0.55, y + 36, width + 40, y - 18);
      ctx.stroke();
    }
    ctx.restore();
  }

  function drawDaylightGeometry() {
    if (!ctx) return;
    const grid = 96;
    const drift = (time * 0.18) % grid;
    const lineColor = "rgba(226, 232, 240, 0.14)";
    const accentColor = "rgba(148, 163, 184, 0.18)";

    ctx.save();
    ctx.lineWidth = 1;
    ctx.strokeStyle = lineColor;
    ctx.beginPath();
    for (let x = -grid * 2 + drift; x < width + grid * 2; x += grid) {
      ctx.moveTo(x, -grid);
      ctx.lineTo(x + height + grid, height + grid);
    }
    for (let x = -grid * 2 - drift; x < width + grid * 2; x += grid) {
      ctx.moveTo(x, height + grid);
      ctx.lineTo(x + height + grid, -grid);
    }
    ctx.stroke();

    ctx.strokeStyle = accentColor;
    ctx.lineWidth = 1.25;
    drawPolygon(width * 0.18, height * 0.2, Math.min(width, height) * 0.18, 6, time * 0.0012);
    drawPolygon(width * 0.78, height * 0.24, Math.min(width, height) * 0.16, 5, -time * 0.001);
    drawPolygon(width * 0.58, height * 0.78, Math.min(width, height) * 0.22, 7, time * 0.0008);
    ctx.restore();
  }

  function drawToxicBase() {
    if (!ctx) return;
    const drift = time * 0.004;
    const haze = ctx.createLinearGradient(0, 0, width, height);
    haze.addColorStop(0, "rgb(42, 0, 68)");
    haze.addColorStop(0.42, "rgb(8, 22, 16)");
    haze.addColorStop(1, "rgb(70, 0, 52)");
    ctx.fillStyle = haze;
    ctx.fillRect(0, 0, width, height);

    drawGlow(width * (0.12 + Math.sin(drift) * 0.035), height * 0.18, Math.max(width, height) * 0.45, "rgba(236, 72, 153, 0.58)");
    drawGlow(width * (0.82 + Math.cos(drift * 0.9) * 0.04), height * 0.18, Math.max(width, height) * 0.42, "rgba(163, 230, 53, 0.54)");
    drawGlow(width * 0.52, height * (0.82 + Math.sin(drift * 1.3) * 0.03), Math.max(width, height) * 0.5, "rgba(34, 211, 238, 0.42)");
    drawGlow(width * (0.46 + Math.cos(drift * 1.6) * 0.04), height * 0.44, Math.max(width, height) * 0.34, "rgba(250, 204, 21, 0.24)");
  }

  function drawToxicGeometry() {
    if (!ctx) return;
    const grid = 54;
    const drift = (time * 0.42) % grid;

    ctx.save();
    ctx.lineWidth = 1;
    ctx.strokeStyle = "rgba(217, 70, 239, 0.22)";
    ctx.beginPath();
    for (let x = -grid + drift; x < width + grid; x += grid) {
      ctx.moveTo(x, 0);
      ctx.lineTo(x + height * 0.26, height);
    }
    for (let y = -grid - drift; y < height + grid; y += grid) {
      ctx.moveTo(0, y);
      ctx.lineTo(width, y + width * 0.08);
    }
    ctx.stroke();

    ctx.strokeStyle = "rgba(190, 242, 100, 0.24)";
    ctx.lineWidth = 1.4;
    drawPolygon(width * 0.2, height * 0.22, Math.min(width, height) * 0.16, 3, time * 0.002);
    drawPolygon(width * 0.76, height * 0.3, Math.min(width, height) * 0.14, 6, -time * 0.0016);
    drawPolygon(width * 0.58, height * 0.76, Math.min(width, height) * 0.2, 5, time * 0.0012);

    ctx.globalAlpha = 0.72;
    ctx.strokeStyle = "rgba(34, 211, 238, 0.22)";
    ctx.lineWidth = 2;
    for (let i = 0; i < 4; i++) {
      const y = height * (0.18 + i * 0.2) + Math.sin(time * 0.018 + i) * 14;
      ctx.beginPath();
      ctx.moveTo(-60, y);
      ctx.bezierCurveTo(width * 0.26, y + 46, width * 0.58, y - 52, width + 60, y + 24);
      ctx.stroke();
    }
    ctx.restore();
  }

  function drawAuroraBase() {
    if (!ctx) return;
    const drift = time * 0.003;
    const sky = ctx.createLinearGradient(0, 0, width, height);
    sky.addColorStop(0, "rgb(4, 20, 17)");
    sky.addColorStop(0.5, "rgb(16, 27, 31)");
    sky.addColorStop(1, "rgb(24, 12, 35)");
    ctx.fillStyle = sky;
    ctx.fillRect(0, 0, width, height);

    drawGlow(width * (0.18 + Math.sin(drift) * 0.04), height * 0.18, Math.max(width, height) * 0.48, "rgba(20, 184, 166, 0.38)");
    drawGlow(width * (0.78 + Math.cos(drift * 0.7) * 0.04), height * 0.22, Math.max(width, height) * 0.42, "rgba(167, 139, 250, 0.3)");
    drawGlow(width * 0.48, height * (0.88 + Math.sin(drift * 1.1) * 0.03), Math.max(width, height) * 0.52, "rgba(74, 222, 128, 0.2)");
  }

  function drawAuroraBands() {
    if (!ctx) return;
    ctx.save();
    ctx.globalCompositeOperation = "screen";
    for (let band = 0; band < 4; band++) {
      const baseY = height * (0.16 + band * 0.115);
      const amplitude = height * (0.04 + band * 0.012);
      const phase = time * (0.008 + band * 0.002) + band * 1.7;
      const gradient = ctx.createLinearGradient(0, baseY - amplitude * 2, 0, baseY + amplitude * 4);
      gradient.addColorStop(0, "rgba(45, 212, 191, 0)");
      gradient.addColorStop(0.36, band % 2 === 0 ? "rgba(94, 234, 212, 0.2)" : "rgba(167, 139, 250, 0.17)");
      gradient.addColorStop(0.64, band % 2 === 0 ? "rgba(134, 239, 172, 0.16)" : "rgba(45, 212, 191, 0.14)");
      gradient.addColorStop(1, "rgba(45, 212, 191, 0)");

      ctx.beginPath();
      ctx.moveTo(-40, baseY + Math.sin(phase) * amplitude);
      for (let x = -40; x <= width + 80; x += 80) {
        const y = baseY + Math.sin(x * 0.006 + phase) * amplitude + Math.sin(x * 0.013 - phase * 0.7) * amplitude * 0.42;
        ctx.lineTo(x, y);
      }
      for (let x = width + 80; x >= -40; x -= 80) {
        const y = baseY + amplitude * (2.3 + band * 0.2) + Math.sin(x * 0.005 + phase + 2) * amplitude * 0.55;
        ctx.lineTo(x, y);
      }
      ctx.closePath();
      ctx.fillStyle = gradient;
      ctx.fill();
    }
    ctx.restore();

    ctx.save();
    ctx.strokeStyle = "rgba(187, 247, 208, 0.1)";
    ctx.lineWidth = 1;
    const drift = (time * 0.24) % 88;
    for (let x = -88 + drift; x < width + 88; x += 88) {
      ctx.beginPath();
      ctx.moveTo(x, 0);
      ctx.lineTo(x + height * 0.18, height);
      ctx.stroke();
    }
    ctx.restore();
  }

  function drawPolygon(x: number, y: number, radius: number, sides: number, rotation: number) {
    if (!ctx) return;
    ctx.beginPath();
    for (let i = 0; i <= sides; i++) {
      const angle = rotation + (i / sides) * Math.PI * 2;
      const px = x + Math.cos(angle) * radius;
      const py = y + Math.sin(angle) * radius;
      if (i === 0) {
        ctx.moveTo(px, py);
      } else {
        ctx.lineTo(px, py);
      }
    }
    ctx.stroke();
  }

  function drawGlow(x: number, y: number, radius: number, color: string) {
    if (!ctx) return;
    const gradient = ctx.createRadialGradient(x, y, 0, x, y, radius);
    gradient.addColorStop(0, color);
    gradient.addColorStop(1, "rgba(255, 255, 255, 0)");
    ctx.fillStyle = gradient;
    ctx.fillRect(0, 0, width, height);
  }

  function animate(now: number) {
    animationFrameId = undefined;
    if (!ctx) {
      return;
    }

    const delta = lastFrameAt > 0 ? Math.min((now - lastFrameAt) / 16.67, 4) : 1;
    lastFrameAt = now;
    time += delta;
    scrollOffset += (targetScrollOffset - scrollOffset) * Math.min(1, 0.2 * delta);
    targetScrollOffset *= Math.pow(0.9, delta);

    for (let i = 0; i < particles.length; i++) {
      const p = particles[i];
      p.x += (p.vx + Math.sin(time * p.pulseSpeed + p.phase) * 0.08) * delta;
      p.y += p.vy * delta;

      if (p.y < -10) {
        p.y = height + 10;
        p.x = Math.random() * width;
      } else if (p.y > height + 10) {
        p.y = -10;
        p.x = Math.random() * width;
      }

      if (p.x < -10) p.x = width + 10;
      if (p.x > width + 10) p.x = -10;
    }

    streaks = streaks
      .map((streak) => ({
        ...streak,
        x: streak.x + streak.vx * delta,
        y: streak.y + streak.vy * delta,
        age: streak.age + delta
      }))
      .filter((streak) => streak.age < streak.lifetime && streak.x > -streak.length && streak.y < height + streak.length);
    if (mode === "cosmos" && streaks.length < maxStreaks && Math.random() < 0.018 * delta) {
      streaks = [...streaks, createStreak()];
    }

    drawParticles();
    requestNextFrame();
  }

  function wrapParticleY(y: number) {
    if (height <= 0) return y;
    if (y < -10) return y + height + 20;
    if (y > height + 10) return y - height - 20;
    return y;
  }

  function currentFrameMs() {
    return performance.now() < scrollBoostUntil ? scrollFrameMs : idleFrameMs;
  }

  function requestImmediateFrame() {
    if (!animated || animationFrameId !== undefined || reducedMotion || document.visibilityState === "hidden") return;
    if (frameTimerId !== undefined) {
      clearTimeout(frameTimerId);
      frameTimerId = undefined;
    }
    animationFrameId = requestAnimationFrame(animate);
  }

  function requestNextFrame() {
    if (
      animationFrameId !== undefined ||
      frameTimerId !== undefined ||
      !animated ||
      reducedMotion ||
      document.visibilityState === "hidden"
    ) return;

    const delay = Math.max(0, currentFrameMs() - (performance.now() - lastFrameAt));
    if (delay > 4) {
      frameTimerId = window.setTimeout(() => {
        frameTimerId = undefined;
        if (animationFrameId === undefined && animated && !reducedMotion && document.visibilityState !== "hidden") {
          animationFrameId = requestAnimationFrame(animate);
        }
      }, delay);
      return;
    }

    animationFrameId = requestAnimationFrame(animate);
  }

  function cancelScheduledFrame() {
    if (animationFrameId !== undefined) {
      cancelAnimationFrame(animationFrameId);
      animationFrameId = undefined;
    }
    if (frameTimerId !== undefined) {
      clearTimeout(frameTimerId);
      frameTimerId = undefined;
    }
  }

  function handleVisibilityChange() {
    if (document.visibilityState === "hidden") {
      cancelScheduledFrame();
      return;
    }

    lastFrameAt = 0;
    lastScrollY = window.scrollY;
    drawParticles();
    if (animated) requestNextFrame();
  }

  function syncMotionPreference() {
    reducedMotion = Boolean(motionMediaQuery?.matches);
    if (reducedMotion) {
      cancelScheduledFrame();
      time = 0;
      scrollOffset = 0;
      targetScrollOffset = 0;
      drawParticles();
      return;
    }

    lastFrameAt = 0;
    lastScrollY = window.scrollY;
    if (animated) requestNextFrame();
  }

  onMount(() => {
    ctx = canvas.getContext("2d", { alpha: false });
    motionMediaQuery = window.matchMedia("(prefers-reduced-motion: reduce)");
    reducedMotion = motionMediaQuery.matches;
    lastScrollY = window.scrollY;
    resize();
    window.addEventListener("resize", resize);
    window.addEventListener("scroll", handleScroll, { passive: true });
    document.addEventListener("visibilitychange", handleVisibilityChange);
    motionMediaQuery.addEventListener("change", syncMotionPreference);
    if (animated) requestNextFrame();

    return () => {
      window.removeEventListener("resize", resize);
      window.removeEventListener("scroll", handleScroll);
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      motionMediaQuery?.removeEventListener("change", syncMotionPreference);
      cancelScheduledFrame();
    };
  });
</script>

<canvas
  bind:this={canvas}
  class="fixed inset-0 pointer-events-none"
  style="z-index: -1; width: 100vw; height: 100vh;"
></canvas>
