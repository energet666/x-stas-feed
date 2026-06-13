<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { Activity, Check, CircleHelp, CloudOff, FlipHorizontal2, Hand, HandGrab, History, Images, LoaderCircle, Minus, Palette, Pencil, Plus, RotateCw, Undo2, X } from 'lucide-svelte';
  import {
    createBoardImage,
    createBoardImageFromAsset,
    createBoardAsset,
    createBoardOperations,
    createStroke,
    fetchBoard,
    fetchBoardAssets,
    type BoardAsset,
    type BoardImage,
    type BoardImageInput,
    type BoardOperation,
    type BoardOperationInput,
    type Stroke,
    type StrokeInput
  } from '../lib/feed';
  import { boardEvents } from '../lib/board_events.svelte';
  import { uiText as t } from '../lib/ui_text';

  let {
    mediaId,
    expanded = false,
    username = t.common.guest,
    ambientCanvas,
    previewFill = false,
    debugToolsEnabled = false,
    onClose
  }: {
    mediaId: string;
    expanded: boolean;
    username: string;
    ambientCanvas?: HTMLCanvasElement;
    previewFill?: boolean;
    debugToolsEnabled?: boolean;
    onClose?: () => void;
  } = $props();

  type Tool = 'pan' | 'freeform' | 'line';

  const FIXED_COLORS = ['#ffffff', '#000000', '#60a5fa', '#f87171', '#4ade80', '#facc15'];
  const DEFAULT_CUSTOM_COLORS = ['#ff4757', '#ffa502', '#ffdd59', '#5352ed', '#a855f7', '#ff6348'];
  const CUSTOM_COLOR_SLOT_COUNT = 6;

  const BRUSH_SIZES = [2, 4, 8, 14, 22];
  const DEBUG_SEGMENT_COLORS = ['#ff4757', '#ffa502', '#ffdd59', '#2ed573', '#1e90ff', '#a855f7'];
  const FREEFORM_POINT_DISTANCE = 3;
  const LINE_MIN_DISTANCE = 1;
  const CANCEL_HINT_DELAY_MS = 350;
  const DEFAULT_FREEFORM_SIMPLIFY_EPSILON = 0.5;
  const MIN_FREEFORM_SIMPLIFY_EPSILON = 0;
  const MAX_FREEFORM_SIMPLIFY_EPSILON = 24;
  const MIN_BRUSH_SIZE = 1;
  const MAX_BRUSH_SIZE = 200;
  const MIN_BRUSH_OPACITY = 0.1;
  const brushColorStorageKey = 'feed-ai:drawing-brush-color';
  const brushSizeStorageKey = 'feed-ai:drawing-brush-size';
  const brushOpacityStorageKey = 'feed-ai:drawing-brush-opacity';
  const customColorsStorageKey = 'feed-ai:drawing-custom-colors';

  let canvasEl = $state<HTMLCanvasElement | undefined>(undefined);
  let previewCanvasEl = $state<HTMLCanvasElement | undefined>(undefined);
  let historyRangeEl = $state<HTMLInputElement | undefined>(undefined);
  
  // Optimization: offscreen canvases
  let gridCanvas: HTMLCanvasElement | undefined;
  let committedCanvas: HTMLCanvasElement | undefined;
  let activeStrokeCanvas: HTMLCanvasElement | undefined;

  let strokes = $state<Stroke[]>([]);
  let operations = $state<BoardOperation[]>([]);
  let strokeIds = new Set<string>();
  let imageIds = new Set<string>();
  const operationImages = new Map<string, HTMLImageElement>();
  const localOperationImages = new Map<string, HTMLImageElement>();
  let currentTool = $state<Tool>('pan');
  let currentColor = $state('#ffffff');
  let customColors = $state([...DEFAULT_CUSTOM_COLORS]);
  let currentSize = $state(4);
  let currentOpacity = $state(1);
  let localMode = $state(false);
  type LocalOperation =
    | { type: 'stroke'; stroke: StrokeInput }
    | { type: 'image'; image: BoardImageInput & { url: string } };
  let localOperations = $state<LocalOperation[]>([]);
  let localStrokesSaving = $state(false);
  let localStrokesError = $state('');
  let isDrawing = $state(false);
  let activeStrokePointerId = $state<number | null>(null);
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
  let boardName = $state<string>(t.common.board);
  let brushCursorVisible = $state(false);
  let brushCursorX = $state(0);
  let brushCursorY = $state(0);
  let brushCursorSize = $state(0);
  let cancelHintVisible = $state(false);
  let cancelHintTimer: ReturnType<typeof setTimeout> | null = null;
  let sizeDragPointerId = $state<number | null>(null);
  let sizeDragStartY = $state(0);
  let sizeDragStartValue = $state(0);
  let sizeDragAppliedDelta = $state(0);
  let sizeDragActive = $state(false);
  let sizeDragSuppressClick = $state(false);
  let backgroundImage = $state<HTMLImageElement | undefined>(undefined);
  type ImageDraft = {
    file?: File;
    assetId?: string;
    url: string;
    revokeURL: boolean;
    x: number;
    y: number;
    width: number;
    height: number;
    rotation: number;
    flipX: boolean;
  };
  let imageDraft = $state<ImageDraft | null>(null);
  let imageDraftSaving = $state(false);
  let assetUploadSaving = $state(false);
  let imageDraftError = $state('');
  let showAssetLibrary = $state(false);
  let showHelp = $state(false);
  let boardAssets = $state<BoardAsset[]>([]);
  let boardAssetsLoading = $state(false);
  let boardAssetsError = $state('');
  let imageTransformPointerId = $state<number | null>(null);
  let imageTransformMode = $state<'move' | 'resize' | 'rotate' | null>(null);
  let imageTransformStart = $state({ x: 0, y: 0, draftX: 0, draftY: 0, width: 0, height: 0, rotation: 0, pointerAngle: 0 });

  let canvasWidth = $state(1200);
  let canvasHeight = $state(800);
  const SIZE_DRAG_STEP_PX = 3;
  const SIZE_DRAG_START_THRESHOLD_PX = 3;
  const NUMBER_INPUT_STEPPER_HIT_WIDTH = 18;
  const BOARD_WHEEL_SIZE_STEP_DELTA = 28;
  const MIN_ZOOM = 1;
  const MAX_ZOOM = 6;
  const ZOOM_STEP = 0.25;
  const WHEEL_ZOOM_SENSITIVITY = 0.002;

  let zoom = $state(MIN_ZOOM);
  let panX = $state(0);
  let panY = $state(0);
  let spacePressed = $state(false);
  let panPointerId = $state<number | null>(null);
  let panStartClientX = $state(0);
  let panStartClientY = $state(0);
  let panStartX = $state(0);
  let panStartY = $state(0);
  let panCursorVisible = $state(false);
  let panCursorX = $state(0);
  let panCursorY = $state(0);
  let lastPointerClientX = $state(0);
  let lastPointerClientY = $state(0);
  let hasPointerPosition = $state(false);
  const drawingToolSelected = $derived(currentTool === 'freeform' || currentTool === 'line');
  const canvasCursor = $derived(
    panPointerId !== null ||
    spacePressed ||
    (!historyMode && (brushCursorVisible || (currentTool === 'pan' && panCursorVisible)))
      ? 'none'
      : 'default'
  );

  onMount(() => {
    loadBrushSettings();
    setupCanvasBuffers(canvasWidth, canvasHeight);

    window.addEventListener('keydown', handleWindowKeydown, { capture: true });
    window.addEventListener('keyup', handleWindowKeyup, { capture: true });
    window.addEventListener('resize', clampPanToViewport);
    void loadBoard();

    return () => {
      window.removeEventListener('keydown', handleWindowKeydown, { capture: true });
      window.removeEventListener('keyup', handleWindowKeyup, { capture: true });
      window.removeEventListener('resize', clampPanToViewport);
      hideCancelHint();
      releaseImageDraftURL(imageDraft);
    };
  });

  // Global SSE subscription
  $effect(() => {
    if (!mediaId) return;
    
    return boardEvents.subscribe((event) => {
      if (event.mediaId === mediaId) {
        if (event.type === 'stroke') {
          appendCommittedStroke(event.stroke);
        } else {
          appendCommittedImage(event.image);
        }
      }
    });
  });

  // Redraw main canvas when state changes
  $effect(() => {
    if (canvasEl || previewCanvasEl || ambientCanvas) {
      requestAnimationFrame(redraw);
    }
  });

  $effect(() => {
    if (!ambientCanvas) return;
    requestAnimationFrame(redraw);
    tick().then(() => {
      requestAnimationFrame(redraw);
    });
  });

  $effect(() => {
    if (!debugToolsEnabled) {
      showDebugSegments = false;
    }
  });

  async function loadBoard() {
    try {
      const data = await fetchBoard(mediaId);
      boardName = data.board.name;
      const width = data.board.canvas?.width;
      const height = data.board.canvas?.height;
      if (
        typeof width === 'number' &&
        typeof height === 'number' &&
        Number.isFinite(width) &&
        Number.isFinite(height) &&
        width > 0 &&
        height > 0
      ) {
        setupCanvasBuffers(Math.round(width), Math.round(height));
      }
      setBackground(data.board.background);
      const loadedIds = new Set(data.operations.map(operationID));
      const sseOperations = operations.filter((operation) => !loadedIds.has(operationID(operation)));
      rebuildCommittedCanvas([...data.operations, ...sseOperations]);
    } catch {
      // Board might not exist yet
    }
  }

  function setupCanvasBuffers(width: number, height: number) {
    if (canvasWidth === width && canvasHeight === height && gridCanvas && committedCanvas && activeStrokeCanvas) {
      return;
    }

    canvasWidth = width;
    canvasHeight = height;

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
  }

  function setBackground(background: { type?: string; url?: string } | undefined) {
    if (background?.type !== 'image' || !background.url) {
      backgroundImage = undefined;
      redraw();
      return;
    }

    const img = new Image();
    img.decoding = 'async';
    img.onload = () => {
      if (backgroundImage === img) redraw();
    };
    img.src = background.url;
    backgroundImage = img;
    redraw();
  }

  function rebuildCommittedCanvas(nextOperations: BoardOperation[]) {
    if (!committedCanvas) return;
    const ctx = committedCanvas.getContext('2d');
    if (!ctx) return;

    const wasViewingLatest = historyStrokeCount >= operations.length;
    operations = nextOperations;
    strokes = nextOperations.flatMap((operation) => operation.type === 'stroke' ? [operation.stroke] : []);
    strokeIds = new Set(strokes.map((stroke) => stroke.id));
    imageIds = new Set(nextOperations.flatMap((operation) => operation.type === 'image' ? [operation.image.id] : []));
    if (!historyMode || wasViewingLatest) {
      historyStrokeCount = nextOperations.length;
    } else {
      historyStrokeCount = Math.min(historyStrokeCount, nextOperations.length);
    }
    ctx.clearRect(0, 0, canvasWidth, canvasHeight);
    for (const operation of nextOperations) {
      drawOperation(ctx, operation);
    }
    redraw();
  }

  function appendCommittedStroke(stroke: Stroke) {
    if (strokeIds.has(stroke.id)) return;
    strokeIds.add(stroke.id);
    const wasViewingLatest = historyStrokeCount >= operations.length;
    strokes = [...strokes, stroke];
    const operation: BoardOperation = { type: 'stroke', stroke };
    operations = [...operations, operation];
    if (!historyMode || wasViewingLatest) {
      historyStrokeCount = operations.length;
    }

    if (!committedCanvas) return;
    const ctx = committedCanvas.getContext('2d');
    if (!ctx) return;

    drawStroke(ctx, stroke.points, stroke.color, stroke.size, stroke.tool, stroke.opacity);
    redraw();
  }

  function appendCommittedImage(image: BoardImage) {
    if (imageIds.has(image.id)) return;
    imageIds.add(image.id);
    const wasViewingLatest = historyStrokeCount >= operations.length;
    operations = [...operations, { type: 'image', image }];
    if (!historyMode || wasViewingLatest) {
      historyStrokeCount = operations.length;
    }
    rebuildCommittedCanvas(operations);
  }

  function operationID(operation: BoardOperation) {
    return operation.type === 'stroke' ? operation.stroke.id : operation.image.id;
  }

  function drawOperation(ctx: CanvasRenderingContext2D, operation: BoardOperation) {
    if (operation.type === 'stroke') {
      const stroke = operation.stroke;
      drawStroke(ctx, stroke.points, stroke.color, stroke.size, stroke.tool, stroke.opacity);
      return;
    }
    const image = operation.image;
    let element = operationImages.get(image.id);
    if (!element) {
      element = new Image();
      element.decoding = 'async';
      element.onload = () => rebuildCommittedCanvas(operations);
      element.src = image.url;
      operationImages.set(image.id, element);
    }
    if (!element.complete || element.naturalWidth <= 0) return;
    ctx.save();
    ctx.translate(image.x + image.width / 2, image.y + image.height / 2);
    ctx.rotate((image.rotation * Math.PI) / 180);
    ctx.scale(image.flipX ? -1 : 1, 1);
    ctx.drawImage(element, -image.width / 2, -image.height / 2, image.width, image.height);
    ctx.restore();
  }


  function getCanvas() {
    return expanded ? canvasEl : previewCanvasEl;
  }

  function getCanvasMetrics() {
    const canvas = getCanvas();
    if (!canvas) return null;
    
    const rect = canvas.getBoundingClientRect();
    const renderedWidth = rect.width;
    const renderedHeight = rect.height;
    const scaleX = canvasWidth / renderedWidth;
    const scaleY = canvasHeight / renderedHeight;

    return { rect, renderedWidth, renderedHeight, offsetX: 0, offsetY: 0, scaleX, scaleY };
  }

  function canvasCoords(event: Pick<PointerEvent, 'clientX' | 'clientY'>): [number, number] {
    const metrics = getCanvasMetrics();
    if (!metrics) return [0, 0];
    
    return [
      (event.clientX - metrics.rect.left - metrics.offsetX) * metrics.scaleX,
      (event.clientY - metrics.rect.top - metrics.offsetY) * metrics.scaleY
    ];
  }

  function isPointerInsideRenderedCanvas(event: Pick<MouseEvent, 'clientX' | 'clientY'>) {
    const metrics = getCanvasMetrics();
    if (!metrics) return false;

    const x = event.clientX - metrics.rect.left;
    const y = event.clientY - metrics.rect.top;

    return (
      x >= metrics.offsetX &&
      x <= metrics.offsetX + metrics.renderedWidth &&
      y >= metrics.offsetY &&
      y <= metrics.offsetY + metrics.renderedHeight
    );
  }

  function updateBrushCursor(event: Pick<MouseEvent, 'clientX' | 'clientY'>) {
    if (!expanded) return;
    updatePanCursorPosition(event);
    if (!drawingToolSelected || spacePressed || panPointerId !== null) {
      brushCursorVisible = false;
      return;
    }

    const metrics = getCanvasMetrics();
    if (!metrics) return;

    if (!isPointerInsideRenderedCanvas(event)) {
      brushCursorVisible = false;
      return;
    }

    const wrap = canvasEl?.parentElement?.parentElement;
    if (!wrap) return;

    const wrapRect = wrap.getBoundingClientRect();

    brushCursorVisible = true;
    brushCursorX = event.clientX - wrapRect.left;
    brushCursorY = event.clientY - wrapRect.top;
    updateBrushCursorSize();
  }

  function updatePanCursorPosition(event: Pick<MouseEvent, 'clientX' | 'clientY'>) {
    const wrap = canvasEl?.parentElement?.parentElement;
    if (!wrap) return;

    lastPointerClientX = event.clientX;
    lastPointerClientY = event.clientY;
    hasPointerPosition = true;
    const wrapRect = wrap.getBoundingClientRect();
    panCursorX = event.clientX - wrapRect.left;
    panCursorY = event.clientY - wrapRect.top;
    panCursorVisible =
      panCursorX >= 0 &&
      panCursorX <= wrapRect.width &&
      panCursorY >= 0 &&
      panCursorY <= wrapRect.height;
  }

  function updateBrushCursorSize() {
    const stageWidth = canvasEl?.parentElement?.offsetWidth;
    brushCursorSize =
      stageWidth && canvasWidth > 0
        ? Math.max(2, currentSize * (stageWidth / canvasWidth) * zoom)
        : currentSize;
  }

  function updateCancelHint(pointerInsideCanvas: boolean) {
    if (!isDrawing || pointerInsideCanvas) {
      hideCancelHint();
      return;
    }

    if (cancelHintVisible || cancelHintTimer) return;

    cancelHintTimer = setTimeout(() => {
      cancelHintTimer = null;
      if (isDrawing) {
        cancelHintVisible = true;
      }
    }, CANCEL_HINT_DELAY_MS);
  }

  function hideCancelHint() {
    if (cancelHintTimer) {
      clearTimeout(cancelHintTimer);
      cancelHintTimer = null;
    }
    cancelHintVisible = false;
  }

  function handlePointerEnter(event: PointerEvent) {
    if (historyMode) return;
    updateBrushCursor(event);
  }

  function handlePointerLeave() {
    if (!isDrawing) {
      brushCursorVisible = false;
      hideCancelHint();
    }
    if (panPointerId === null) {
      panCursorVisible = false;
      hasPointerPosition = false;
    }
  }

  function handlePointerDown(event: PointerEvent) {
    if (!expanded) return;
    if (localStrokesSaving || assetUploadSaving) return;
    if (
      event.button === 1 ||
      (event.button === 0 && (spacePressed || (!historyMode && currentTool === 'pan')))
    ) {
      beginPan(event);
      return;
    }
    if (historyMode) return;
    if (event.button !== 0) return;

    updateBrushCursor(event);
    hideCancelHint();
    const canvas = getCanvas();
    if (!canvas) return;
    canvas.setPointerCapture(event.pointerId);

    const [x, y] = canvasCoords(event);

    if (currentTool === 'freeform') {
      isDrawing = true;
      activeStrokePointerId = event.pointerId;
      currentPoints = [[x, y]];
      if (activeStrokeCanvas) {
        const ctx = activeStrokeCanvas.getContext('2d')!;
        ctx.clearRect(0, 0, canvasWidth, canvasHeight);
      }
    } else if (currentTool === 'line') {
      isDrawing = true;
      activeStrokePointerId = event.pointerId;
      lineStart = [x, y];
      mousePos = [x, y];
      redraw();
    }
  }

  function handlePointerMove(event: PointerEvent) {
    if (!expanded) return;
    if (panPointerId === event.pointerId) {
      updatePan(event);
      return;
    }
    if (historyMode) return;

    updateBrushCursor(event);
    const pointerInsideCanvas = isPointerInsideRenderedCanvas(event);

    const [x, y] = canvasCoords(event);

    if (currentTool === 'line' && isDrawing && lineStart) {
      mousePos = [x, y];
      updateCancelHint(pointerInsideCanvas);
      redraw();
      return;
    }

    if (!isDrawing || currentTool !== 'freeform' || !activeStrokeCanvas) return;
    updateCancelHint(pointerInsideCanvas);

    const lastPoint = currentPoints[currentPoints.length - 1];
    const dist = Math.hypot(x - lastPoint[0], y - lastPoint[1]);
    
    // Filter points that are too close to reduce data and complexity
    if (dist < FREEFORM_POINT_DISTANCE) return;

    const newPoints = [...currentPoints, [x, y]];

    const ctx = activeStrokeCanvas.getContext('2d')!;
    ctx.clearRect(0, 0, canvasWidth, canvasHeight);
    if (debugToolsEnabled && showDebugSegments) {
      drawDebugSegmentedFreeformStroke(ctx, newPoints, currentSize);
    } else {
      drawStroke(ctx, newPoints, currentColor, currentSize, 'freeform', currentOpacity);
    }

    currentPoints = newPoints;
    redraw();
  }

  function handlePointerUp(event: PointerEvent) {
    if (!expanded) return;
    if (panPointerId === event.pointerId) {
      finishPan(event);
      return;
    }
    if (historyMode) return;

    const pointerEndedInsideCanvas = isPointerInsideRenderedCanvas(event);
    updateBrushCursor(event);
    hideCancelHint();
    const canvas = getCanvas();
    if (canvas?.hasPointerCapture(event.pointerId)) {
      canvas.releasePointerCapture(event.pointerId);
    }

    if (currentTool === 'freeform' && isDrawing && !pointerEndedInsideCanvas) {
      clearActiveStroke();
      return;
    }

    if (currentTool === 'line' && lineStart && !pointerEndedInsideCanvas) {
      clearActiveStroke();
      return;
    }

    if (currentTool === 'line' && isDrawing && lineStart) {
      const [x, y] = canvasCoords(event);
      const lineDistance = Math.hypot(x - lineStart[0], y - lineStart[1]);
      if (lineDistance < LINE_MIN_DISTANCE) {
        clearActiveStroke();
        return;
      }

      const pts: number[][] = [lineStart, [x, y]];
      isDrawing = false;
      activeStrokePointerId = null;
      lineStart = null;
      mousePos = null;
      void submitStroke('line', pts);
      redraw();
      return;
    }

    if (currentTool === 'freeform' && isDrawing && currentPoints.length >= 1) {
      isDrawing = false;
      activeStrokePointerId = null;
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
      activeStrokePointerId = null;
      currentPoints = [];
      if (activeStrokeCanvas) {
        activeStrokeCanvas.getContext('2d')!.clearRect(0, 0, canvasWidth, canvasHeight);
      }
    }
  }

  function handlePointerCancel(event: PointerEvent) {
    if (panPointerId === event.pointerId) {
      finishPan(event);
      return;
    }
    clearActiveStroke();
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
    const stroke = {
      tool,
      points,
      color: currentColor,
      size: currentSize,
      opacity: currentOpacity,
      author: username
    };
    if (localMode) {
      localOperations = [...localOperations, { type: 'stroke', stroke }];
      localStrokesError = '';
      redraw();
      return;
    }
    try {
      await createStroke(mediaId, tool, points, stroke.color, stroke.size, stroke.opacity, stroke.author);
    } catch {
      // Failed to submit stroke
    }
  }

  function toggleLocalMode() {
    if (localStrokesSaving || assetUploadSaving || imageDraft) return;
    if (localMode) {
      cancelLocalMode();
      return;
    }
    localMode = true;
    localStrokesError = '';
    showAssetLibrary = false;
    showColorPicker = false;
  }

  function undoLocalStroke() {
    if (!localMode || localStrokesSaving || assetUploadSaving || imageDraft || localOperations.length === 0) return;
    localOperations = localOperations.slice(0, -1);
    localStrokesError = '';
    redraw();
  }

  function cancelLocalMode() {
    if (!localMode || localStrokesSaving || assetUploadSaving) return;
    cancelImageDraft();
    clearActiveStroke();
    localMode = false;
    localOperations = [];
    localStrokesError = '';
    redraw();
  }

  async function publishLocalStrokes() {
    if (!localMode || localStrokesSaving || assetUploadSaving || imageDraft) return;
    if (localOperations.length === 0) {
      localStrokesError = t.board.localStrokePublishEmpty;
      return;
    }

    localStrokesSaving = true;
    localStrokesError = '';
    const drafts: BoardOperationInput[] = localOperations.map((operation) => {
      if (operation.type === 'stroke') return operation;
      const { url: _, ...image } = operation.image;
      return { type: 'image', image };
    });
    try {
      const created = await createBoardOperations(mediaId, drafts);
      for (const operation of created) {
        if (operation.type === 'stroke') {
          appendCommittedStroke(operation.stroke);
        } else {
          appendCommittedImage(operation.image);
        }
      }
      localOperations = [];
    } catch (error) {
      localStrokesError = error instanceof Error ? error.message : t.board.localStrokePublishFailed;
    } finally {
      localStrokesSaving = false;
      redraw();
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
    if (canvas) {
      drawBoardToCanvas(canvas, { includeActiveStroke: true, includeHistoryMode: expanded });
    }
    if (ambientCanvas) {
      drawBoardToCanvas(ambientCanvas, { includeActiveStroke: false, includeHistoryMode: false });
    }
  }

  function drawBoardToCanvas(
    canvas: HTMLCanvasElement,
    options: { includeActiveStroke: boolean; includeHistoryMode: boolean }
  ) {
    if (canvas.width !== canvasWidth) {
      canvas.width = canvasWidth;
    }
    if (canvas.height !== canvasHeight) {
      canvas.height = canvasHeight;
    }

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    // 1. Draw static background
    if (backgroundImage?.complete && backgroundImage.naturalWidth > 0) {
      ctx.fillStyle = '#0f0f17';
      ctx.fillRect(0, 0, canvasWidth, canvasHeight);
      ctx.drawImage(backgroundImage, 0, 0, canvasWidth, canvasHeight);
    } else if (gridCanvas) {
      ctx.drawImage(gridCanvas, 0, 0);
    } else {
      ctx.fillStyle = '#0f0f17';
      ctx.fillRect(0, 0, canvasWidth, canvasHeight);
    }

    if (historyMode && options.includeHistoryMode) {
      for (const operation of operations.slice(0, historyStrokeCount)) {
        drawOperation(ctx, operation);
      }
      return;
    }

    // 2. Draw all committed strokes from buffer
    if (committedCanvas) {
      ctx.drawImage(committedCanvas, 0, 0);
    }

    // 3. Draw strokes kept in the current local session.
    if (options.includeActiveStroke) {
      for (const operation of localOperations) {
        drawLocalOperation(ctx, operation);
      }
    }

    // 4. Draw active stroke canvas (incremental)
    if (options.includeActiveStroke && isDrawing && activeStrokeCanvas) {
      ctx.drawImage(activeStrokeCanvas, 0, 0);
    }

    // 5. Draw line preview (not incremental but very few points)
    if (options.includeActiveStroke && currentTool === 'line' && lineStart && mousePos) {
      drawStroke(ctx, [lineStart, mousePos], currentColor, currentSize, 'line', currentOpacity);
    }
  }

  function drawLocalOperation(ctx: CanvasRenderingContext2D, operation: LocalOperation) {
    if (operation.type === 'stroke') {
      const stroke = operation.stroke;
      drawStroke(ctx, stroke.points, stroke.color, stroke.size, stroke.tool, stroke.opacity);
      return;
    }

    const image = operation.image;
    let element = localOperationImages.get(image.url);
    if (!element) {
      element = new Image();
      element.decoding = 'async';
      element.onload = redraw;
      element.src = image.url;
      localOperationImages.set(image.url, element);
    }
    if (!element.complete || element.naturalWidth <= 0) return;
    ctx.save();
    ctx.translate(image.x + image.width / 2, image.y + image.height / 2);
    ctx.rotate((image.rotation * Math.PI) / 180);
    ctx.scale(image.flipX ? -1 : 1, 1);
    ctx.drawImage(element, -image.width / 2, -image.height / 2, image.width, image.height);
    ctx.restore();
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

  function drawPoint(ctx: CanvasRenderingContext2D, point: number[], color: string, size: number, opacity = 1) {
    ctx.save();
    ctx.globalAlpha = normalizedOpacity(opacity);
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
    ctx.globalAlpha = currentOpacity;
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
    tool: string,
    opacity = 1
  ) {
    if (points.length === 0) return;
    if (points.length === 1) {
      drawPoint(ctx, points[0], color, size, opacity);
      return;
    }

    ctx.save();
    ctx.globalAlpha = normalizedOpacity(opacity);
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

  function normalizedOpacity(opacity: number | undefined) {
    return typeof opacity === 'number' && Number.isFinite(opacity) && opacity > 0 && opacity <= 1
      ? opacity
      : 1;
  }

  function colorWithOpacity(color: string, opacity: number) {
    const normalizedColor = normalizeHexColor(color);
    const normalizedAlpha = normalizedOpacity(opacity);
    if (!normalizedColor || normalizedAlpha === 1) return 'transparent';

    const red = Number.parseInt(normalizedColor.slice(1, 3), 16);
    const green = Number.parseInt(normalizedColor.slice(3, 5), 16);
    const blue = Number.parseInt(normalizedColor.slice(5, 7), 16);
    return `rgba(${red}, ${green}, ${blue}, ${normalizedAlpha})`;
  }

  function selectTool(tool: Tool) {
    clearActiveStroke();
    currentTool = tool;
    lineStart = null;
    mousePos = null;
    brushCursorVisible = false;
    hideCancelHint();
    showColorPicker = false;
    if (tool === 'pan' && hasPointerPosition) {
      updatePanCursorPosition({
        clientX: lastPointerClientX,
        clientY: lastPointerClientY
      });
    }
    redraw();
  }

  function activateFreeformForBrushSettings() {
    if (currentTool === 'pan') {
      selectTool('freeform');
    }
  }

  function toggleColorPicker() {
    const nextOpen = !showColorPicker;
    activateFreeformForBrushSettings();
    showAssetLibrary = false;
    showColorPicker = nextOpen;
  }

  function selectColor(color: string) {
    activateFreeformForBrushSettings();
    currentColor = color;
    saveBrushColor(color);
    showColorPicker = false;
  }

  function setCustomColor(color: string) {
    const normalizedColor = normalizeHexColor(color);
    if (!normalizedColor) return;

    activateFreeformForBrushSettings();
    currentColor = normalizedColor;
    saveBrushColor(normalizedColor);
  }

  function selectCustomColor(color: string) {
    const normalizedColor = normalizeHexColor(color);
    if (!normalizedColor) return;

    addCustomColor(normalizedColor);
    selectColor(normalizedColor);
  }

  function addCustomColor(color: string) {
    const normalizedColor = normalizeHexColor(color);
    if (
      !normalizedColor ||
      FIXED_COLORS.includes(normalizedColor) ||
      customColors.includes(normalizedColor)
    ) {
      return;
    }

    customColors = [...customColors.slice(1), normalizedColor].slice(-CUSTOM_COLOR_SLOT_COUNT);
    saveCustomColors(customColors);
  }

  function selectSize(size: number) {
    activateFreeformForBrushSettings();
    const normalizedSize = Math.min(MAX_BRUSH_SIZE, Math.max(MIN_BRUSH_SIZE, Math.round(size)));
    currentSize = normalizedSize;
    saveBrushSize(normalizedSize);
    if (brushCursorVisible && canvasEl) {
      updateBrushCursorSize();
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

  function selectOpacity(opacity: number) {
    activateFreeformForBrushSettings();
    currentOpacity = Math.min(1, Math.max(MIN_BRUSH_OPACITY, Math.round(opacity * 100) / 100));
    try {
      window.localStorage.setItem(brushOpacityStorageKey, String(currentOpacity));
    } catch {
      // Ignore storage failures; the selected opacity still applies in memory.
    }
    redraw();
  }

  function handleOpacityInput(event: Event) {
    selectOpacity((event.currentTarget as HTMLInputElement).valueAsNumber / 100);
  }

  function handleBoardWheel(event: WheelEvent) {
    if (!expanded) return;

    const delta = event.deltaY || event.deltaX;
    if (delta === 0) return;

    if (event.ctrlKey && drawingToolSelected && !historyMode) {
      event.preventDefault();
      event.stopPropagation();
      const step = Math.max(1, Math.round(Math.abs(delta) / BOARD_WHEEL_SIZE_STEP_DELTA));
      selectSize(currentSize + (delta < 0 ? step : -step));
      updateBrushCursor(event);
      return;
    }

    event.preventDefault();
    event.stopPropagation();
    const nextZoom = zoom * Math.exp(-delta * WHEEL_ZOOM_SENSITIVITY);
    setZoom(nextZoom, event.clientX, event.clientY);
    updateBrushCursor(event);
  }

  function handleAssetLibraryWheel(event: WheelEvent) {
    event.stopPropagation();
  }

  function handleHelpWheel(event: WheelEvent) {
    event.stopPropagation();
  }

  function beginPan(event: PointerEvent) {
    clearActiveStroke();
    updatePanCursorPosition(event);
    panPointerId = event.pointerId;
    panStartClientX = event.clientX;
    panStartClientY = event.clientY;
    panStartX = panX;
    panStartY = panY;
    brushCursorVisible = false;
    const canvas = getCanvas();
    canvas?.setPointerCapture(event.pointerId);
    event.preventDefault();
  }

  function updatePan(event: PointerEvent) {
    updatePanCursorPosition(event);
    panX = panStartX + event.clientX - panStartClientX;
    panY = panStartY + event.clientY - panStartClientY;
    clampPanToViewport();
    event.preventDefault();
  }

  function finishPan(event: PointerEvent) {
    const canvas = getCanvas();
    if (canvas?.hasPointerCapture(event.pointerId)) {
      canvas.releasePointerCapture(event.pointerId);
    }
    panPointerId = null;
    updateBrushCursor(event);
  }

  function setZoom(value: number, anchorClientX?: number, anchorClientY?: number) {
    const nextZoom = clampZoom(value);
    if (nextZoom === zoom) return;

    const wrapRect = canvasEl?.parentElement?.parentElement?.getBoundingClientRect();
    if (wrapRect && anchorClientX !== undefined && anchorClientY !== undefined) {
      const anchorX = anchorClientX - (wrapRect.left + wrapRect.width / 2);
      const anchorY = anchorClientY - (wrapRect.top + wrapRect.height / 2);
      const ratio = nextZoom / zoom;
      panX = anchorX - ratio * (anchorX - panX);
      panY = anchorY - ratio * (anchorY - panY);
    }

    zoom = nextZoom;
    clampPanToViewport();
    if (brushCursorVisible) updateBrushCursorSize();
  }

  function resetZoom() {
    zoom = MIN_ZOOM;
    panX = 0;
    panY = 0;
    if (brushCursorVisible) updateBrushCursorSize();
  }

  function clampZoom(value: number) {
    return Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, Math.round(value * 100) / 100));
  }

  function clampPanToViewport() {
    const wrap = canvasEl?.parentElement?.parentElement;
    if (!wrap) return;

    const wrapRect = wrap.getBoundingClientRect();
    const canvasAspect = canvasWidth / canvasHeight;
    let baseWidth = wrapRect.width;
    let baseHeight = baseWidth / canvasAspect;
    if (baseHeight > wrapRect.height) {
      baseHeight = wrapRect.height;
      baseWidth = baseHeight * canvasAspect;
    }

    const maxPanX = Math.max(0, (baseWidth * zoom - wrapRect.width) / 2);
    const maxPanY = Math.max(0, (baseHeight * zoom - wrapRect.height) / 2);
    panX = Math.min(maxPanX, Math.max(-maxPanX, panX));
    panY = Math.min(maxPanY, Math.max(-maxPanY, panY));
  }

  function handleCustomSizePointerDown(event: PointerEvent) {
    if (event.button !== 0) return;

    activateFreeformForBrushSettings();
    const input = event.currentTarget as HTMLInputElement;
    if (isCustomSizeStepperPress(input, event)) return;

    sizeDragPointerId = event.pointerId;
    sizeDragStartY = event.clientY;
    sizeDragStartValue = currentSize;
    sizeDragAppliedDelta = 0;
    sizeDragActive = false;
    sizeDragSuppressClick = false;
    input.setPointerCapture(event.pointerId);
  }

  function isCustomSizeStepperPress(input: HTMLInputElement, event: PointerEvent) {
    const rect = input.getBoundingClientRect();
    return event.clientX >= rect.right - NUMBER_INPUT_STEPPER_HIT_WIDTH;
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

  function clearActiveStroke() {
    let cleared = false;

    if (isDrawing || currentPoints.length > 0) {
      isDrawing = false;
      currentPoints = [];
      cleared = true;
    }

    hideCancelHint();

    if (currentTool === 'line' && lineStart) {
      lineStart = null;
      mousePos = null;
      cleared = true;
    }

    if (activeStrokeCanvas) {
      activeStrokeCanvas.getContext('2d')!.clearRect(0, 0, canvasWidth, canvasHeight);
    }

    const canvas = getCanvas();
    if (canvas && activeStrokePointerId !== null && canvas.hasPointerCapture(activeStrokePointerId)) {
      canvas.releasePointerCapture(activeStrokePointerId);
    }
    activeStrokePointerId = null;

    if (cleared) {
      redraw();
    }

    return cleared;
  }

  function enterHistoryMode() {
    if (localMode) return;
    historyMode = true;
    historyStrokeCount = operations.length;
    cancelImageDraft();
    showAssetLibrary = false;
    lineStart = null;
    mousePos = null;
    isDrawing = false;
    activeStrokePointerId = null;
    currentPoints = [];
    showColorPicker = false;
    brushCursorVisible = false;
    hideCancelHint();
    if (activeStrokeCanvas) {
      activeStrokeCanvas.getContext('2d')!.clearRect(0, 0, canvasWidth, canvasHeight);
    }
    redraw();
  }

  function exitHistoryMode() {
    historyMode = false;
    historyStrokeCount = operations.length;
    redraw();
  }

  function handleHistoryRangeInput(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const value = Number.isFinite(input.valueAsNumber) ? input.valueAsNumber : operations.length;
    setHistoryStrokeCount(value);
  }

  function handleHistoryRangePointerDown(event: PointerEvent) {
    (event.currentTarget as HTMLInputElement).focus({ preventScroll: true });
  }

  function handleHistoryRangeKeydown(event: KeyboardEvent) {
    applyHistoryKey(event);
  }

  function handleWindowKeydown(event: KeyboardEvent) {
    if (!expanded) return;

    if (event.key === 'Escape' && showHelp) {
      showHelp = false;
      event.preventDefault();
      event.stopPropagation();
      event.stopImmediatePropagation();
      return;
    }

    if (event.key === 'Escape' && showAssetLibrary) {
      showAssetLibrary = false;
      event.preventDefault();
      event.stopPropagation();
      event.stopImmediatePropagation();
      return;
    }

    if (event.key === 'Escape' && cancelImageDraft()) {
      event.preventDefault();
      event.stopPropagation();
      event.stopImmediatePropagation();
      return;
    }

    if (event.code === 'Space' && !isEditableKeyboardTarget(event.target)) {
      spacePressed = true;
      brushCursorVisible = false;
      event.preventDefault();
    }

    if (
      localMode &&
      !isEditableKeyboardTarget(event.target) &&
      (event.metaKey || event.ctrlKey) &&
      event.key.toLowerCase() === 'z'
    ) {
      undoLocalStroke();
      event.preventDefault();
      event.stopPropagation();
      event.stopImmediatePropagation();
      return;
    }

    if (event.key === 'Escape' && clearActiveStroke()) {
      event.preventDefault();
      event.stopPropagation();
      event.stopImmediatePropagation();
      return;
    }

    if (!historyMode && !isEditableKeyboardTarget(event.target)) {
      if (event.key === '+' || event.key === '=') {
        setZoom(zoom + ZOOM_STEP);
      } else if (event.key === '-') {
        setZoom(zoom - ZOOM_STEP);
      } else if (event.key === '0') {
        resetZoom();
      } else {
        return;
      }
      event.preventDefault();
      event.stopPropagation();
      event.stopImmediatePropagation();
      return;
    }

    if (!historyMode) return;
    if (isEditableKeyboardTarget(event.target)) return;
    applyHistoryKey(event);
  }

  function handleWindowKeyup(event: KeyboardEvent) {
    if (event.code === 'Space') {
      spacePressed = false;
      if (!historyMode && panPointerId === null && hasPointerPosition) {
        updateBrushCursor({
          clientX: lastPointerClientX,
          clientY: lastPointerClientY
        });
      }
    }
  }

  function applyHistoryKey(event: KeyboardEvent) {
    if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') {
      setHistoryStrokeCount(historyStrokeCount - 1);
    } else if (event.key === 'ArrowRight' || event.key === 'ArrowUp') {
      setHistoryStrokeCount(historyStrokeCount + 1);
    } else if (event.key === 'Home') {
      setHistoryStrokeCount(0);
    } else if (event.key === 'End') {
      setHistoryStrokeCount(operations.length);
    } else {
      return;
    }

    event.preventDefault();
    event.stopPropagation();
    event.stopImmediatePropagation();
    historyRangeEl?.focus({ preventScroll: true });
  }

  function setHistoryStrokeCount(value: number) {
    historyStrokeCount = Math.min(operations.length, Math.max(0, Math.round(value)));
    redraw();
  }

  function isEditableKeyboardTarget(target: EventTarget | null) {
    if (!(target instanceof HTMLElement)) return false;
    return Boolean(target.closest('input:not([type="range"]), textarea, select, [contenteditable="true"]'));
  }

  function historyLastAuthor() {
    if (historyStrokeCount <= 0) return '—';
    const operation = operations[historyStrokeCount - 1];
    const author = operation?.type === 'stroke' ? operation.stroke.author.trim() : operation?.image.author.trim();
    return author || t.common.guest;
  }

  function loadBrushSettings() {
    try {
      const storedColor = window.localStorage.getItem(brushColorStorageKey);
      const normalizedStoredColor = storedColor ? normalizeHexColor(storedColor) : null;
      if (normalizedStoredColor) {
        currentColor = normalizedStoredColor;
      }

      const storedSize = Number.parseInt(window.localStorage.getItem(brushSizeStorageKey) ?? '', 10);
      if (Number.isFinite(storedSize)) {
        currentSize = Math.min(MAX_BRUSH_SIZE, Math.max(MIN_BRUSH_SIZE, storedSize));
      }

      const storedOpacity = Number.parseFloat(
        window.localStorage.getItem(brushOpacityStorageKey) ?? ''
      );
      if (Number.isFinite(storedOpacity)) {
        currentOpacity = Math.min(1, Math.max(MIN_BRUSH_OPACITY, storedOpacity));
      }

      const storedCustomColors = parseStoredCustomColors(
        window.localStorage.getItem(customColorsStorageKey)
      );
      if (storedCustomColors.length > 0) {
        customColors = [
          ...storedCustomColors,
          ...DEFAULT_CUSTOM_COLORS.filter((color) => !storedCustomColors.includes(color))
        ].slice(0, CUSTOM_COLOR_SLOT_COUNT);
      }
    } catch {
      // Ignore storage failures; drawing controls should keep working in memory.
    }
  }

  function parseStoredCustomColors(value: string | null) {
    if (!value) return [];

    try {
      const parsed = JSON.parse(value);
      if (!Array.isArray(parsed)) return [];

      const colors: string[] = [];
      for (const entry of parsed) {
        if (typeof entry !== 'string') continue;

        const color = normalizeHexColor(entry);
        if (!color || FIXED_COLORS.includes(color) || colors.includes(color)) continue;

        colors.push(color);
        if (colors.length >= CUSTOM_COLOR_SLOT_COUNT) break;
      }

      return colors;
    } catch {
      return [];
    }
  }

  function normalizeHexColor(color: string) {
    const normalizedColor = color.trim().toLowerCase();
    return /^#[0-9a-f]{6}$/.test(normalizedColor) ? normalizedColor : null;
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

  function saveCustomColors(colors: string[]) {
    try {
      window.localStorage.setItem(customColorsStorageKey, JSON.stringify(colors));
    } catch {
      // Ignore storage failures; the custom palette still applies in memory.
    }
  }

  function handleBoardDragEnter(event: DragEvent) {
    if (!hasBoardDraggedFiles(event)) return;
    event.preventDefault();
    event.stopPropagation();
  }

  function handleBoardDragOver(event: DragEvent) {
    if (!hasBoardDraggedFiles(event)) return;
    event.preventDefault();
    event.stopPropagation();
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = historyMode || imageDraftSaving || assetUploadSaving ? 'none' : 'copy';
    }
  }

  async function handleBoardDrop(event: DragEvent) {
    if (!hasBoardDraggedFiles(event)) return;
    event.preventDefault();
    event.stopPropagation();
    if (historyMode || imageDraftSaving || assetUploadSaving) return;
    const file = Array.from(event.dataTransfer?.files ?? []).find((entry) => entry.type.startsWith('image/'));
    if (!file) {
      imageDraftError = 'На доску можно добавить только изображение';
      return;
    }
    const [dropX, dropY] = canvasCoords(event);
    if (localMode) {
      assetUploadSaving = true;
      imageDraftError = '';
      localStrokesError = '';
      try {
        const asset = await createBoardAsset(file);
        boardAssets = [asset, ...boardAssets.filter((entry) => entry.id !== asset.id)];
        startImageDraft(asset.url, { assetId: asset.id, revokeURL: false, centerX: dropX, centerY: dropY });
      } catch (error) {
        localStrokesError = error instanceof Error ? error.message : t.board.assetUploadFailed;
      } finally {
        assetUploadSaving = false;
      }
      return;
    }
    const url = URL.createObjectURL(file);
    startImageDraft(url, { file, revokeURL: true, centerX: dropX, centerY: dropY });
  }

  function startImageDraft(
    url: string,
    source: { file?: File; assetId?: string; revokeURL: boolean; centerX?: number; centerY?: number }
  ) {
    const image = new Image();
    image.onload = () => {
      releaseImageDraftURL(imageDraft);
      const maxWidth = canvasWidth * 0.45;
      const maxHeight = canvasHeight * 0.45;
      const scale = Math.min(maxWidth / image.naturalWidth, maxHeight / image.naturalHeight, 1);
      const width = Math.max(20, image.naturalWidth * scale);
      const height = Math.max(20, image.naturalHeight * scale);
      const centerX = source.centerX ?? canvasWidth / 2;
      const centerY = source.centerY ?? canvasHeight / 2;
      imageDraft = {
        file: source.file,
        assetId: source.assetId,
        url,
        revokeURL: source.revokeURL,
        x: centerX - width / 2,
        y: centerY - height / 2,
        width,
        height,
        rotation: 0,
        flipX: false
      };
      imageDraftError = '';
      clearActiveStroke();
      currentTool = 'pan';
      showAssetLibrary = false;
    };
    image.onerror = () => {
      if (source.revokeURL) URL.revokeObjectURL(url);
      imageDraftError = 'Не удалось прочитать изображение';
    };
    image.src = url;
  }

  async function toggleAssetLibrary() {
    showAssetLibrary = !showAssetLibrary;
    showColorPicker = false;
    showHelp = false;
    if (!showAssetLibrary) return;
    await loadBoardAssets();
  }

  function toggleHelp() {
    showHelp = !showHelp;
    if (!showHelp) return;
    showAssetLibrary = false;
    showColorPicker = false;
  }

  async function loadBoardAssets() {
    if (boardAssetsLoading) return;
    boardAssetsLoading = true;
    boardAssetsError = '';
    try {
      boardAssets = await fetchBoardAssets();
    } catch (error) {
      boardAssetsError = error instanceof Error ? error.message : 'Не удалось загрузить ассеты';
    } finally {
      boardAssetsLoading = false;
    }
  }

  function selectBoardAsset(asset: BoardAsset) {
    startImageDraft(asset.url, { assetId: asset.id, revokeURL: false });
  }

  function hasBoardDraggedFiles(event: DragEvent) {
    return Array.from(event.dataTransfer?.types ?? []).includes('Files');
  }

  function beginImageTransform(event: PointerEvent, mode: 'move' | 'resize' | 'rotate') {
    if (!imageDraft || imageDraftSaving || event.button !== 0) return;
    event.preventDefault();
    event.stopPropagation();
    const [x, y] = canvasCoords(event);
    const centerX = imageDraft.x + imageDraft.width / 2;
    const centerY = imageDraft.y + imageDraft.height / 2;
    imageTransformPointerId = event.pointerId;
    imageTransformMode = mode;
    imageTransformStart = {
      x,
      y,
      draftX: imageDraft.x,
      draftY: imageDraft.y,
      width: imageDraft.width,
      height: imageDraft.height,
      rotation: imageDraft.rotation,
      pointerAngle: Math.atan2(y - centerY, x - centerX)
    };
    (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
  }

  function updateImageTransform(event: PointerEvent) {
    if (!imageDraft || imageTransformPointerId !== event.pointerId || !imageTransformMode) return;
    event.preventDefault();
    event.stopPropagation();
    const [x, y] = canvasCoords(event);
    if (imageTransformMode === 'move') {
      imageDraft.x = imageTransformStart.draftX + x - imageTransformStart.x;
      imageDraft.y = imageTransformStart.draftY + y - imageTransformStart.y;
      return;
    }
    const centerX = imageTransformStart.draftX + imageTransformStart.width / 2;
    const centerY = imageTransformStart.draftY + imageTransformStart.height / 2;
    if (imageTransformMode === 'rotate') {
      const angle = Math.atan2(y - centerY, x - centerX);
      imageDraft.rotation = imageTransformStart.rotation + ((angle - imageTransformStart.pointerAngle) * 180) / Math.PI;
      return;
    }
    const startDistance = Math.hypot(imageTransformStart.x - centerX, imageTransformStart.y - centerY);
    const distance = Math.hypot(x - centerX, y - centerY);
    const scale = Math.min(10, Math.max(20 / Math.min(imageTransformStart.width, imageTransformStart.height), distance / Math.max(1, startDistance)));
    imageDraft.width = imageTransformStart.width * scale;
    imageDraft.height = imageTransformStart.height * scale;
    imageDraft.x = centerX - imageDraft.width / 2;
    imageDraft.y = centerY - imageDraft.height / 2;
  }

  function finishImageTransform(event: PointerEvent) {
    if (imageTransformPointerId !== event.pointerId) return;
    const target = event.currentTarget as HTMLElement;
    if (target.hasPointerCapture(event.pointerId)) target.releasePointerCapture(event.pointerId);
    imageTransformPointerId = null;
    imageTransformMode = null;
    event.stopPropagation();
  }

  function cancelImageDraft() {
    if (!imageDraft || imageDraftSaving) return false;
    releaseImageDraftURL(imageDraft);
    imageDraft = null;
    imageDraftError = '';
    imageTransformPointerId = null;
    imageTransformMode = null;
    return true;
  }

  function toggleImageDraftFlip() {
    if (!imageDraft || imageDraftSaving) return;
    imageDraft.flipX = !imageDraft.flipX;
  }

  async function confirmImageDraft() {
    if (!imageDraft || imageDraftSaving) return;
    const assetId = imageDraft.assetId;
    if (localMode && assetId) {
      const draft = imageDraft;
      localOperations = [
        ...localOperations,
        {
          type: 'image',
          image: {
            assetId,
            url: draft.url,
            x: draft.x,
            y: draft.y,
            width: draft.width,
            height: draft.height,
            rotation: draft.rotation,
            flipX: draft.flipX,
            author: username
          }
        }
      ];
      imageDraft = null;
      imageDraftError = '';
      localStrokesError = '';
      redraw();
      return;
    }
    imageDraftSaving = true;
    imageDraftError = '';
    const draft = imageDraft;
    try {
      if (draft.file) {
        await createBoardImage(mediaId, draft.file, draft, username);
      } else if (draft.assetId) {
        await createBoardImageFromAsset(mediaId, draft.assetId, draft, username);
      } else {
        throw new Error('Источник изображения не найден');
      }
      releaseImageDraftURL(draft);
      imageDraft = null;
    } catch (error) {
      imageDraftError = error instanceof Error ? error.message : 'Не удалось добавить изображение';
    } finally {
      imageDraftSaving = false;
    }
  }

  function releaseImageDraftURL(draft: ImageDraft | null) {
    if (draft?.revokeURL) URL.revokeObjectURL(draft.url);
  }

  function handleKeyDown(event: KeyboardEvent) {
    if (event.key === 'Escape' && showHelp) {
      showHelp = false;
      event.preventDefault();
      event.stopPropagation();
      return;
    }

    if (event.key === 'Escape' && showAssetLibrary) {
      showAssetLibrary = false;
      event.preventDefault();
      event.stopPropagation();
      return;
    }

    if (event.key === 'Escape' && cancelImageDraft()) {
      event.preventDefault();
      event.stopPropagation();
      return;
    }

    if (event.key === 'Escape' && historyMode) {
      exitHistoryMode();
      event.stopPropagation();
      return;
    }

    if (event.key === 'Escape' && clearActiveStroke()) {
      event.preventDefault();
      event.stopPropagation();
    }
  }

  function closeBoard(event: MouseEvent) {
    event.stopPropagation();
    onClose?.();
  }
</script>

<div
  class="drawing-board"
  class:drawing-board-expanded={expanded}
  class:drawing-board-preview-fill={previewFill}
>
  {#if expanded}
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions - the focused drawing surface needs keyboard shortcuts while the canvas handles pointer input. -->
    <div
      class="drawing-canvas-wrap"
      role="application"
      aria-label={t.board.drawingBoardNamed(boardName)}
      style="--drawing-canvas-aspect: {canvasWidth / canvasHeight}; cursor: {canvasCursor};"
      onkeydown={handleKeyDown}
      onwheel={handleBoardWheel}
      ondragenter={handleBoardDragEnter}
      ondragover={handleBoardDragOver}
      ondrop={handleBoardDrop}
      tabindex="-1"
    >
      <div
        class="drawing-canvas-stage"
        style="
          --drawing-canvas-aspect: {canvasWidth / canvasHeight};
          transform: translate3d({panX}px, {panY}px, 0) scale({zoom});
        "
      >
        <canvas
          bind:this={canvasEl}
          width={canvasWidth}
          height={canvasHeight}
          class="drawing-canvas"
          onpointerdown={handlePointerDown}
          onpointermove={handlePointerMove}
          onpointerup={handlePointerUp}
          onpointercancel={handlePointerCancel}
          onpointerenter={handlePointerEnter}
          onpointerleave={handlePointerLeave}
          style="touch-action: none;"
        ></canvas>
        {#if imageDraft}
          <div
            class="drawing-image-draft"
            class:drawing-image-draft-saving={imageDraftSaving}
            style="
              left: {(imageDraft.x / canvasWidth) * 100}%;
              top: {(imageDraft.y / canvasHeight) * 100}%;
              width: {(imageDraft.width / canvasWidth) * 100}%;
              height: {(imageDraft.height / canvasHeight) * 100}%;
              transform: rotate({imageDraft.rotation}deg);
            "
            role="group"
            aria-label="Размещаемое изображение"
            onpointerdown={(event) => beginImageTransform(event, 'move')}
            onpointermove={updateImageTransform}
            onpointerup={finishImageTransform}
            onpointercancel={finishImageTransform}
          >
            <img
              src={imageDraft.url}
              alt=""
              draggable="false"
              style:transform={imageDraft.flipX ? 'scaleX(-1)' : 'none'}
            />
            {#each ['nw', 'ne', 'se', 'sw'] as corner}
              <button
                type="button"
                class="drawing-image-resize-handle drawing-image-resize-{corner}"
                aria-label="Изменить размер изображения"
                onpointerdown={(event) => beginImageTransform(event, 'resize')}
                onpointermove={updateImageTransform}
                onpointerup={finishImageTransform}
                onpointercancel={finishImageTransform}
              ></button>
            {/each}
            <button
              type="button"
              class="drawing-image-rotate-handle"
              aria-label="Повернуть изображение"
              title="Повернуть"
              onpointerdown={(event) => beginImageTransform(event, 'rotate')}
              onpointermove={updateImageTransform}
              onpointerup={finishImageTransform}
              onpointercancel={finishImageTransform}
            >
              <RotateCw size={15} />
            </button>
          </div>
        {/if}
        <div class="drawing-canvas-boundary" aria-hidden="true"></div>
      </div>
      {#if imageDraft}
        <div class="drawing-image-actions">
          <button type="button" disabled={imageDraftSaving} onclick={confirmImageDraft}>
            <Check size={16} />
            {imageDraftSaving ? 'Сохранение…' : 'Зафиксировать'}
          </button>
          <button
            type="button"
            class:drawing-image-action-active={imageDraft.flipX}
            disabled={imageDraftSaving}
            aria-label="Отразить изображение по горизонтали"
            aria-pressed={imageDraft.flipX}
            title="Отразить по горизонтали"
            onclick={toggleImageDraftFlip}
          >
            <FlipHorizontal2 size={16} />
            Зеркально
          </button>
          <button type="button" disabled={imageDraftSaving} onclick={cancelImageDraft}>
            <X size={16} />
            Отмена
          </button>
          {#if imageDraftError}<span>{imageDraftError}</span>{/if}
        </div>
      {/if}
      {#if localMode && !imageDraft}
        <div class="drawing-local-actions">
          <button
            type="button"
            disabled={localStrokesSaving || assetUploadSaving || Boolean(imageDraft) || localOperations.length === 0}
            onclick={publishLocalStrokes}
          >
            {#if localStrokesSaving}<LoaderCircle class="drawing-local-spinner" size={16} />{:else}<Check size={16} />{/if}
            {localStrokesSaving ? t.board.publishingLocalStrokes : t.board.publishLocalStrokes}
          </button>
          <span class="drawing-local-count">{t.board.localStrokeCount(localOperations.length)}</span>
          {#if assetUploadSaving}
            <span class="drawing-local-uploading">
              <LoaderCircle class="drawing-local-spinner" size={15} />
              {t.board.savingAsset}
            </span>
          {/if}
          <button
            type="button"
            disabled={localStrokesSaving || assetUploadSaving || Boolean(imageDraft) || localOperations.length === 0}
            title={`${t.board.undoLocalStroke} (Ctrl/Cmd+Z)`}
            onclick={undoLocalStroke}
          >
            <Undo2 size={16} />
            {t.board.undoLocalStroke}
          </button>
          <button type="button" disabled={localStrokesSaving || assetUploadSaving} onclick={cancelLocalMode}>
            <X size={16} />
            {t.board.cancelLocalMode}
          </button>
          {#if localStrokesError}<span class="drawing-local-error">{localStrokesError}</span>{/if}
        </div>
      {/if}
      {#if showAssetLibrary}
        <section
          class="drawing-asset-library"
          aria-label="Библиотека ассетов"
          onwheel={handleAssetLibraryWheel}
        >
          <header>
            <strong>Ассеты</strong>
            <span>{boardAssets.length}</span>
            <button type="button" aria-label="Закрыть библиотеку ассетов" onclick={() => (showAssetLibrary = false)}>
              <X size={15} />
            </button>
          </header>
          {#if boardAssetsLoading}
            <div class="drawing-asset-library-state">
              <LoaderCircle class="drawing-asset-spinner" size={22} />
              Загрузка…
            </div>
          {:else if boardAssetsError}
            <div class="drawing-asset-library-state drawing-asset-library-error">
              {boardAssetsError}
              <button type="button" onclick={loadBoardAssets}>Повторить</button>
            </div>
          {:else if boardAssets.length === 0}
            <div class="drawing-asset-library-state">
              Перетащите изображение на доску, чтобы добавить первый ассет.
            </div>
          {:else}
            <div class="drawing-asset-grid">
              {#each boardAssets as asset (asset.id)}
                <button
                  type="button"
                  title="Использований: {asset.usageCount}"
                  aria-label="Добавить ассет на доску. Использований: {asset.usageCount}"
                  onclick={() => selectBoardAsset(asset)}
                >
                  <img src={asset.url} alt="" loading="lazy" />
                  <span>{asset.usageCount}</span>
                </button>
              {/each}
            </div>
          {/if}
        </section>
      {/if}
      <div class="drawing-cancel-hint-layer" aria-hidden="true">
        <div
          class="drawing-cancel-hint"
          class:drawing-cancel-hint-visible={!historyMode && cancelHintVisible}
        >
          отпустите для отмены
        </div>
      </div>
      {#if !historyMode}
        <div
          class="drawing-brush-cursor"
          class:drawing-brush-cursor-visible={drawingToolSelected && brushCursorVisible && !spacePressed && panPointerId === null}
          style="
            width: {brushCursorSize}px;
            height: {brushCursorSize}px;
            transform: translate3d({brushCursorX}px, {brushCursorY}px, 0) translate(-50%, -50%);
            border-color: {currentColor};
            background: {colorWithOpacity(currentColor, currentOpacity)};
          "
        ></div>
      {/if}
      {#if panCursorVisible && (currentTool === 'pan' || spacePressed || panPointerId !== null)}
        <div
          class="drawing-pan-cursor"
          style="transform: translate3d({panCursorX}px, {panCursorY}px, 0) translate(-50%, -50%);"
          aria-hidden="true"
        >
          {#if panPointerId !== null}
            <HandGrab size={24} strokeWidth={2.2} />
          {:else}
            <Hand size={24} strokeWidth={2.2} />
          {/if}
        </div>
      {/if}

      {#if onClose}
        <button
          class="drawing-close-btn"
          type="button"
          aria-label={t.board.close}
          title={t.board.close}
          onclick={closeBoard}
        >
          <X size={18} />
        </button>
      {/if}

      <button
        class="drawing-help-btn"
        class:drawing-help-btn-with-close={Boolean(onClose)}
        class:drawing-help-btn-active={showHelp}
        type="button"
        aria-label={t.board.openHelp}
        title={t.board.openHelp}
        aria-expanded={showHelp}
        aria-controls="drawing-board-help"
        onclick={toggleHelp}
      >
        <CircleHelp size={18} />
      </button>

      {#if showHelp}
        <div
          class="drawing-help-backdrop"
          role="presentation"
          onwheel={handleHelpWheel}
          onclick={(event) => {
            if (event.currentTarget === event.target) showHelp = false;
          }}
        >
          <div
            id="drawing-board-help"
            class="drawing-help-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="drawing-board-help-title"
          >
            <header>
              <div>
                <h2 id="drawing-board-help-title">{t.board.helpTitle}</h2>
                <p>{t.board.helpSubtitle}</p>
              </div>
              <button type="button" aria-label={t.board.closeHelp} title={t.board.closeHelp} onclick={() => (showHelp = false)}>
                <X size={18} />
              </button>
            </header>

            <div class="drawing-help-content">
              <section>
                <h3>{t.board.helpDrawingTitle}</h3>
                <dl>
                  <div>
                    <dt>
                      <span class="drawing-help-tool-icon" aria-label={t.board.pan} title={t.board.pan}>
                        <Hand size={16} />
                      </span>
                    </dt>
                    <dd>{t.board.helpPanText}</dd>
                  </div>
                  <div>
                    <dt>
                      <span class="drawing-help-tool-icon" aria-label={t.board.freeform} title={t.board.freeform}>
                        <Pencil size={16} />
                      </span>
                    </dt>
                    <dd>{t.board.helpFreeformText}</dd>
                  </div>
                  <div>
                    <dt>
                      <span class="drawing-help-tool-icon" aria-label={t.board.line} title={t.board.line}>
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
                      </span>
                    </dt>
                    <dd>{t.board.helpLineText}</dd>
                  </div>
                  <div><dt>{t.board.helpCancelStrokeLabel}</dt><dd>{t.board.helpCancelStrokeText}</dd></div>
                  <div>
                    <dt>
                      <span class="drawing-help-tool-icon" aria-label={t.board.localMode} title={t.board.localMode}>
                        <CloudOff size={16} />
                      </span>
                    </dt>
                    <dd>{t.board.helpLocalModeText}</dd>
                  </div>
                  <div><dt><kbd>Ctrl</kbd>/<kbd>Cmd</kbd> + <kbd>Z</kbd></dt><dd>{t.board.helpLocalUndo}</dd></div>
                </dl>
              </section>

              <section>
                <h3>{t.board.helpNavigationTitle}</h3>
                <dl>
                  <div><dt><kbd>Space</kbd> + drag</dt><dd>{t.board.helpTemporaryPan}</dd></div>
                  <div><dt>{t.board.helpWheelLabel}</dt><dd>{t.board.helpWheelText}</dd></div>
                  <div><dt><kbd>+</kbd> / <kbd>−</kbd></dt><dd>{t.board.helpZoomKeys}</dd></div>
                  <div><dt><kbd>0</kbd></dt><dd>{t.board.helpResetZoomKey}</dd></div>
                  <div><dt><kbd>Ctrl</kbd> + {t.board.helpWheelLabel}</dt><dd>{t.board.helpBrushWheel}</dd></div>
                </dl>
              </section>

              <section>
                <h3>{t.board.helpImagesTitle}</h3>
                <dl>
                  <div><dt>{t.board.helpDropLabel}</dt><dd>{t.board.helpDropText}</dd></div>
                  <div>
                    <dt>
                      <span class="drawing-help-tool-icon" aria-label={t.board.assetLibrary} title={t.board.assetLibrary}>
                        <Images size={16} />
                      </span>
                    </dt>
                    <dd>{t.board.helpAssetsText}</dd>
                  </div>
                  <div><dt>{t.board.helpTransformLabel}</dt><dd>{t.board.helpTransformText}</dd></div>
                  <div><dt><kbd>Esc</kbd></dt><dd>{t.board.helpCancelImage}</dd></div>
                </dl>
              </section>

              <section>
                <h3>{t.board.helpHistoryTitle}</h3>
                <dl>
                  <div><dt><kbd>←</kbd> <kbd>↓</kbd></dt><dd>{t.board.helpHistoryBack}</dd></div>
                  <div><dt><kbd>→</kbd> <kbd>↑</kbd></dt><dd>{t.board.helpHistoryForward}</dd></div>
                  <div><dt><kbd>Home</kbd> / <kbd>End</kbd></dt><dd>{t.board.helpHistoryEdges}</dd></div>
                  <div><dt><kbd>Esc</kbd></dt><dd>{t.board.helpExitHistory}</dd></div>
                </dl>
              </section>
            </div>
          </div>
        </div>
      {/if}

      <div class="drawing-zoom-controls" aria-label={t.board.zoomControls}>
        <button
          class="drawing-tool-btn"
          type="button"
          aria-label={t.board.zoomOut}
          title={t.board.zoomOut}
          disabled={zoom <= MIN_ZOOM}
          onclick={() => setZoom(zoom - ZOOM_STEP)}
        >
          <Minus size={16} />
        </button>
        <button
          class="drawing-zoom-value"
          type="button"
          aria-label={t.board.resetZoom}
          title={t.board.resetZoom}
          onclick={resetZoom}
        >
          {Math.round(zoom * 100)}%
        </button>
        <button
          class="drawing-tool-btn"
          type="button"
          aria-label={t.board.zoomIn}
          title={t.board.zoomIn}
          disabled={zoom >= MAX_ZOOM}
          onclick={() => setZoom(zoom + ZOOM_STEP)}
        >
          <Plus size={16} />
        </button>
      </div>

      {#if historyMode}
        <div class="drawing-history-toolbar">
          <input
            bind:this={historyRangeEl}
            class="drawing-history-range"
            type="range"
            min="0"
            max={operations.length}
            step="1"
            value={historyStrokeCount}
            aria-label={t.board.visibleHistoryStrokes}
            title={t.board.visibleHistoryStrokes}
            onpointerdown={handleHistoryRangePointerDown}
            oninput={handleHistoryRangeInput}
            onkeydown={handleHistoryRangeKeydown}
          />
          <div
            class="drawing-history-count"
            aria-label={t.board.showingHistoryStrokes(historyStrokeCount, operations.length)}
          >
            {historyStrokeCount}/{operations.length}
          </div>
          <div
            class="drawing-history-author"
            aria-label={t.board.lastVisibleStrokeAuthor(historyLastAuthor())}
            title={t.board.lastVisibleStrokeAuthor(historyLastAuthor())}
          >
            <span>Автор</span>
            <strong>{historyLastAuthor()}</strong>
          </div>
          <button
            class="drawing-tool-btn drawing-history-exit"
            type="button"
            aria-label={t.board.exitHistory}
            title={t.board.exitHistory}
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
            class:drawing-tool-btn-active={currentTool === 'pan'}
            type="button"
            title={t.board.pan}
            aria-label={t.board.pan}
            onclick={() => selectTool('pan')}
          >
            <Hand size={16} />
          </button>
          <button
            class="drawing-tool-btn"
            class:drawing-tool-btn-active={currentTool === 'freeform'}
            type="button"
            title={t.board.freeform}
            aria-label={t.board.freeform}
            onclick={() => selectTool('freeform')}
          >
            <Pencil size={16} />
          </button>
          <button
            class="drawing-tool-btn"
            class:drawing-tool-btn-active={currentTool === 'line'}
            type="button"
            title={t.board.line}
            aria-label={t.board.line}
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
          {#if debugToolsEnabled}
            <button
              class="drawing-tool-btn"
              class:drawing-tool-btn-active={showDebugSegments}
              type="button"
              title={t.board.showPointDensity}
              aria-label={t.board.showPointDensity}
              aria-pressed={showDebugSegments}
              onclick={() => (showDebugSegments = !showDebugSegments)}
            >
              <Activity size={16} />
            </button>
            {#if lastRawPointCount !== null && lastSimplifiedPointCount !== null}
              <div
                class="drawing-point-stats"
                title={t.board.strokePointStatsTitle}
                aria-label={t.board.strokePointStats(lastRawPointCount, lastSimplifiedPointCount)}
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
              aria-label={t.board.simplificationTolerance}
              title={t.board.simplificationTolerance}
              oninput={handleSimplifyEpsilonInput}
            />
          {/if}
        </div>

        <div class="drawing-toolbar-divider"></div>

        <div class="drawing-toolbar-group drawing-sizes">
          {#each BRUSH_SIZES as size}
            <button
              class="drawing-size-btn"
              class:drawing-size-btn-active={currentSize === size}
              title={t.board.size(size)}
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
            aria-label={t.board.customBrushSize}
            title={t.board.customBrushSize}
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

        <label class="drawing-opacity-control" title={t.board.opacity}>
          <span>{Math.round(currentOpacity * 100)}%</span>
          <input
            type="range"
            min={MIN_BRUSH_OPACITY * 100}
            max="100"
            step="10"
            value={currentOpacity * 100}
            aria-label={t.board.opacity}
            onpointerdown={activateFreeformForBrushSettings}
            oninput={handleOpacityInput}
          />
        </label>

        <div class="drawing-toolbar-divider"></div>

        <div class="drawing-toolbar-group drawing-colors">
          <button
            class="drawing-tool-btn"
            class:drawing-tool-btn-active={showColorPicker}
            title={t.board.color}
            onclick={toggleColorPicker}
          >
            <Palette size={16} />
            <span
              class="drawing-color-indicator"
              style="background: {currentColor};"
            ></span>
          </button>
          {#if showColorPicker}
            <div class="drawing-color-grid">
              {#each FIXED_COLORS as color}
                <button
                  class="drawing-color-swatch"
                  class:drawing-color-swatch-active={currentColor === color}
                  style="background: {color};"
                  title={color}
                  onclick={() => selectColor(color)}
                ></button>
              {/each}
              <div class="drawing-color-divider" aria-hidden="true"></div>
              {#each customColors as color}
                <button
                  class="drawing-color-swatch"
                  class:drawing-color-swatch-active={currentColor === color}
                  style="background: {color};"
                  title={color}
                  onclick={() => selectColor(color)}
                ></button>
              {/each}
              <label class="drawing-color-custom-wrap" title={t.board.customColor} aria-label={t.board.customColor}>
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
                  onchange={(e) => selectCustomColor((e.currentTarget as HTMLInputElement).value)}
                />
              </label>
            </div>
          {/if}
        </div>
        <div class="drawing-toolbar-divider"></div>
        <div class="drawing-toolbar-group">
          <button
            class="drawing-tool-btn"
            class:drawing-tool-btn-active={localMode}
            type="button"
            title={localMode ? t.board.disableLocalMode : t.board.enableLocalMode}
            aria-label={localMode ? t.board.disableLocalMode : t.board.enableLocalMode}
            aria-pressed={localMode}
            disabled={Boolean(imageDraft) || localStrokesSaving || assetUploadSaving}
            onclick={toggleLocalMode}
          >
            <CloudOff size={16} />
          </button>
          <button
            class="drawing-tool-btn"
            class:drawing-tool-btn-active={showAssetLibrary}
            type="button"
            title={t.board.assetLibrary}
            aria-label={t.board.openAssetLibrary}
            aria-pressed={showAssetLibrary}
            onclick={toggleAssetLibrary}
          >
            <Images size={16} />
          </button>
          <button
            class="drawing-tool-btn"
            type="button"
            title={t.board.history}
            aria-label={t.board.openHistory}
            disabled={localMode}
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
    background: transparent;
  }

  .drawing-board:not(.drawing-board-expanded) {
    background: transparent;
  }

  .drawing-board-preview-fill {
    background: transparent;
  }

  .drawing-canvas-wrap {
    position: relative;
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
  }

  .drawing-canvas-stage {
    position: absolute;
    width: min(100%, calc(100vh * var(--drawing-canvas-aspect)));
    max-height: 100%;
    aspect-ratio: var(--drawing-canvas-aspect);
    transform-origin: center;
    will-change: transform;
  }

  .drawing-canvas {
    width: 100%;
    height: 100%;
    display: block;
    cursor: inherit;
  }

  .drawing-canvas-boundary {
    position: absolute;
    inset: 0;
    outline: 1px solid rgba(255, 255, 255, 0.12);
    outline-offset: 0;
    box-shadow:
      0 0 0 1px rgba(0, 0, 0, 0.34),
      0 0.9rem 2.4rem rgba(0, 0, 0, 0.22);
    pointer-events: none;
  }

  .drawing-image-draft {
    position: absolute;
    z-index: 6;
    box-sizing: border-box;
    border: 2px solid #60a5fa;
    box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.55);
    cursor: move;
    touch-action: none;
    transform-origin: center;
    user-select: none;
  }

  .drawing-image-draft-saving {
    opacity: 0.7;
    pointer-events: none;
  }

  .drawing-image-draft img {
    display: block;
    width: 100%;
    height: 100%;
    object-fit: fill;
    pointer-events: none;
    transform-origin: center;
  }

  .drawing-image-resize-handle,
  .drawing-image-rotate-handle {
    position: absolute;
    z-index: 2;
    display: grid;
    width: 16px;
    height: 16px;
    padding: 0;
    place-items: center;
    border: 2px solid #fff;
    border-radius: 50%;
    background: #3b82f6;
    color: #fff;
    box-shadow: 0 1px 4px rgba(0, 0, 0, 0.65);
    touch-action: none;
  }

  .drawing-image-resize-nw { top: 0; left: 0; cursor: nwse-resize; transform: translate(-50%, -50%); }
  .drawing-image-resize-ne { top: 0; right: 0; cursor: nesw-resize; transform: translate(50%, -50%); }
  .drawing-image-resize-se { right: 0; bottom: 0; cursor: nwse-resize; transform: translate(50%, 50%); }
  .drawing-image-resize-sw { bottom: 0; left: 0; cursor: nesw-resize; transform: translate(-50%, 50%); }

  .drawing-image-rotate-handle {
    top: -34px;
    left: 50%;
    width: 24px;
    height: 24px;
    cursor: grab;
    transform: translateX(-50%);
  }

  .drawing-image-rotate-handle::after {
    position: absolute;
    top: 100%;
    left: 50%;
    width: 1px;
    height: 12px;
    background: #60a5fa;
    content: '';
  }

  .drawing-image-actions {
    position: absolute;
    top: 1rem;
    left: 50%;
    z-index: 14;
    display: flex;
    align-items: center;
    gap: 0.4rem;
    transform: translateX(-50%);
  }

  .drawing-image-actions button {
    display: flex;
    height: 2.25rem;
    align-items: center;
    gap: 0.35rem;
    padding: 0 0.75rem;
    border: 1px solid rgba(255, 255, 255, 0.18);
    border-radius: 0.65rem;
    background: rgba(15, 15, 23, 0.88);
    color: #fff;
    font-size: 0.75rem;
    font-weight: 650;
  }

  .drawing-image-actions button:first-child {
    border-color: rgba(96, 165, 250, 0.55);
    background: rgba(37, 99, 235, 0.9);
  }

  .drawing-image-actions .drawing-image-action-active {
    border-color: rgba(96, 165, 250, 0.65);
    background: rgba(37, 99, 235, 0.72);
  }

  .drawing-image-actions span {
    max-width: min(26rem, 45vw);
    color: #fca5a5;
    font-size: 0.72rem;
  }

  .drawing-local-actions {
    position: absolute;
    top: 1rem;
    left: 50%;
    z-index: 14;
    display: flex;
    align-items: center;
    gap: 0.4rem;
    max-width: calc(100vw - 7rem);
    transform: translateX(-50%);
  }

  .drawing-local-actions button {
    display: flex;
    height: 2.25rem;
    flex: 0 0 auto;
    align-items: center;
    gap: 0.35rem;
    padding: 0 0.75rem;
    border: 1px solid rgba(255, 255, 255, 0.18);
    border-radius: 0.65rem;
    background: rgba(15, 15, 23, 0.88);
    color: #fff;
    font-size: 0.75rem;
    font-weight: 650;
  }

  .drawing-local-actions button:first-child {
    border-color: rgba(96, 165, 250, 0.55);
    background: rgba(37, 99, 235, 0.9);
  }

  .drawing-local-actions button:disabled {
    cursor: default;
    opacity: 0.42;
  }

  .drawing-local-count {
    flex: 0 0 auto;
    padding: 0.4rem 0.6rem;
    border: 1px solid rgba(96, 165, 250, 0.25);
    border-radius: 999px;
    background: rgba(15, 15, 23, 0.78);
    color: rgba(191, 219, 254, 0.92);
    font-size: 0.7rem;
    font-variant-numeric: tabular-nums;
  }

  .drawing-local-error {
    max-width: min(26rem, 38vw);
    color: #fca5a5;
    font-size: 0.72rem;
  }

  .drawing-local-uploading {
    display: flex;
    flex: 0 0 auto;
    align-items: center;
    gap: 0.35rem;
    color: rgba(255, 255, 255, 0.72);
    font-size: 0.72rem;
  }

  .drawing-local-spinner {
    animation: drawing-asset-spin 900ms linear infinite;
  }

  @media (max-width: 760px) {
    .drawing-local-actions {
      right: 4.5rem;
      left: 0.75rem;
      max-width: none;
      flex-wrap: wrap;
      transform: none;
    }

    .drawing-local-actions button {
      padding-inline: 0.55rem;
    }
  }

  .drawing-asset-library {
    position: absolute;
    top: 50%;
    right: 5rem;
    z-index: 13;
    display: flex;
    width: min(22rem, calc(100vw - 7rem));
    max-height: min(32rem, calc(100vh - 3rem));
    flex-direction: column;
    overflow: hidden;
    border: 1px solid rgba(255, 255, 255, 0.14);
    border-radius: 0.9rem;
    background: rgba(15, 15, 23, 0.94);
    box-shadow: 0 18px 48px rgba(0, 0, 0, 0.48);
    backdrop-filter: blur(18px) saturate(145%);
    -webkit-backdrop-filter: blur(18px) saturate(145%);
    transform: translateY(-50%);
  }

  .drawing-asset-library header {
    display: flex;
    height: 2.75rem;
    flex: 0 0 auto;
    align-items: center;
    gap: 0.45rem;
    padding: 0 0.55rem 0 0.8rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.09);
  }

  .drawing-asset-library header strong {
    color: rgba(255, 255, 255, 0.92);
    font-size: 0.82rem;
  }

  .drawing-asset-library header span {
    color: rgba(255, 255, 255, 0.45);
    font-size: 0.7rem;
  }

  .drawing-asset-library header button {
    display: grid;
    width: 1.9rem;
    height: 1.9rem;
    margin-left: auto;
    place-items: center;
    border: 0;
    border-radius: 0.45rem;
    background: transparent;
    color: rgba(255, 255, 255, 0.65);
  }

  .drawing-asset-library header button:hover {
    background: rgba(255, 255, 255, 0.08);
    color: #fff;
  }

  .drawing-asset-grid {
    display: grid;
    flex: 1 1 auto;
    min-height: 0;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    grid-auto-rows: max-content;
    align-content: start;
    gap: 0.55rem;
    padding: 0.65rem;
    overflow-x: hidden;
    overflow-y: auto;
    overscroll-behavior: contain;
    touch-action: pan-y;
  }

  .drawing-asset-grid button {
    position: relative;
    display: block;
    width: 100%;
    min-width: 0;
    height: auto;
    padding: 0;
    overflow: hidden;
    border: 1px solid rgba(255, 255, 255, 0.11);
    border-radius: 0.55rem;
    aspect-ratio: 1;
    background-color: #1b1b24;
    background-image:
      linear-gradient(45deg, rgba(255, 255, 255, 0.06) 25%, transparent 25%),
      linear-gradient(-45deg, rgba(255, 255, 255, 0.06) 25%, transparent 25%),
      linear-gradient(45deg, transparent 75%, rgba(255, 255, 255, 0.06) 75%),
      linear-gradient(-45deg, transparent 75%, rgba(255, 255, 255, 0.06) 75%);
    background-position: 0 0, 0 6px, 6px -6px, -6px 0;
    background-size: 12px 12px;
  }

  .drawing-asset-grid button:hover,
  .drawing-asset-grid button:focus-visible {
    border-color: rgba(96, 165, 250, 0.75);
    box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
    outline: none;
  }

  .drawing-asset-grid img {
    position: absolute;
    inset: 0;
    display: block;
    width: 100%;
    height: 100%;
    object-fit: contain;
  }

  .drawing-asset-grid span {
    position: absolute;
    right: 0.3rem;
    bottom: 0.3rem;
    min-width: 1.15rem;
    padding: 0.15rem 0.3rem;
    border-radius: 999px;
    background: rgba(0, 0, 0, 0.72);
    color: rgba(255, 255, 255, 0.9);
    font-size: 0.62rem;
    font-variant-numeric: tabular-nums;
    line-height: 1;
    text-align: center;
  }

  .drawing-asset-library-state {
    display: flex;
    min-height: 9rem;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    padding: 1.25rem;
    color: rgba(255, 255, 255, 0.58);
    font-size: 0.76rem;
    line-height: 1.45;
    text-align: center;
  }

  .drawing-asset-library-error {
    flex-direction: column;
    color: #fca5a5;
  }

  .drawing-asset-library-error button {
    padding: 0.35rem 0.6rem;
    border: 1px solid rgba(255, 255, 255, 0.15);
    border-radius: 0.45rem;
    background: rgba(255, 255, 255, 0.07);
    color: #fff;
  }

  .drawing-asset-spinner {
    animation: drawing-asset-spin 900ms linear infinite;
  }

  @keyframes drawing-asset-spin {
    to { transform: rotate(360deg); }
  }

  @media (max-width: 700px) {
    .drawing-asset-library {
      top: 4.25rem;
      right: 0.75rem;
      left: 0.75rem;
      width: auto;
      max-height: calc(100vh - 9rem);
      transform: none;
    }

    .drawing-asset-grid {
      grid-template-columns: repeat(4, minmax(0, 1fr));
    }
  }

  .drawing-cancel-hint-layer {
    position: absolute;
    inset: 0;
    pointer-events: none;
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
    transition: opacity 90ms ease;
    will-change: transform, width, height;
  }

  .drawing-brush-cursor-visible {
    opacity: 1;
  }

  .drawing-pan-cursor {
    position: absolute;
    top: 0;
    left: 0;
    z-index: 20;
    display: grid;
    width: 1.5rem;
    height: 1.5rem;
    place-items: center;
    color: #fff;
    filter:
      drop-shadow(0 0 1px rgba(0, 0, 0, 0.95))
      drop-shadow(0 1px 2px rgba(0, 0, 0, 0.72));
    pointer-events: none;
    will-change: transform;
  }

  .drawing-cancel-hint {
    position: absolute;
    top: 14px;
    left: 50%;
    z-index: 9;
    max-width: min(190px, calc(100vw - 32px));
    padding: 5px 9px;
    border-radius: 999px;
    background: rgba(18, 18, 22, 0.88);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.34);
    color: #fff;
    font-size: 12px;
    font-weight: 700;
    line-height: 1;
    opacity: 0;
    pointer-events: none;
    transform: translateX(-50%);
    white-space: nowrap;
    transition: opacity 120ms ease;
  }

  .drawing-cancel-hint-visible {
    opacity: 1;
  }

  .drawing-preview {
    position: relative;
    display: flex;
    width: 100%;
    height: 100%;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    box-shadow: none;
  }

  .drawing-preview::after {
    content: none;
  }

  .drawing-canvas-preview {
    display: block;
    width: auto;
    height: auto;
    max-width: 100%;
    max-height: 100%;
    pointer-events: none;
  }

  .drawing-board-preview-fill .drawing-canvas-preview {
    width: 100%;
    height: 100%;
    max-width: none;
    max-height: none;
    object-fit: cover;
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

  .drawing-help-btn {
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
    color: rgba(255, 255, 255, 0.78);
    backdrop-filter: blur(14px) saturate(150%);
    -webkit-backdrop-filter: blur(14px) saturate(150%);
    transition:
      background 140ms ease,
      border-color 140ms ease,
      color 140ms ease;
  }

  .drawing-help-btn-with-close {
    right: 4rem;
  }

  .drawing-help-btn:hover,
  .drawing-help-btn-active {
    border-color: rgba(255, 255, 255, 0.28);
    background: rgba(15, 15, 23, 0.9);
    color: #fff;
  }

  .drawing-help-backdrop {
    position: absolute;
    inset: 0;
    z-index: 30;
    display: grid;
    padding: 1rem;
    place-items: center;
    background: rgba(0, 0, 0, 0.52);
    backdrop-filter: blur(5px);
    -webkit-backdrop-filter: blur(5px);
  }

  .drawing-help-dialog {
    display: flex;
    width: min(48rem, 100%);
    max-height: min(42rem, calc(100vh - 2rem));
    flex-direction: column;
    overflow: hidden;
    border: 1px solid rgba(255, 255, 255, 0.16);
    border-radius: 1.25rem;
    background: rgba(15, 15, 23, 0.96);
    box-shadow: 0 24px 80px rgba(0, 0, 0, 0.56);
    color: rgba(255, 255, 255, 0.9);
  }

  .drawing-help-dialog > header {
    display: flex;
    flex: 0 0 auto;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
    padding: 1.1rem 1.15rem 0.9rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  }

  .drawing-help-dialog h2,
  .drawing-help-dialog h3,
  .drawing-help-dialog p {
    margin: 0;
  }

  .drawing-help-dialog h2 {
    font-size: 1rem;
    line-height: 1.3;
  }

  .drawing-help-dialog header p {
    margin-top: 0.25rem;
    color: rgba(255, 255, 255, 0.52);
    font-size: 0.76rem;
    line-height: 1.4;
  }

  .drawing-help-dialog header button {
    display: grid;
    width: 2rem;
    height: 2rem;
    flex: 0 0 auto;
    padding: 0;
    place-items: center;
    border: 0;
    border-radius: 0.55rem;
    background: transparent;
    color: rgba(255, 255, 255, 0.65);
  }

  .drawing-help-dialog header button:hover {
    background: rgba(255, 255, 255, 0.08);
    color: #fff;
  }

  .drawing-help-content {
    display: grid;
    min-height: 0;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 1.25rem 1.5rem;
    padding: 1rem 1.15rem 1.2rem;
    overflow-y: auto;
    overscroll-behavior: contain;
  }

  .drawing-help-content h3 {
    margin-bottom: 0.5rem;
    color: rgba(255, 255, 255, 0.92);
    font-size: 0.72rem;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .drawing-help-content dl {
    display: grid;
    gap: 0;
    margin: 0;
  }

  .drawing-help-content dl > div {
    display: grid;
    grid-template-columns: minmax(6rem, 0.8fr) minmax(0, 1.4fr);
    gap: 0.75rem;
    padding: 0.55rem 0;
    border-top: 1px solid rgba(255, 255, 255, 0.07);
  }

  .drawing-help-content dt,
  .drawing-help-content dd {
    margin: 0;
    font-size: 0.75rem;
    line-height: 1.45;
  }

  .drawing-help-content dt {
    color: rgba(255, 255, 255, 0.86);
    font-weight: 650;
  }

  .drawing-help-tool-icon {
    display: inline-grid;
    width: 2rem;
    height: 2rem;
    place-items: center;
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 0.5rem;
    background: rgba(255, 255, 255, 0.06);
    color: rgba(255, 255, 255, 0.88);
  }

  .drawing-help-content dd {
    color: rgba(255, 255, 255, 0.56);
  }

  .drawing-help-content kbd {
    display: inline-flex;
    min-width: 1.5rem;
    min-height: 1.35rem;
    align-items: center;
    justify-content: center;
    padding: 0.1rem 0.35rem;
    border: 1px solid rgba(255, 255, 255, 0.18);
    border-radius: 0.35rem;
    background: rgba(255, 255, 255, 0.08);
    box-shadow: 0 1px 0 rgba(255, 255, 255, 0.1) inset;
    color: rgba(255, 255, 255, 0.9);
    font: inherit;
    font-size: 0.68rem;
    line-height: 1;
  }

  @media (max-width: 700px) {
    .drawing-help-content {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 420px) {
    .drawing-help-backdrop {
      padding: 0.5rem;
    }

    .drawing-help-dialog {
      max-height: calc(100vh - 1rem);
      border-radius: 1rem;
    }

    .drawing-help-content dl > div {
      grid-template-columns: 1fr;
      gap: 0.2rem;
    }
  }

  .drawing-zoom-controls {
    position: absolute;
    top: 1rem;
    left: 1rem;
    z-index: 12;
    display: flex;
    align-items: center;
    gap: 0.25rem;
    padding: 0.25rem;
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 0.8rem;
    background: rgba(15, 15, 23, 0.72);
    backdrop-filter: blur(14px) saturate(150%);
    -webkit-backdrop-filter: blur(14px) saturate(150%);
  }

  .drawing-zoom-value {
    min-width: 3.5rem;
    height: 2rem;
    border: 0;
    border-radius: 0.5rem;
    background: transparent;
    color: rgba(255, 255, 255, 0.9);
    font-size: 0.75rem;
    font-variant-numeric: tabular-nums;
    font-weight: 700;
  }

  .drawing-zoom-value:hover {
    background: rgba(255, 255, 255, 0.08);
    color: #fff;
  }

  .drawing-zoom-controls .drawing-tool-btn:disabled {
    cursor: default;
    opacity: 0.35;
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
    right: 1rem;
    bottom: 1rem;
    left: 1rem;
    z-index: 10;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto 12rem auto;
    align-items: center;
    gap: 0.55rem;
    padding: 0.38rem 0.5rem;
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 0.65rem;
    background: rgba(15, 15, 23, 0.46);
    backdrop-filter: blur(8px) saturate(125%);
    -webkit-backdrop-filter: blur(8px) saturate(125%);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.24);
    pointer-events: auto;
  }

  .drawing-history-range {
    width: 100%;
    accent-color: #ffffff;
    cursor: pointer;
  }

  .drawing-history-count {
    min-width: 3.8rem;
    padding: 0.25rem 0.35rem;
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 0.38rem;
    background: rgba(255, 255, 255, 0.05);
    color: rgba(255, 255, 255, 0.72);
    font-size: 0.68rem;
    font-variant-numeric: tabular-nums;
    line-height: 1;
    text-align: center;
  }

  .drawing-history-author {
    display: flex;
    min-width: 0;
    width: 12rem;
    height: 1.55rem;
    align-items: center;
    gap: 0.35rem;
    padding: 0.25rem 0.45rem;
    overflow: hidden;
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 0.38rem;
    background: rgba(255, 255, 255, 0.05);
    color: rgba(255, 255, 255, 0.74);
    font-size: 0.68rem;
    line-height: 1;
    white-space: nowrap;
  }

  .drawing-history-author span {
    flex: 0 0 auto;
    color: rgba(255, 255, 255, 0.46);
  }

  .drawing-history-author strong {
    min-width: 0;
    overflow: hidden;
    color: rgba(255, 255, 255, 0.9);
    font-weight: 600;
    text-overflow: ellipsis;
  }

  .drawing-history-exit {
    border-color: rgba(255, 255, 255, 0.12);
    background: rgba(255, 255, 255, 0.07);
  }

  @media (max-width: 520px) {
    .drawing-history-toolbar {
      right: 0.65rem;
      bottom: 0.65rem;
      left: 0.65rem;
      grid-template-columns: auto minmax(0, 1fr) auto;
      gap: 0.42rem;
      padding: 0.42rem;
    }

    .drawing-history-range {
      grid-column: 1 / -1;
      grid-row: 2;
      min-height: 1.35rem;
    }

    .drawing-history-count {
      min-width: 3.35rem;
      padding-inline: 0.3rem;
    }

    .drawing-history-author {
      width: auto;
      height: 1.45rem;
      padding-inline: 0.38rem;
    }

    .drawing-history-author span {
      display: none;
    }
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

  .drawing-tool-btn:disabled {
    cursor: default;
    opacity: 0.3;
  }

  .drawing-tool-btn:disabled:hover {
    background: transparent;
    color: rgba(255, 255, 255, 0.6);
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

  .drawing-opacity-control {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    color: rgba(255, 255, 255, 0.8);
    font-size: 0.65rem;
    font-variant-numeric: tabular-nums;
    font-weight: 700;
  }

  .drawing-opacity-control span {
    min-width: 2.25rem;
    text-align: center;
  }

  .drawing-opacity-control input {
    width: 4.5rem;
    accent-color: currentColor;
  }

  .drawing-board-expanded .drawing-opacity-control {
    flex-direction: column;
    gap: 0.15rem;
  }

  .drawing-board-expanded .drawing-opacity-control input {
    width: 3.5rem;
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

  .drawing-color-divider {
    grid-column: 1 / -1;
    height: 1px;
    margin: 0.1rem 0;
    background: rgba(255, 255, 255, 0.16);
  }

  .drawing-color-swatch {
    width: 1.75rem;
    height: 1.75rem;
    border-radius: 50%;
    border: 2px solid rgba(255, 255, 255, 0.34);
    box-shadow:
      inset 0 0 0 1px rgba(0, 0, 0, 0.28),
      0 1px 3px rgba(0, 0, 0, 0.28);
    cursor: pointer;
    transition: all 150ms ease;
  }

  .drawing-color-swatch:hover {
    transform: scale(1.15);
  }

  .drawing-color-swatch-active {
    border-color: #fff;
    box-shadow:
      inset 0 0 0 1px rgba(0, 0, 0, 0.32),
      0 0 0 2px rgba(0, 0, 0, 0.34),
      0 2px 6px rgba(0, 0, 0, 0.32);
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
