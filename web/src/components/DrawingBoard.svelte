<script lang="ts">
  import { onMount } from 'svelte';
  import { Activity, History, Palette, Pencil, X } from 'lucide-svelte';
  import {
    createStroke,
    fetchBoard,
    type Stroke
  } from '../lib/feed';
  import { boardEvents } from '../lib/board_events.svelte';

  let {
    boardId,
    expanded = false,
    username = 'Guest',
    onClose
  }: {
    boardId: string;
    expanded: boolean;
    username: string;
    onClose?: () => void;
  } = $props();

  type Tool = 'freeform' | 'line';

  const COLORS = [
    '#ffffff', '#ff4757', '#ff6b81', '#ffa502', '#ffdd59',
    '#2ed573', '#1e90ff', '#5352ed', '#a855f7', '#ff6348'
  ];

  const BRUSH_SIZES = [2, 4, 8, 14, 22];
  const DEBUG_SEGMENT_COLORS = ['#ff4757', '#ffa502', '#ffdd59', '#2ed573', '#1e90ff', '#a855f7'];
  const FREEFORM_POINT_DISTANCE = 3;
  const DEFAULT_FREEFORM_SIMPLIFY_EPSILON = 0.5;
  const MIN_FREEFORM_SIMPLIFY_EPSILON = 0;
  const MAX_FREEFORM_SIMPLIFY_EPSILON = 24;
  const MIN_BRUSH_SIZE = 1;
  const MAX_BRUSH_SIZE = 200;
  const brushColorStorageKey = 'feed-ai:drawing-brush-color';
  const brushSizeStorageKey = 'feed-ai:drawing-brush-size';

  let canvasEl = $state<HTMLCanvasElement | undefined>(undefined);
  let previewCanvasEl = $state<HTMLCanvasElement | undefined>(undefined);
  
  // Optimization: offscreen canvases
  let gridCanvas: HTMLCanvasElement | undefined;
  let committedCanvas: HTMLCanvasElement | undefined;
  let activeStrokeCanvas: HTMLCanvasElement | undefined;

  let strokes = $state<Stroke[]>([]);
  let strokeIds = new Set<string>();
  let currentTool = $state<Tool>('freeform');
  let currentColor = $state('#ffffff');
  let currentSize = $state(4);
  let isDrawing = $state(false);
  let currentPoints = $state<number[][]>([]);
  let lineStart = $state<number[] | null>(null);
  let mousePos = $state<number[] | null>(null);
  let showColorPicker = $state(false);
  let showDebugSegments = $state(false);
  let lastRawPointCount = $state<number | null>(null);
  let lastSimplifiedPointCount = $state<number | null>(null);
  let freeformSimplifyEpsilon = $state(DEFAULT_FREEFORM_SIMPLIFY_EPSILON);
  let historyMode = $state(false);
  let historyStrokeCount = $state(0);
  let boardName = $state('Board');
  let brushCursorVisible = $state(false);
  let brushCursorX = $state(0);
  let brushCursorY = $state(0);
  let brushCursorSize = $state(0);
  let sizeDragPointerId = $state<number | null>(null);
  let sizeDragStartY = $state(0);
  let sizeDragStartValue = $state(0);
  let sizeDragAppliedDelta = $state(0);
  let sizeDragActive = $state(false);
  let sizeDragSuppressClick = $state(false);

  const canvasWidth = 1200;
  const canvasHeight = 800;
  const SIZE_DRAG_STEP_PX = 3;
  const SIZE_DRAG_START_THRESHOLD_PX = 3;

  onMount(() => {
    loadBrushSettings();

    gridCanvas = document.createElement('canvas');
    gridCanvas.width = canvasWidth;
    gridCanvas.height = canvasHeight;
    drawGrid(gridCanvas.getContext('2d')!);

    committedCanvas = document.createElement('canvas');
    committedCanvas.width = canvasWidth;
    committedCanvas.height = canvasHeight;

    activeStrokeCanvas = document.createElement('canvas');
    activeStrokeCanvas.width = canvasWidth;
    activeStrokeCanvas.height = canvasHeight;

    void loadBoard();
  });

  // Global SSE subscription
  $effect(() => {
    if (!boardId) return;
    
    return boardEvents.subscribe((event) => {
      if (event.boardId === boardId) {
        appendCommittedStroke(event.stroke);
      }
    });
  });

  // Redraw main canvas when state changes
  $effect(() => {
    if (canvasEl || previewCanvasEl) {
      requestAnimationFrame(redraw);
    }
  });

  async function loadBoard() {
    try {
      const data = await fetchBoard(boardId);
      boardName = data.board.name;
      const loadedIds = new Set(data.strokes.map((stroke) => stroke.id));
      const sseStrokes = strokes.filter((stroke) => !loadedIds.has(stroke.id));
      rebuildCommittedCanvas([...data.strokes, ...sseStrokes]);
    } catch {
      // Board might not exist yet
    }
  }

  function rebuildCommittedCanvas(nextStrokes: Stroke[]) {
    if (!committedCanvas) return;
    const ctx = committedCanvas.getContext('2d');
    if (!ctx) return;

    const wasViewingLatest = historyStrokeCount >= strokes.length;
    strokes = nextStrokes;
    strokeIds = new Set(nextStrokes.map((stroke) => stroke.id));
    if (!historyMode || wasViewingLatest) {
      historyStrokeCount = nextStrokes.length;
    } else {
      historyStrokeCount = Math.min(historyStrokeCount, nextStrokes.length);
    }
    ctx.clearRect(0, 0, canvasWidth, canvasHeight);
    for (const stroke of nextStrokes) {
      drawStroke(ctx, stroke.points, stroke.color, stroke.size, stroke.tool);
    }
    redraw();
  }

  function appendCommittedStroke(stroke: Stroke) {
    if (strokeIds.has(stroke.id)) return;
    strokeIds.add(stroke.id);
    const wasViewingLatest = historyStrokeCount >= strokes.length;
    strokes = [...strokes, stroke];
    if (!historyMode || wasViewingLatest) {
      historyStrokeCount = strokes.length;
    }

    if (!committedCanvas) return;
    const ctx = committedCanvas.getContext('2d');
    if (!ctx) return;

    drawStroke(ctx, stroke.points, stroke.color, stroke.size, stroke.tool);
    redraw();
  }


  function getCanvas() {
    return expanded ? canvasEl : previewCanvasEl;
  }

  function getCanvasMetrics() {
    const canvas = getCanvas();
    if (!canvas) return null;
    
    const rect = canvas.getBoundingClientRect();
    const containerWidth = rect.width;
    const containerHeight = rect.height;
    
    const canvasAspect = canvasWidth / canvasHeight;
    const containerAspect = containerWidth / containerHeight;
    
    let renderedWidth, renderedHeight;
    let offsetX = 0;
    let offsetY = 0;
    
    if (containerAspect > canvasAspect) {
      // Pillarboxing (black bars on sides)
      renderedHeight = containerHeight;
      renderedWidth = renderedHeight * canvasAspect;
      offsetX = (containerWidth - renderedWidth) / 2;
    } else {
      // Letterboxing (black bars on top/bottom)
      renderedWidth = containerWidth;
      renderedHeight = renderedWidth / canvasAspect;
      offsetY = (containerHeight - renderedHeight) / 2;
    }
    
    const scaleX = canvasWidth / renderedWidth;
    const scaleY = canvasHeight / renderedHeight;

    return { rect, renderedWidth, renderedHeight, offsetX, offsetY, scaleX, scaleY };
  }

  function canvasCoords(event: PointerEvent): [number, number] {
    const metrics = getCanvasMetrics();
    if (!metrics) return [0, 0];
    
    return [
      (event.clientX - metrics.rect.left - metrics.offsetX) * metrics.scaleX,
      (event.clientY - metrics.rect.top - metrics.offsetY) * metrics.scaleY
    ];
  }

  function updateBrushCursor(event: PointerEvent) {
    if (!expanded) return;

    const metrics = getCanvasMetrics();
    if (!metrics) return;

    const wrap = (event.currentTarget as HTMLElement).parentElement;
    if (!wrap) return;

    const wrapRect = wrap.getBoundingClientRect();
    const displayScale = metrics.renderedWidth / canvasWidth;

    brushCursorVisible = true;
    brushCursorX = event.clientX - wrapRect.left;
    brushCursorY = event.clientY - wrapRect.top;
    brushCursorSize = Math.max(2, currentSize * displayScale);
  }

  function handlePointerEnter(event: PointerEvent) {
    if (historyMode) return;
    updateBrushCursor(event);
  }

  function handlePointerLeave() {
    if (!isDrawing) {
      brushCursorVisible = false;
    }
  }

  function handlePointerDown(event: PointerEvent) {
    if (!expanded || historyMode) return;
    updateBrushCursor(event);
    const canvas = getCanvas();
    if (!canvas) return;
    canvas.setPointerCapture(event.pointerId);

    const [x, y] = canvasCoords(event);

    if (currentTool === 'freeform') {
      isDrawing = true;
      currentPoints = [[x, y]];
      if (activeStrokeCanvas) {
        const ctx = activeStrokeCanvas.getContext('2d')!;
        ctx.clearRect(0, 0, canvasWidth, canvasHeight);
      }
    } else if (currentTool === 'line') {
      if (!lineStart) {
        lineStart = [x, y];
      } else {
        const pts: number[][] = [lineStart, [x, y]];
        lineStart = null;
        mousePos = null;
        void submitStroke('line', pts);
      }
    }
  }

  function handlePointerMove(event: PointerEvent) {
    if (!expanded || historyMode) return;
    updateBrushCursor(event);

    const [x, y] = canvasCoords(event);

    if (currentTool === 'line' && lineStart) {
      mousePos = [x, y];
      redraw();
      return;
    }

    if (!isDrawing || currentTool !== 'freeform' || !activeStrokeCanvas) return;

    const lastPoint = currentPoints[currentPoints.length - 1];
    const dist = Math.hypot(x - lastPoint[0], y - lastPoint[1]);
    
    // Filter points that are too close to reduce data and complexity
    if (dist < FREEFORM_POINT_DISTANCE) return;

    const newPoints = [...currentPoints, [x, y]];

    const ctx = activeStrokeCanvas.getContext('2d')!;
    ctx.clearRect(0, 0, canvasWidth, canvasHeight);
    if (showDebugSegments) {
      drawDebugSegmentedFreeformStroke(ctx, newPoints, currentSize);
    } else {
      drawStroke(ctx, newPoints, currentColor, currentSize, 'freeform');
    }

    currentPoints = newPoints;
    redraw();
  }

  function handlePointerUp(event: PointerEvent) {
    if (!expanded || historyMode) return;
    updateBrushCursor(event);
    const canvas = getCanvas();
    if (canvas) canvas.releasePointerCapture(event.pointerId);

    if (currentTool === 'freeform' && isDrawing && currentPoints.length >= 1) {
      isDrawing = false;
      const simplifiedPoints = simplifyFreeformPoints(currentPoints);
      lastRawPointCount = currentPoints.length;
      lastSimplifiedPointCount = simplifiedPoints.length;
      void submitStroke('freeform', simplifiedPoints);
      currentPoints = [];
      if (activeStrokeCanvas) {
        activeStrokeCanvas.getContext('2d')!.clearRect(0, 0, canvasWidth, canvasHeight);
      }
    } else {
      isDrawing = false;
      currentPoints = [];
      if (activeStrokeCanvas) {
        activeStrokeCanvas.getContext('2d')!.clearRect(0, 0, canvasWidth, canvasHeight);
      }
    }
  }

  function simplifyFreeformPoints(points: number[][]) {
    if (points.length <= 2) return points;

    const keep = new Array<boolean>(points.length).fill(false);
    keep[0] = true;
    keep[points.length - 1] = true;
    simplifyPointRange(points, 0, points.length - 1, freeformSimplifyEpsilon ** 2, keep);

    return points.filter((_, index) => keep[index]);
  }

  function simplifyPointRange(
    points: number[][],
    startIndex: number,
    endIndex: number,
    epsilonSquared: number,
    keep: boolean[]
  ) {
    if (endIndex <= startIndex + 1) return;

    let farthestIndex = -1;
    let farthestDistance = 0;
    const start = points[startIndex];
    const end = points[endIndex];

    for (let i = startIndex + 1; i < endIndex; i += 1) {
      const distance = pointToSegmentDistanceSquared(points[i], start, end);
      if (distance > farthestDistance) {
        farthestDistance = distance;
        farthestIndex = i;
      }
    }

    if (farthestIndex === -1 || farthestDistance <= epsilonSquared) return;

    keep[farthestIndex] = true;
    simplifyPointRange(points, startIndex, farthestIndex, epsilonSquared, keep);
    simplifyPointRange(points, farthestIndex, endIndex, epsilonSquared, keep);
  }

  function pointToSegmentDistanceSquared(point: number[], start: number[], end: number[]) {
    const dx = end[0] - start[0];
    const dy = end[1] - start[1];
    const lengthSquared = dx * dx + dy * dy;

    if (lengthSquared === 0) {
      const pointDx = point[0] - start[0];
      const pointDy = point[1] - start[1];
      return pointDx * pointDx + pointDy * pointDy;
    }

    const t = Math.max(
      0,
      Math.min(1, ((point[0] - start[0]) * dx + (point[1] - start[1]) * dy) / lengthSquared)
    );
    const projectionX = start[0] + t * dx;
    const projectionY = start[1] + t * dy;
    const projectionDx = point[0] - projectionX;
    const projectionDy = point[1] - projectionY;

    return projectionDx * projectionDx + projectionDy * projectionDy;
  }

  async function submitStroke(tool: string, points: number[][]) {
    try {
      await createStroke(boardId, tool, points, currentColor, currentSize, username);
    } catch {
      // Failed to submit stroke
    }
  }

  function drawGrid(ctx: CanvasRenderingContext2D) {
    ctx.clearRect(0, 0, canvasWidth, canvasHeight);
    // Dark background
    ctx.fillStyle = '#0f0f17';
    ctx.fillRect(0, 0, canvasWidth, canvasHeight);

    // Subtle grid
    ctx.strokeStyle = 'rgba(255,255,255,0.03)';
    ctx.lineWidth = 1;
    const gridSize = 40;
    for (let x = gridSize; x < canvasWidth; x += gridSize) {
      ctx.beginPath();
      ctx.moveTo(x, 0);
      ctx.lineTo(x, canvasHeight);
      ctx.stroke();
    }
    for (let y = gridSize; y < canvasHeight; y += gridSize) {
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(canvasWidth, y);
      ctx.stroke();
    }
  }

  function redraw() {
    const canvas = expanded ? canvasEl : previewCanvasEl;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    // 1. Draw static grid
    if (gridCanvas) {
      ctx.drawImage(gridCanvas, 0, 0);
    } else {
      ctx.fillStyle = '#0f0f17';
      ctx.fillRect(0, 0, canvasWidth, canvasHeight);
    }

    if (historyMode && expanded) {
      for (const stroke of strokes.slice(0, historyStrokeCount)) {
        drawStroke(ctx, stroke.points, stroke.color, stroke.size, stroke.tool);
      }
      return;
    }

    // 2. Draw all committed strokes from buffer
    if (committedCanvas) {
      ctx.drawImage(committedCanvas, 0, 0);
    }

    // 3. Draw active stroke canvas (incremental)
    if (isDrawing && activeStrokeCanvas) {
      ctx.drawImage(activeStrokeCanvas, 0, 0);
    }

    // 4. Draw line preview (not incremental but very few points)
    if (currentTool === 'line' && lineStart && mousePos) {
      drawStroke(ctx, [lineStart, mousePos], currentColor, currentSize, 'line');
    }
  }

  function drawSegment(ctx: CanvasRenderingContext2D, p1: number[], p2: number[], color: string, size: number) {
    ctx.save();
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    ctx.strokeStyle = color;
    ctx.lineWidth = size;
    ctx.beginPath();
    ctx.moveTo(p1[0], p1[1]);
    ctx.lineTo(p2[0], p2[1]);
    ctx.stroke();
    ctx.restore();
  }

  function drawPoint(ctx: CanvasRenderingContext2D, point: number[], color: string, size: number) {
    ctx.save();
    ctx.fillStyle = color;
    ctx.beginPath();
    ctx.arc(point[0], point[1], size / 2, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
  }

  function drawDebugSegmentedFreeformStroke(
    ctx: CanvasRenderingContext2D,
    points: number[][],
    size: number
  ) {
    if (points.length === 0) return;
    if (points.length === 1) {
      drawPoint(ctx, points[0], DEBUG_SEGMENT_COLORS[0], size);
      return;
    }

    ctx.save();
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    ctx.lineWidth = size;

    let start = points[0];
    for (let i = 1; i < points.length; i += 1) {
      ctx.strokeStyle = DEBUG_SEGMENT_COLORS[(i - 1) % DEBUG_SEGMENT_COLORS.length];
      ctx.beginPath();
      ctx.moveTo(start[0], start[1]);

      let end = points[i];
      if (i < points.length - 1) {
        end = [
          (points[i][0] + points[i + 1][0]) / 2,
          (points[i][1] + points[i + 1][1]) / 2
        ];
        ctx.quadraticCurveTo(points[i][0], points[i][1], end[0], end[1]);
      } else {
        ctx.lineTo(end[0], end[1]);
      }

      ctx.stroke();
      start = end;
    }

    ctx.restore();
  }

  function drawStroke(
    ctx: CanvasRenderingContext2D,
    points: number[][],
    color: string,
    size: number,
    tool: string
  ) {
    if (points.length === 0) return;
    if (points.length === 1) {
      drawPoint(ctx, points[0], color, size);
      return;
    }

    ctx.save();
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    ctx.strokeStyle = color;
    ctx.lineWidth = size;
    
    ctx.beginPath();
    ctx.moveTo(points[0][0], points[0][1]);

    if (tool === 'line') {
      ctx.lineTo(points[points.length - 1][0], points[points.length - 1][1]);
    } else {
      for (let i = 1; i < points.length; i++) {
        if (i < points.length - 1) {
          const midX = (points[i][0] + points[i + 1][0]) / 2;
          const midY = (points[i][1] + points[i + 1][1]) / 2;
          ctx.quadraticCurveTo(points[i][0], points[i][1], midX, midY);
        } else {
          ctx.lineTo(points[i][0], points[i][1]);
        }
      }
    }
    ctx.stroke();
    ctx.restore();
  }

  function selectTool(tool: Tool) {
    currentTool = tool;
    lineStart = null;
    mousePos = null;
    showColorPicker = false;
    redraw();
  }

  function selectColor(color: string) {
    currentColor = color;
    saveBrushColor(color);
    showColorPicker = false;
  }

  function setCustomColor(color: string) {
    currentColor = color;
    saveBrushColor(color);
  }

  function selectSize(size: number) {
    const normalizedSize = Math.min(MAX_BRUSH_SIZE, Math.max(MIN_BRUSH_SIZE, Math.round(size)));
    currentSize = normalizedSize;
    saveBrushSize(normalizedSize);
    if (brushCursorVisible && canvasEl) {
      const metrics = getCanvasMetrics();
      brushCursorSize = metrics
        ? Math.max(2, normalizedSize * (metrics.renderedWidth / canvasWidth))
        : normalizedSize;
    }
  }

  function handleCustomSizeInput(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    selectSize(input.valueAsNumber || MIN_BRUSH_SIZE);
  }

  function handleCustomSizeWheel(event: WheelEvent) {
    event.preventDefault();
    event.stopPropagation();
    selectSize(currentSize + (event.deltaY < 0 ? 1 : -1));
  }

  function handleCustomSizePointerDown(event: PointerEvent) {
    if (event.button !== 0) return;

    const input = event.currentTarget as HTMLInputElement;
    sizeDragPointerId = event.pointerId;
    sizeDragStartY = event.clientY;
    sizeDragStartValue = currentSize;
    sizeDragAppliedDelta = 0;
    sizeDragActive = false;
    sizeDragSuppressClick = false;
    input.setPointerCapture(event.pointerId);
  }

  function handleCustomSizePointerMove(event: PointerEvent) {
    if (sizeDragPointerId !== event.pointerId) return;

    const distance = sizeDragStartY - event.clientY;
    if (!sizeDragActive && Math.abs(distance) < SIZE_DRAG_START_THRESHOLD_PX) return;

    sizeDragActive = true;
    event.preventDefault();
    event.stopPropagation();

    const nextDelta = Math.trunc(distance / SIZE_DRAG_STEP_PX);
    if (nextDelta === sizeDragAppliedDelta) return;

    sizeDragAppliedDelta = nextDelta;
    selectSize(sizeDragStartValue + nextDelta);
  }

  function finishCustomSizeDrag(event: PointerEvent) {
    if (sizeDragPointerId !== event.pointerId) return;

    const input = event.currentTarget as HTMLInputElement;
    if (input.hasPointerCapture(event.pointerId)) {
      input.releasePointerCapture(event.pointerId);
    }
    sizeDragSuppressClick = sizeDragActive;
    sizeDragPointerId = null;
    sizeDragActive = false;
  }

  function handleCustomSizeClick(event: MouseEvent) {
    if (!sizeDragSuppressClick) return;
    event.preventDefault();
    event.stopPropagation();
    sizeDragSuppressClick = false;
  }

  function handleSimplifyEpsilonInput(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const value = Number.isFinite(input.valueAsNumber)
      ? input.valueAsNumber
      : DEFAULT_FREEFORM_SIMPLIFY_EPSILON;
    freeformSimplifyEpsilon = Math.min(
      MAX_FREEFORM_SIMPLIFY_EPSILON,
      Math.max(MIN_FREEFORM_SIMPLIFY_EPSILON, value)
    );
  }

  function enterHistoryMode() {
    historyMode = true;
    historyStrokeCount = strokes.length;
    lineStart = null;
    mousePos = null;
    isDrawing = false;
    currentPoints = [];
    showColorPicker = false;
    brushCursorVisible = false;
    if (activeStrokeCanvas) {
      activeStrokeCanvas.getContext('2d')!.clearRect(0, 0, canvasWidth, canvasHeight);
    }
    redraw();
  }

  function exitHistoryMode() {
    historyMode = false;
    historyStrokeCount = strokes.length;
    redraw();
  }

  function handleHistoryRangeInput(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const value = Number.isFinite(input.valueAsNumber) ? input.valueAsNumber : strokes.length;
    historyStrokeCount = Math.min(strokes.length, Math.max(0, Math.round(value)));
    redraw();
  }

  function loadBrushSettings() {
    try {
      const storedColor = window.localStorage.getItem(brushColorStorageKey);
      if (storedColor && /^#[0-9a-f]{6}$/i.test(storedColor)) {
        currentColor = storedColor;
      }

      const storedSize = Number.parseInt(window.localStorage.getItem(brushSizeStorageKey) ?? '', 10);
      if (Number.isFinite(storedSize)) {
        currentSize = Math.min(MAX_BRUSH_SIZE, Math.max(MIN_BRUSH_SIZE, storedSize));
      }
    } catch {
      // Ignore storage failures; drawing controls should keep working in memory.
    }
  }

  function saveBrushColor(color: string) {
    try {
      window.localStorage.setItem(brushColorStorageKey, color);
    } catch {
      // Ignore storage failures; the selected color still applies in memory.
    }
  }

  function saveBrushSize(size: number) {
    try {
      window.localStorage.setItem(brushSizeStorageKey, String(size));
    } catch {
      // Ignore storage failures; the selected size still applies in memory.
    }
  }

  function handleKeyDown(event: KeyboardEvent) {
    if (event.key === 'Escape' && historyMode) {
      exitHistoryMode();
      event.stopPropagation();
      return;
    }

    if (event.key === 'Escape' && lineStart) {
      lineStart = null;
      mousePos = null;
      redraw();
      event.stopPropagation();
    }
  }

  function closeBoard(event: MouseEvent) {
    event.stopPropagation();
    onClose?.();
  }
</script>

<div class="drawing-board" class:drawing-board-expanded={expanded}>
  {#if expanded}
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions - the focused drawing surface needs keyboard shortcuts while the canvas handles pointer input. -->
    <div
      class="drawing-canvas-wrap"
      role="application"
      aria-label="Drawing board: {boardName}"
      onkeydown={handleKeyDown}
      tabindex="-1"
    >
      <canvas
        bind:this={canvasEl}
        width={canvasWidth}
        height={canvasHeight}
        class="drawing-canvas"
        onpointerdown={handlePointerDown}
        onpointermove={handlePointerMove}
        onpointerup={handlePointerUp}
        onpointerenter={handlePointerEnter}
        onpointerleave={handlePointerLeave}
        style="cursor: {historyMode ? 'default' : 'none'}; touch-action: none;"
      ></canvas>
      {#if !historyMode}
        <div
          class="drawing-brush-cursor"
          class:drawing-brush-cursor-visible={brushCursorVisible}
          style="
            width: {brushCursorSize}px;
            height: {brushCursorSize}px;
            transform: translate3d({brushCursorX}px, {brushCursorY}px, 0) translate(-50%, -50%);
            border-color: {currentColor};
          "
        ></div>
      {/if}

      {#if onClose}
        <button
          class="drawing-close-btn"
          type="button"
          aria-label="Close drawing board"
          title="Close drawing board"
          onclick={closeBoard}
        >
          <X size={18} />
        </button>
      {/if}

      {#if historyMode}
        <div class="drawing-history-toolbar">
          <input
            class="drawing-history-range"
            type="range"
            min="0"
            max={strokes.length}
            step="1"
            value={historyStrokeCount}
            aria-label="Visible drawing history strokes"
            title="Visible drawing history strokes"
            oninput={handleHistoryRangeInput}
          />
          <div
            class="drawing-history-count"
            aria-label={`Showing ${historyStrokeCount} of ${strokes.length} strokes`}
          >
            {historyStrokeCount}/{strokes.length}
          </div>
          <button
            class="drawing-tool-btn drawing-history-exit"
            type="button"
            aria-label="Exit history mode"
            title="Exit history mode"
            onclick={exitHistoryMode}
          >
            <X size={16} />
          </button>
        </div>
      {:else}
      <div class="drawing-toolbar">
        <div class="drawing-toolbar-group">
          <button
            class="drawing-tool-btn"
            class:drawing-tool-btn-active={currentTool === 'freeform'}
            title="Freeform"
            onclick={() => selectTool('freeform')}
          >
            <Pencil size={16} />
          </button>
          <button
            class="drawing-tool-btn"
            class:drawing-tool-btn-active={currentTool === 'line'}
            title="Line"
            onclick={() => selectTool('line')}
          >
            <svg
              class="drawing-segment-icon"
              viewBox="0 0 24 24"
              width="16"
              height="16"
              aria-hidden="true"
            >
              <line x1="7" y1="17" x2="17" y2="7"></line>
              <circle cx="7" cy="17" r="2.25"></circle>
              <circle cx="17" cy="7" r="2.25"></circle>
            </svg>
          </button>
          <button
            class="drawing-tool-btn"
            class:drawing-tool-btn-active={showDebugSegments}
            type="button"
            title="Show point density"
            aria-label="Show point density"
            aria-pressed={showDebugSegments}
            onclick={() => (showDebugSegments = !showDebugSegments)}
          >
            <Activity size={16} />
          </button>
          {#if lastRawPointCount !== null && lastSimplifiedPointCount !== null}
            <div
              class="drawing-point-stats"
              title="Last stroke points before/after simplification"
              aria-label={`Last stroke points before simplification ${lastRawPointCount}, after simplification ${lastSimplifiedPointCount}`}
            >
              <span>{lastRawPointCount}</span>
              <span>{lastSimplifiedPointCount}</span>
            </div>
          {/if}
          <input
            class="drawing-epsilon-input"
            type="number"
            min={MIN_FREEFORM_SIMPLIFY_EPSILON}
            max={MAX_FREEFORM_SIMPLIFY_EPSILON}
            step="0.5"
            value={freeformSimplifyEpsilon}
            aria-label="Simplification error tolerance"
            title="Simplification error tolerance"
            oninput={handleSimplifyEpsilonInput}
          />
        </div>

        <div class="drawing-toolbar-divider"></div>

        <div class="drawing-toolbar-group drawing-sizes">
          {#each BRUSH_SIZES as size}
            <button
              class="drawing-size-btn"
              class:drawing-size-btn-active={currentSize === size}
              title="Size {size}"
              onclick={() => selectSize(size)}
            >
              <span
                class="drawing-size-dot"
                style="width: {Math.min(size, 18)}px; height: {Math.min(size, 18)}px; background: {currentColor};"
              ></span>
            </button>
          {/each}
          <input
            class="drawing-size-custom"
            class:drawing-size-custom-dragging={sizeDragActive}
            type="number"
            min={MIN_BRUSH_SIZE}
            max={MAX_BRUSH_SIZE}
            step="1"
            value={currentSize}
            aria-label="Custom brush size"
            title="Custom brush size"
            onclick={handleCustomSizeClick}
            oninput={handleCustomSizeInput}
            onwheel={handleCustomSizeWheel}
            onpointerdown={handleCustomSizePointerDown}
            onpointermove={handleCustomSizePointerMove}
            onpointerup={finishCustomSizeDrag}
            onpointercancel={finishCustomSizeDrag}
          />
        </div>

        <div class="drawing-toolbar-divider"></div>

        <div class="drawing-toolbar-group drawing-colors">
          <button
            class="drawing-tool-btn"
            class:drawing-tool-btn-active={showColorPicker}
            title="Color"
            onclick={() => (showColorPicker = !showColorPicker)}
          >
            <Palette size={16} />
            <span
              class="drawing-color-indicator"
              style="background: {currentColor};"
            ></span>
          </button>
          {#if showColorPicker}
            <div class="drawing-color-grid">
              {#each COLORS as color}
                <button
                  class="drawing-color-swatch"
                  class:drawing-color-swatch-active={currentColor === color}
                  style="background: {color};"
                  title={color}
                  onclick={() => selectColor(color)}
                ></button>
              {/each}
              <label class="drawing-color-custom-wrap" title="Custom color" aria-label="Custom color">
                <Palette size={14} />
                <span
                  class="drawing-color-custom-preview"
                  style="background: {currentColor};"
                ></span>
                <input
                  type="color"
                class="drawing-color-custom"
                value={currentColor}
                oninput={(e) => setCustomColor((e.currentTarget as HTMLInputElement).value)}
                onchange={(e) => selectColor((e.currentTarget as HTMLInputElement).value)}
              />
            </label>
            </div>
          {/if}
        </div>
        <div class="drawing-toolbar-divider"></div>
        <div class="drawing-toolbar-group">
          <button
            class="drawing-tool-btn"
            type="button"
            title="History"
            aria-label="Open history mode"
            onclick={enterHistoryMode}
          >
            <History size={16} />
          </button>
        </div>
      </div>
      {/if}
    </div>
  {:else}
    <div class="drawing-preview">
      <canvas
        bind:this={previewCanvasEl}
        width={canvasWidth}
        height={canvasHeight}
        class="drawing-canvas-preview"
      ></canvas>
    </div>
  {/if}
</div>

<style>
  .drawing-board {
    position: relative;
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    background: #0f0f17;
    overflow: hidden;
  }

  .drawing-board-expanded {
    position: fixed;
    inset: 0;
    z-index: 200; /* Ensure it's above FeedCardFrame overlays (z-index 6) */
  }

  .drawing-canvas-wrap {
    position: relative;
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .drawing-canvas {
    width: 100%;
    height: 100%;
    object-fit: contain;
    display: block;
  }

  .drawing-brush-cursor {
    position: absolute;
    top: 0;
    left: 0;
    z-index: 8;
    border: 1.5px solid #fff;
    border-radius: 50%;
    box-shadow:
      0 0 0 1px rgba(0, 0, 0, 0.55),
      0 0 8px rgba(0, 0, 0, 0.35);
    opacity: 0;
    pointer-events: none;
    transition:
      opacity 90ms ease,
      width 90ms ease,
      height 90ms ease;
    will-change: transform, width, height;
  }

  .drawing-brush-cursor-visible {
    opacity: 1;
  }

  .drawing-preview {
    position: relative;
    width: 100%;
    aspect-ratio: 3 / 2;
    overflow: hidden;
  }

  .drawing-canvas-preview {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
    pointer-events: none;
  }

  .drawing-toolbar {
    position: absolute;
    bottom: 1.25rem;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    flex-direction: row; /* Horizontal in preview if ever shown, but we only show in expanded */
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 0.75rem;
    border-radius: 1rem;
    background: rgba(15, 15, 23, 0.6); /* Slightly more opaque base */
    border: 1px solid rgba(255, 255, 255, 0.08);
    z-index: 10;
    opacity: 0.6; /* Increased default visibility */
    backdrop-filter: blur(4px); /* Subtle blur even in idle */
    -webkit-backdrop-filter: blur(4px);
    transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
    pointer-events: auto;
  }

  .drawing-close-btn {
    position: absolute;
    top: 1rem;
    right: 1rem;
    z-index: 12;
    display: grid;
    width: 2.5rem;
    height: 2.5rem;
    place-items: center;
    border: 1px solid rgba(255, 255, 255, 0.16);
    border-radius: 0.75rem;
    background: rgba(15, 15, 23, 0.72);
    color: rgba(255, 255, 255, 0.88);
    backdrop-filter: blur(14px) saturate(150%);
    -webkit-backdrop-filter: blur(14px) saturate(150%);
    transition:
      background 140ms ease,
      border-color 140ms ease,
      color 140ms ease;
  }

  .drawing-close-btn:hover {
    border-color: rgba(255, 255, 255, 0.28);
    background: rgba(15, 15, 23, 0.9);
    color: #fff;
  }

  .drawing-toolbar:hover {
    opacity: 1;
    background: rgba(15, 15, 23, 0.85);
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
  }

  .drawing-history-toolbar {
    position: absolute;
    right: 1.25rem;
    bottom: 1.25rem;
    left: 1.25rem;
    z-index: 10;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto auto;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem;
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 0.9rem;
    background: rgba(15, 15, 23, 0.84);
    backdrop-filter: blur(16px) saturate(150%);
    -webkit-backdrop-filter: blur(16px) saturate(150%);
    box-shadow: 0 12px 36px rgba(0, 0, 0, 0.36);
    pointer-events: auto;
  }

  .drawing-history-range {
    width: 100%;
    accent-color: #ffffff;
    cursor: pointer;
  }

  .drawing-history-count {
    min-width: 4.2rem;
    padding: 0.35rem 0.45rem;
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 0.45rem;
    background: rgba(255, 255, 255, 0.07);
    color: rgba(255, 255, 255, 0.76);
    font-size: 0.72rem;
    font-variant-numeric: tabular-nums;
    line-height: 1;
    text-align: center;
  }

  .drawing-history-exit {
    border-color: rgba(255, 255, 255, 0.12);
    background: rgba(255, 255, 255, 0.07);
  }

  /* Vertical toolbar on the right when expanded */
  .drawing-board-expanded .drawing-toolbar {
    bottom: auto;
    left: auto;
    top: 50%;
    right: 1.25rem;
    transform: translateY(-50%);
    flex-direction: column;
    padding: 0.75rem 0.5rem;
    border-radius: 1.25rem;
  }

  .drawing-toolbar-group {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    position: relative;
  }

  .drawing-board-expanded .drawing-toolbar-group {
    flex-direction: column;
  }

  .drawing-toolbar-divider {
    width: 1px;
    height: 1.5rem;
    background: rgba(255, 255, 255, 0.1);
  }

  .drawing-board-expanded .drawing-toolbar-divider {
    width: 1.5rem;
    height: 1px;
    margin: 0.25rem 0;
  }

  .drawing-tool-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.35rem;
    width: 2.25rem;
    height: 2.25rem;
    border-radius: 0.5rem;
    border: 1px solid transparent;
    background: transparent;
    color: rgba(255, 255, 255, 0.6);
    cursor: pointer;
    transition: all 150ms ease;
  }

  .drawing-tool-btn:hover {
    background: rgba(255, 255, 255, 0.08);
    color: #fff;
  }

  .drawing-tool-btn-active {
    background: rgba(255, 255, 255, 0.12);
    border-color: rgba(255, 255, 255, 0.15);
    color: #fff;
  }

  .drawing-point-stats {
    display: flex;
    flex-direction: column;
    min-width: 2.25rem;
    max-width: 2.75rem;
    height: 2.25rem;
    align-items: center;
    justify-content: center;
    gap: 0.12rem;
    padding: 0 0.35rem;
    overflow: hidden;
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 0.4rem;
    background: rgba(255, 255, 255, 0.07);
    color: rgba(255, 255, 255, 0.72);
    font-size: 0.64rem;
    font-variant-numeric: tabular-nums;
    line-height: 1;
  }

  .drawing-point-stats span {
    width: 100%;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    text-align: center;
  }

  .drawing-epsilon-input {
    width: 2.75rem;
    height: 1.65rem;
    border: 1px solid rgba(255, 255, 255, 0.14);
    border-radius: 0.45rem;
    background: rgba(255, 255, 255, 0.07);
    color: rgba(255, 255, 255, 0.86);
    font-size: 0.68rem;
    font-variant-numeric: tabular-nums;
    text-align: center;
    outline: none;
    transition:
      background 150ms ease,
      border-color 150ms ease;
  }

  .drawing-epsilon-input:hover,
  .drawing-epsilon-input:focus {
    border-color: rgba(255, 255, 255, 0.26);
    background: rgba(255, 255, 255, 0.12);
  }

  .drawing-epsilon-input::-webkit-inner-spin-button,
  .drawing-epsilon-input::-webkit-outer-spin-button {
    opacity: 0.85;
  }

  .drawing-segment-icon {
    fill: none;
    stroke: currentColor;
    stroke-width: 2;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .drawing-color-indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .drawing-sizes {
    gap: 0.15rem;
  }

  .drawing-size-btn {
    display: grid;
    place-items: center;
    width: 2rem;
    height: 2rem;
    border-radius: 0.5rem;
    border: 1px solid transparent;
    background: transparent;
    cursor: pointer;
    transition: all 150ms ease;
  }

  .drawing-size-btn:hover {
    background: rgba(255, 255, 255, 0.08);
  }

  .drawing-size-btn-active {
    background: rgba(255, 255, 255, 0.12);
    border-color: rgba(255, 255, 255, 0.15);
  }

  .drawing-size-dot {
    border-radius: 50%;
    display: block;
  }

  .drawing-size-custom {
    width: 2.5rem;
    height: 1.85rem;
    border: 1px solid rgba(255, 255, 255, 0.14);
    border-radius: 0.45rem;
    background: rgba(255, 255, 255, 0.07);
    color: #fff;
    font-size: 0.72rem;
    font-variant-numeric: tabular-nums;
    text-align: center;
    outline: none;
    cursor: ns-resize;
    user-select: none;
    transition:
      background 150ms ease,
      border-color 150ms ease;
  }

  .drawing-size-custom:hover,
  .drawing-size-custom:focus {
    border-color: rgba(255, 255, 255, 0.26);
    background: rgba(255, 255, 255, 0.12);
  }

  .drawing-size-custom-dragging {
    border-color: rgba(255, 255, 255, 0.42);
    background: rgba(255, 255, 255, 0.16);
  }

  .drawing-size-custom::-webkit-inner-spin-button,
  .drawing-size-custom::-webkit-outer-spin-button {
    opacity: 0.85;
  }

  .drawing-color-grid {
    position: absolute;
    bottom: 3rem;
    left: 50%;
    transform: translateX(-50%);
    display: grid;
    grid-template-columns: repeat(5, 1.75rem);
    gap: 0.35rem;
    padding: 0.6rem;
    border-radius: 0.75rem;
    background: rgba(15, 15, 23, 0.95);
    border: 1px solid rgba(255, 255, 255, 0.08);
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
  }

  .drawing-board-expanded .drawing-color-grid {
    bottom: 0;
    left: auto;
    right: 3.5rem;
    transform: none;
    grid-template-columns: repeat(2, 1.75rem);
  }

  .drawing-color-swatch {
    width: 1.75rem;
    height: 1.75rem;
    border-radius: 50%;
    border: 2px solid transparent;
    cursor: pointer;
    transition: all 150ms ease;
  }

  .drawing-color-swatch:hover {
    transform: scale(1.15);
  }

  .drawing-color-swatch-active {
    border-color: #fff;
    box-shadow: 0 0 0 2px rgba(0, 0, 0, 0.3);
  }

  .drawing-color-custom-wrap {
    position: relative;
    grid-column: 1 / -1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.25rem;
    width: 100%;
    height: 1.75rem;
    border: 1px solid rgba(255, 255, 255, 0.2);
    border-radius: 0.45rem;
    background: rgba(255, 255, 255, 0.08);
    color: rgba(255, 255, 255, 0.78);
    cursor: pointer;
    transition:
      background 150ms ease,
      border-color 150ms ease,
      color 150ms ease;
  }

  .drawing-color-custom-wrap:hover {
    border-color: rgba(255, 255, 255, 0.34);
    background: rgba(255, 255, 255, 0.14);
    color: #fff;
  }

  .drawing-color-custom-preview {
    width: 0.8rem;
    height: 0.8rem;
    border-radius: 0.2rem;
    border: 1px solid rgba(255, 255, 255, 0.38);
    box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.25);
  }

  .drawing-color-custom {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    opacity: 0;
    cursor: pointer;
  }
</style>
