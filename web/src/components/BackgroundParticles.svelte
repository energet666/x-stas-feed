<script lang="ts">
  import { onMount } from "svelte";

  let { mode = "cosmos" }: { mode?: "cosmos" | "daylight" } = $props();

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
    glow: boolean;
    phase: number;
  };

  let particles: Particle[] = [];
  let animationFrameId: number | undefined = undefined;
  let width = 0;
  let height = 0;
  let time = 0;
  let lastFrameAt = 0;
  let lastScrollY = 0;
  let scrollOffset = 0;
  let targetScrollOffset = 0;

  function resize() {
    if (!canvas) return;
    width = window.innerWidth;
    height = window.innerHeight;
    canvas.width = width;
    canvas.height = height;
    initParticles();
    drawParticles();
  }

  function handleScroll() {
    const currentScrollY = window.scrollY;
    const delta = currentScrollY - lastScrollY;
    lastScrollY = currentScrollY;
    targetScrollOffset += Math.max(-18, Math.min(18, -delta * 0.065));
  }

  function initParticles() {
    particles = [];
    // Reduced density for better performance during scrolling
    const numParticles = Math.min(Math.floor((width * height) / 10000), 100);
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
      pulseSpeed: Math.random() * 0.018 + 0.008,
      glow: Math.random() > 0.8,
      phase: Math.random() * Math.PI * 2
    };
  }

  function drawParticles() {
    if (!ctx) return;
    if (mode === "daylight") {
      drawDaylightBase();
    } else {
      ctx.fillStyle = "rgb(0, 0, 0)";
      ctx.fillRect(0, 0, width, height);
    }

    for (let i = 0; i < particles.length; i++) {
      const p = particles[i];
      const alphaClamped = Math.max(0.05, Math.min(0.8, p.alpha));
      let drawY = p.y + scrollOffset;
      if (drawY < -10) drawY += height + 20;
      if (drawY > height + 10) drawY -= height + 20;

      ctx.beginPath();
      ctx.arc(p.x, drawY, p.radius, 0, Math.PI * 2);

      if (mode === "daylight") {
        const hue = p.glow ? "49, 70, 82" : "120, 101, 58";
        ctx.fillStyle = `rgba(${hue}, ${alphaClamped * 0.34})`;
      } else {
        // Removed expensive shadowBlur/shadowColor
        if (p.glow) {
          ctx.fillStyle = `rgba(255, 255, 255, ${alphaClamped})`;
        } else {
          ctx.fillStyle = `rgba(255, 255, 255, ${alphaClamped * 0.6})`;
        }
      }

      ctx.fill();
    }
  }

  function drawDaylightBase() {
    if (!ctx) return;
    const drift = time * 0.0025;
    const sky = ctx.createLinearGradient(0, 0, width, height);
    sky.addColorStop(0, "rgb(190, 195, 200)");
    sky.addColorStop(0.45, "rgb(174, 181, 187)");
    sky.addColorStop(1, "rgb(158, 168, 176)");
    ctx.fillStyle = sky;
    ctx.fillRect(0, 0, width, height);

    drawGlow(width * (0.16 + Math.sin(drift) * 0.04), height * 0.14, Math.max(width, height) * 0.5, "rgba(90, 106, 128, 0.28)");
    drawGlow(width * (0.86 + Math.cos(drift * 0.8) * 0.03), height * 0.22, Math.max(width, height) * 0.44, "rgba(56, 70, 92, 0.24)");
    drawGlow(width * 0.62, height * (0.88 + Math.sin(drift * 1.2) * 0.025), Math.max(width, height) * 0.52, "rgba(174, 184, 194, 0.2)");

    ctx.save();
    ctx.globalAlpha = 0.18;
    ctx.strokeStyle = "rgba(71, 85, 105, 0.14)";
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
    scrollOffset += (targetScrollOffset - scrollOffset) * Math.min(1, 0.14 * delta);
    targetScrollOffset *= Math.pow(0.92, delta);

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

    drawParticles();
    requestNextFrame();
  }

  function requestNextFrame() {
    if (animationFrameId !== undefined || document.visibilityState === "hidden") return;
    animationFrameId = requestAnimationFrame(animate);
  }

  function handleVisibilityChange() {
    if (document.visibilityState === "hidden") {
      if (animationFrameId !== undefined) {
        cancelAnimationFrame(animationFrameId);
        animationFrameId = undefined;
      }
      return;
    }

    lastFrameAt = 0;
    drawParticles();
    requestNextFrame();
  }

  onMount(() => {
    ctx = canvas.getContext("2d", { alpha: false });
    lastScrollY = window.scrollY;
    resize();
    window.addEventListener("resize", resize);
    window.addEventListener("scroll", handleScroll, { passive: true });
    document.addEventListener("visibilitychange", handleVisibilityChange);
    requestNextFrame();

    return () => {
      window.removeEventListener("resize", resize);
      window.removeEventListener("scroll", handleScroll);
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      if (animationFrameId !== undefined) {
        cancelAnimationFrame(animationFrameId);
      }
    };
  });
</script>

<canvas
  bind:this={canvas}
  class="fixed inset-0 pointer-events-none"
  style="z-index: -1; width: 100vw; height: 100vh;"
></canvas>
