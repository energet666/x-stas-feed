<script lang="ts">
  import { onMount } from "svelte";

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
  let animationFrameId: number;
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
      vx: (Math.random() - 0.5) * 0.3,
      vy: -(Math.random() * 0.5 + 0.2), // move upwards
      alpha: Math.random() * 0.5 + 0.2, // Increased alpha
      pulseSpeed: Math.random() * 0.02 + 0.01,
      glow: Math.random() > 0.8,
      phase: Math.random() * Math.PI * 2,
    };
  }

  function animate(now: number) {
    if (!ctx) return;
    const delta =
      lastFrameAt > 0 ? Math.min((now - lastFrameAt) / 16.67, 2) : 1;
    lastFrameAt = now;
    ctx.clearRect(0, 0, width, height);
    time += delta;
    scrollOffset +=
      (targetScrollOffset - scrollOffset) * Math.min(1, 0.14 * delta);
    targetScrollOffset *= Math.pow(0.92, delta);

    for (let i = 0; i < particles.length; i++) {
      let p = particles[i];

      p.x += (p.vx + Math.sin(time * p.pulseSpeed + p.phase) * 0.1) * delta;
      p.y += p.vy * delta;

      // Wrap around (recreate if they go off screen)
      if (p.y < -10) {
        p.y = height + 10;
        p.x = Math.random() * width;
      } else if (p.y > height + 10) {
        p.y = -10;
        p.x = Math.random() * width;
      }

      if (p.x < -10) p.x = width + 10;
      if (p.x > width + 10) p.x = -10;

      // Draw
      const currentAlpha =
        p.alpha + Math.sin(time * p.pulseSpeed * 2 + p.phase) * 0.1;
      const alphaClamped = Math.max(0.05, Math.min(0.8, currentAlpha));
      let drawY = p.y + scrollOffset;
      if (drawY < -10) drawY += height + 20;
      if (drawY > height + 10) drawY -= height + 20;

      ctx.beginPath();
      ctx.arc(p.x, drawY, p.radius, 0, Math.PI * 2);

      // Removed expensive shadowBlur/shadowColor
      if (p.glow) {
        ctx.fillStyle = `rgba(255, 255, 255, ${alphaClamped})`;
      } else {
        ctx.fillStyle = `rgba(255, 255, 255, ${alphaClamped * 0.6})`;
      }

      ctx.fill();
    }

    animationFrameId = requestAnimationFrame(animate);
  }

  onMount(() => {
    ctx = canvas.getContext("2d");
    lastScrollY = window.scrollY;
    resize();
    window.addEventListener("resize", resize);
    window.addEventListener("scroll", handleScroll, { passive: true });
    animationFrameId = requestAnimationFrame(animate);

    return () => {
      window.removeEventListener("resize", resize);
      window.removeEventListener("scroll", handleScroll);
      cancelAnimationFrame(animationFrameId);
    };
  });
</script>

<canvas
  bind:this={canvas}
  class="fixed inset-0 pointer-events-none"
  style="z-index: -1;"
></canvas>
