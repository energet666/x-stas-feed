<script lang="ts">
  import { onMount } from 'svelte';

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
  };

  let particles: Particle[] = [];
  let animationFrameId: number;
  let width = 0;
  let height = 0;
  let time = 0;

  let lastScrollY = 0;
  let scrollVelocity = 0;

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
    // We dampen the velocity a bit and cap it to prevent crazy jumps
    scrollVelocity += delta * 0.15;
    lastScrollY = currentScrollY;
  }

  function initParticles() {
    particles = [];
    // Increased density: from 8000 to 6000 divisor (~33% increase)
    // Increased cap: from 120 to 160 (~33% increase)
    const numParticles = Math.min(Math.floor((width * height) / 6000), 160);
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
      glow: Math.random() > 0.8
    };
  }

  function animate() {
    if (!ctx) return;
    ctx.clearRect(0, 0, width, height);
    time += 1;

    // Decay scroll velocity
    scrollVelocity *= 0.92;

    for (let i = 0; i < particles.length; i++) {
      let p = particles[i];
      
      // Update position - particles react to scroll velocity
      // Scroll down (positive delta) makes particles go UP (negative y change)
      p.x += p.vx + Math.sin(time * p.pulseSpeed) * 0.1; // slight sway
      p.y += p.vy - scrollVelocity * 0.5;
      
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
      const currentAlpha = p.alpha + Math.sin(time * p.pulseSpeed * 2) * 0.1;
      const alphaClamped = Math.max(0.05, Math.min(0.8, currentAlpha));

      ctx.beginPath();
      ctx.arc(p.x, p.y, p.radius, 0, Math.PI * 2);
      
      if (p.glow) {
        ctx.shadowBlur = 8;
        ctx.shadowColor = 'rgba(255, 255, 255, 0.8)';
        ctx.fillStyle = `rgba(255, 255, 255, ${alphaClamped})`;
      } else {
        ctx.shadowBlur = 0;
        ctx.fillStyle = `rgba(255, 255, 255, ${alphaClamped * 0.65})`;
      }
      
      ctx.fill();
    }

    animationFrameId = requestAnimationFrame(animate);
  }

  onMount(() => {
    ctx = canvas.getContext('2d');
    lastScrollY = window.scrollY;
    resize();
    window.addEventListener('resize', resize);
    window.addEventListener('scroll', handleScroll, { passive: true });
    animate();

    return () => {
      window.removeEventListener('resize', resize);
      window.removeEventListener('scroll', handleScroll);
      cancelAnimationFrame(animationFrameId);
    };
  });
</script>

<canvas
  bind:this={canvas}
  class="fixed inset-0 pointer-events-none"
  style="z-index: -1;"
></canvas>
