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

  function resize() {
    if (!canvas) return;
    width = window.innerWidth;
    height = window.innerHeight;
    canvas.width = width;
    canvas.height = height;
    initParticles();
  }

  function initParticles() {
    particles = [];
    const numParticles = Math.min(Math.floor((width * height) / 8000), 120);
    for (let i = 0; i < numParticles; i++) {
      particles.push(createParticle(true));
    }
  }

  function createParticle(randomY = false): Particle {
    return {
      x: Math.random() * width,
      y: randomY ? Math.random() * height : height + 10,
      radius: Math.random() * 1.5 + 0.5,
      vx: (Math.random() - 0.5) * 0.3,
      vy: -(Math.random() * 0.5 + 0.2), // move upwards
      alpha: Math.random() * 0.5 + 0.1,
      pulseSpeed: Math.random() * 0.02 + 0.01,
      glow: Math.random() > 0.8
    };
  }

  function animate() {
    if (!ctx) return;
    ctx.clearRect(0, 0, width, height);
    time += 1;

    for (let i = 0; i < particles.length; i++) {
      let p = particles[i];
      
      // Update position
      p.x += p.vx + Math.sin(time * p.pulseSpeed) * 0.1; // slight sway
      p.y += p.vy;
      
      // Wrap around or recreate
      if (p.y < -10 || p.x < -10 || p.x > width + 10) {
        particles[i] = createParticle(false);
        p = particles[i];
      }

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
        ctx.fillStyle = `rgba(255, 255, 255, ${alphaClamped * 0.5})`;
      }
      
      ctx.fill();
    }

    animationFrameId = requestAnimationFrame(animate);
  }

  onMount(() => {
    ctx = canvas.getContext('2d');
    resize();
    window.addEventListener('resize', resize);
    animate();

    return () => {
      window.removeEventListener('resize', resize);
      cancelAnimationFrame(animationFrameId);
    };
  });
</script>

<canvas
  bind:this={canvas}
  class="fixed inset-0 pointer-events-none"
  style="z-index: -1;"
></canvas>
