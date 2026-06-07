let lastPointerPosition: { id: number; x: number; y: number } | undefined;
let lastScrollAt = Number.NEGATIVE_INFINITY;
const scrollPointerSuppressionMs = 250;

function rememberPointerPosition(event: PointerEvent) {
  lastPointerPosition = {
    id: event.pointerId,
    x: event.clientX,
    y: event.clientY
  };
}

if (typeof window !== 'undefined') {
  window.addEventListener('pointermove', rememberPointerPosition, { passive: true });
  window.addEventListener(
    'scroll',
    () => {
      lastScrollAt = performance.now();
    },
    { capture: true, passive: true }
  );
}

export function pointerPositionChanged(event: PointerEvent) {
  const previous = lastPointerPosition;
  return (
    performance.now() - lastScrollAt >= scrollPointerSuppressionMs &&
    previous !== undefined &&
    previous.id === event.pointerId &&
    (previous.x !== event.clientX || previous.y !== event.clientY)
  );
}
