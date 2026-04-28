<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from "vue";

type PickerMode = "date" | "datetime";
type Part = "year" | "month" | "day" | "hour" | "minute";

interface PickerState {
  year: number;
  month: number;
  day: number;
  hour: number;
  minute: number;
}

interface MotionState {
  offset: number;
  targetOffset: number;
  rafId: number;
  snapTimer: number;
  dragging: boolean;
}

const MIN_YEAR = 1000;
const MAX_YEAR = 9999;
const ROW_HEIGHT = 32;
const DRAG_THRESHOLD = 4;
const WHEEL_SENSITIVITY = 0.32;
const WHEEL_MIN_DELTA = 0.35;
const MAX_OFFSET = ROW_HEIGHT * 1.45;
const SNAP_DELAY_MS = 90;
const COLLAPSE_DELAY_MS = 220;
const INPUT_COMMIT_DELAY_MS = 900;
const EASE_ACTIVE = 0.34;
const EASE_SETTLE = 0.2;

const props = withDefaults(
  defineProps<{
    modelValue: string;
    mode?: PickerMode;
    emptyLabel?: string;
  }>(),
  {
    mode: "date",
    emptyLabel: "选择日期",
  },
);

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const root = ref<HTMLElement | null>(null);
const expanded = ref(false);
const focusedPart = ref<Part | "">("");
const renderTick = ref(0);
const hasValue = ref(props.modelValue.trim() !== "");
const state = reactive<PickerState>(parseValue(props.modelValue, props.mode));
const motions = reactive<Record<Part, MotionState>>(makeMotions());
const inputBuffers = reactive<Record<Part, string>>({
  year: "",
  month: "",
  day: "",
  hour: "",
  minute: "",
});
const inputTimers = reactive<Record<Part, number>>({
  year: 0,
  month: 0,
  day: 0,
  hour: 0,
  minute: 0,
});
let collapseTimer = 0;

const orderedParts = computed<Part[]>(() => {
  return props.mode === "datetime" ? ["year", "month", "day", "hour", "minute"] : ["year", "month", "day"];
});

const isEmpty = computed(() => !hasValue.value);

watch(
  () => [props.modelValue, props.mode] as const,
  ([value, mode]) => {
    hasValue.value = value.trim() !== "";
    Object.assign(state, normalizeState(parseValue(value, mode)));
    render();
  },
);

function makeMotions(): Record<Part, MotionState> {
  return {
    year: makeMotion(),
    month: makeMotion(),
    day: makeMotion(),
    hour: makeMotion(),
    minute: makeMotion(),
  };
}

function makeMotion(): MotionState {
  return {
    offset: 0,
    targetOffset: 0,
    rafId: 0,
    snapTimer: 0,
    dragging: false,
  };
}

function parseValue(value: string, mode: PickerMode): PickerState {
  const raw = String(value || "").trim();
  const now = new Date();
  const fallback = {
    year: now.getFullYear(),
    month: now.getMonth() + 1,
    day: now.getDate(),
    hour: 8,
    minute: 0,
  };

  if (!raw) {
    return fallback;
  }

  const match =
    mode === "datetime"
      ? raw.match(/^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/)
      : raw.match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (!match) {
    return fallback;
  }

  return normalizeState({
    year: Number.parseInt(match[1], 10),
    month: Number.parseInt(match[2], 10),
    day: Number.parseInt(match[3], 10),
    hour: mode === "datetime" ? Number.parseInt(match[4], 10) : fallback.hour,
    minute: mode === "datetime" ? Number.parseInt(match[5], 10) : fallback.minute,
  });
}

function normalizeState(rawState: Partial<PickerState>): PickerState {
  const now = new Date();
  const year = clamp(Number.parseInt(String(rawState.year), 10) || now.getFullYear(), MIN_YEAR, MAX_YEAR);
  const month = clamp(Number.parseInt(String(rawState.month), 10) || now.getMonth() + 1, 1, 12);
  const day = clamp(Number.parseInt(String(rawState.day), 10) || now.getDate(), 1, daysInMonth(year, month));
  const hour = clamp(Number.parseInt(String(rawState.hour), 10) || 0, 0, 23);
  const minute = clamp(Number.parseInt(String(rawState.minute), 10) || 0, 0, 59);
  return { year, month, day, hour, minute };
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

function pad2(value: number) {
  return String(value).padStart(2, "0");
}

function daysInMonth(year: number, month: number) {
  return new Date(year, month, 0, 12).getDate();
}

function buildDateFromState(value: PickerState) {
  if (props.mode === "datetime") {
    return new Date(value.year, value.month - 1, value.day, value.hour, value.minute, 0, 0);
  }
  return new Date(value.year, value.month - 1, value.day, 12, 0, 0, 0);
}

function dateToState(value: Date): PickerState {
  return {
    year: value.getFullYear(),
    month: value.getMonth() + 1,
    day: value.getDate(),
    hour: value.getHours(),
    minute: value.getMinutes(),
  };
}

function formatValue(value: PickerState) {
  const normalized = normalizeState(value);
  const date = `${String(normalized.year).padStart(4, "0")}-${pad2(normalized.month)}-${pad2(normalized.day)}`;
  if (props.mode === "datetime") {
    return `${date}T${pad2(normalized.hour)}:${pad2(normalized.minute)}`;
  }
  return date;
}

function normalizeWheelDelta(event: WheelEvent) {
  if (event.deltaMode === 1) {
    return event.deltaY * 16;
  }
  if (event.deltaMode === 2) {
    return event.deltaY * ROW_HEIGHT;
  }
  return event.deltaY;
}

function formatPart(part: Part, value: number) {
  if (part === "year") {
    return String(value);
  }
  return pad2(value);
}

function applyDelta(source: PickerState, part: Part, delta: number): PickerState {
  let nextDate = buildDateFromState(source);

  if (part === "year") {
    const targetYear = clamp(source.year + delta, MIN_YEAR, MAX_YEAR);
    nextDate = buildDateFromState({
      ...source,
      year: targetYear,
      day: clamp(source.day, 1, daysInMonth(targetYear, source.month)),
    });
  } else if (part === "month") {
    const rawIndex = source.year * 12 + (source.month - 1) + delta;
    const minIndex = MIN_YEAR * 12;
    const maxIndex = MAX_YEAR * 12 + 11;
    const index = clamp(rawIndex, minIndex, maxIndex);
    const year = Math.floor(index / 12);
    const month = (index % 12) + 1;
    nextDate = buildDateFromState({
      ...source,
      year,
      month,
      day: clamp(source.day, 1, daysInMonth(year, month)),
    });
  } else if (part === "day") {
    nextDate.setDate(nextDate.getDate() + delta);
  } else if (part === "hour") {
    nextDate.setHours(nextDate.getHours() + delta);
  } else if (part === "minute") {
    nextDate.setMinutes(nextDate.getMinutes() + delta);
  }

  return normalizeState(dateToState(nextDate));
}

function activateValue() {
  if (hasValue.value) {
    return;
  }
  hasValue.value = true;
  syncModel();
}

function syncModel() {
  emit("update:modelValue", hasValue.value ? formatValue(state) : "");
}

function render() {
  renderTick.value += 1;
}

function setExpanded(next: boolean) {
  if (collapseTimer) {
    window.clearTimeout(collapseTimer);
    collapseTimer = 0;
  }
  expanded.value = next;
  if (next) {
    void nextTick(() => {
      document.addEventListener("pointerdown", onDocumentPointerDown);
    });
  } else {
    document.removeEventListener("pointerdown", onDocumentPointerDown);
  }
}

function hasActiveMotion() {
  return orderedParts.value.some((part) => {
    const motion = motions[part];
    return (
      motion.dragging ||
      motion.rafId !== 0 ||
      motion.snapTimer !== 0 ||
      Math.abs(motion.offset) > 0.18 ||
      Math.abs(motion.targetOffset) > 0.18
    );
  });
}

function scheduleCollapse() {
  if (collapseTimer) {
    window.clearTimeout(collapseTimer);
  }
  collapseTimer = window.setTimeout(() => {
    collapseTimer = 0;
    if (orderedParts.value.some((part) => inputBuffers[part])) {
      scheduleCollapse();
      return;
    }
    if (hasActiveMotion()) {
      scheduleCollapse();
      return;
    }
    setExpanded(false);
  }, COLLAPSE_DELAY_MS);
}

function scheduleSnap(part: Part) {
  const motion = motions[part];
  if (motion.snapTimer) {
    window.clearTimeout(motion.snapTimer);
  }
  motion.snapTimer = window.setTimeout(() => {
    motion.snapTimer = 0;
    motion.targetOffset = 0;
    startMotion(part);
  }, SNAP_DELAY_MS);
}

function stepStateIfNeeded(part: Part) {
  const motion = motions[part];
  let changed = false;

  while (motion.offset <= -ROW_HEIGHT) {
    Object.assign(state, applyDelta(state, part, 1));
    motion.offset += ROW_HEIGHT;
    motion.targetOffset += ROW_HEIGHT;
    changed = true;
  }

  while (motion.offset >= ROW_HEIGHT) {
    Object.assign(state, applyDelta(state, part, -1));
    motion.offset -= ROW_HEIGHT;
    motion.targetOffset -= ROW_HEIGHT;
    changed = true;
  }

  if (changed) {
    syncModel();
  }
}

function stopMotionIfSettled(part: Part) {
  const motion = motions[part];
  if (Math.abs(motion.targetOffset - motion.offset) > 0.18 || Math.abs(motion.offset) > 0.18) {
    return false;
  }

  motion.offset = 0;
  motion.targetOffset = 0;
  motion.rafId = 0;
  render();
  scheduleCollapse();
  return true;
}

function tickMotion(part: Part) {
  const motion = motions[part];
  const ease = motion.dragging ? EASE_ACTIVE : EASE_SETTLE;
  motion.offset += (motion.targetOffset - motion.offset) * ease;
  stepStateIfNeeded(part);
  render();

  if (stopMotionIfSettled(part)) {
    return;
  }

  motion.rafId = window.requestAnimationFrame(() => tickMotion(part));
}

function startMotion(part: Part) {
  const motion = motions[part];
  if (motion.rafId) {
    return;
  }
  motion.rafId = window.requestAnimationFrame(() => tickMotion(part));
}

function clearInputTimer(part: Part) {
  if (inputTimers[part]) {
    window.clearTimeout(inputTimers[part]);
    inputTimers[part] = 0;
  }
}

function clearInputBuffer(part: Part) {
  clearInputTimer(part);
  inputBuffers[part] = "";
}

function nudge(part: Part, delta: number) {
  const motion = motions[part];
  activateValue();
  clearInputBuffer(part);
  setExpanded(true);
  motion.targetOffset = clamp(
    motion.targetOffset + (delta > 0 ? -ROW_HEIGHT : ROW_HEIGHT),
    -MAX_OFFSET,
    MAX_OFFSET,
  );
  startMotion(part);
  scheduleSnap(part);
}

function commitInputBuffer(part: Part) {
  const buffer = inputBuffers[part];
  clearInputTimer(part);
  if (!buffer) {
    return;
  }

  const parsed = Number.parseInt(buffer, 10);
  if (Number.isNaN(parsed)) {
    clearInputBuffer(part);
    render();
    return;
  }

  if (part === "year" && buffer.length < 4) {
    clearInputBuffer(part);
    render();
    scheduleCollapse();
    return;
  }

  Object.assign(state, normalizeState({ ...state, [part]: parsed }));
  activateValue();
  clearInputBuffer(part);
  syncModel();
  render();
  scheduleCollapse();
}

function queueInputCommit(part: Part) {
  clearInputTimer(part);
  inputTimers[part] = window.setTimeout(() => {
    inputTimers[part] = 0;
    commitInputBuffer(part);
  }, INPUT_COMMIT_DELAY_MS);
}

function onWheel(event: WheelEvent, part: Part) {
  event.preventDefault();
  const delta = normalizeWheelDelta(event);
  if (Math.abs(delta) < WHEEL_MIN_DELTA) {
    return;
  }

  clearInputBuffer(part);
  activateValue();
  setExpanded(true);
  const motion = motions[part];
  motion.targetOffset = clamp(motion.targetOffset - delta * WHEEL_SENSITIVITY, -MAX_OFFSET, MAX_OFFSET);
  startMotion(part);
  scheduleSnap(part);
}

function onKeydown(event: KeyboardEvent, part: Part) {
  if (event.key === "ArrowUp" || event.key === "PageUp") {
    event.preventDefault();
    nudge(part, -1);
    return;
  }
  if (event.key === "ArrowDown" || event.key === "PageDown") {
    event.preventDefault();
    nudge(part, 1);
    return;
  }
  if (/^\d$/.test(event.key)) {
    event.preventDefault();
    const maxLength = part === "year" ? 4 : 2;
    inputBuffers[part] = (inputBuffers[part] + event.key).slice(-maxLength);
    activateValue();
    render();
    if (inputBuffers[part].length >= maxLength) {
      commitInputBuffer(part);
      return;
    }
    queueInputCommit(part);
    return;
  }
  if (event.key === "Backspace") {
    event.preventDefault();
    if (!inputBuffers[part]) {
      return;
    }
    inputBuffers[part] = inputBuffers[part].slice(0, -1);
    if (!inputBuffers[part]) {
      clearInputBuffer(part);
    } else {
      queueInputCommit(part);
    }
    render();
    return;
  }
  if (event.key === "Enter") {
    event.preventDefault();
    commitInputBuffer(part);
  }
}

const dragStates = new Map<Part, { active: boolean; pointerId: number | null; lastY: number; moved: boolean }>();

function dragState(part: Part) {
  const existing = dragStates.get(part);
  if (existing) {
    return existing;
  }
  const next = { active: false, pointerId: null, lastY: 0, moved: false };
  dragStates.set(part, next);
  return next;
}

function onPointerDown(event: PointerEvent, part: Part) {
  const current = dragState(part);
  current.active = true;
  current.pointerId = event.pointerId;
  current.lastY = event.clientY;
  current.moved = false;
  (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
  (event.currentTarget as HTMLElement).focus();
}

function onPointerMove(event: PointerEvent, part: Part) {
  const current = dragState(part);
  if (!current.active || current.pointerId !== event.pointerId) {
    return;
  }

  const deltaY = event.clientY - current.lastY;
  if (Math.abs(deltaY) < DRAG_THRESHOLD) {
    return;
  }

  const motion = motions[part];
  if (!motion.dragging) {
    motion.dragging = true;
    activateValue();
    clearInputBuffer(part);
    setExpanded(true);
    if (motion.snapTimer) {
      window.clearTimeout(motion.snapTimer);
      motion.snapTimer = 0;
    }
    startMotion(part);
  }

  motion.targetOffset = clamp(motion.targetOffset + deltaY, -MAX_OFFSET, MAX_OFFSET);
  current.lastY = event.clientY;
  current.moved = true;
  startMotion(part);
}

function clearPointer(event: PointerEvent, part: Part) {
  const current = dragState(part);
  if (current.pointerId !== event.pointerId) {
    return;
  }

  current.active = false;
  current.pointerId = null;
  if (motions[part].dragging) {
    motions[part].dragging = false;
    scheduleSnap(part);
  }
}

function onColumnClick(event: MouseEvent, part: Part) {
  const current = dragState(part);
  if (current.moved) {
    current.moved = false;
    return;
  }

  if (!expanded.value) {
    activateValue();
    setExpanded(true);
    scheduleCollapse();
    return;
  }

  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
  const offsetY = event.clientY - rect.top;
  if (offsetY < ROW_HEIGHT) {
    nudge(part, -1);
  } else if (offsetY > ROW_HEIGHT * 2) {
    nudge(part, 1);
  }
}

function displayValue(part: Part, delta = 0) {
  renderTick.value;
  if (delta === 0 && inputBuffers[part]) {
    return inputBuffers[part];
  }
  return formatPart(part, applyDelta(state, part, delta)[part]);
}

function trackStyle(part: Part) {
  renderTick.value;
  return {
    transform: `translateY(${motions[part].offset}px)`,
  };
}

function partUnit(part: Part) {
  return {
    year: "年",
    month: "月",
    day: "日",
    hour: "时",
    minute: "分",
  }[part];
}

function onDocumentPointerDown(event: PointerEvent) {
  if (!root.value || !(event.target instanceof Node) || root.value.contains(event.target)) {
    return;
  }
  setExpanded(false);
}

onBeforeUnmount(() => {
  if (collapseTimer) {
    window.clearTimeout(collapseTimer);
  }
  orderedParts.value.forEach((part) => {
    if (motions[part].rafId) {
      window.cancelAnimationFrame(motions[part].rafId);
    }
    if (motions[part].snapTimer) {
      window.clearTimeout(motions[part].snapTimer);
    }
    clearInputTimer(part);
  });
  document.removeEventListener("pointerdown", onDocumentPointerDown);
});
</script>

<template>
  <div
    ref="root"
    class="wheel-date-picker account-wheel-picker"
    :class="{ 'is-expanded': expanded, 'is-empty': isEmpty, 'is-datetime': mode === 'datetime' }"
    :data-empty-label="emptyLabel"
  >
    <template v-for="part in orderedParts" :key="part">
      <div
        class="wheel-column"
        :class="{ 'is-focused': focusedPart === part, 'is-buffering': inputBuffers[part] }"
        :data-part="part"
        tabindex="0"
        @click="onColumnClick($event, part)"
        @focus="focusedPart = part"
        @blur="focusedPart = ''; commitInputBuffer(part)"
        @wheel="onWheel($event, part)"
        @keydown="onKeydown($event, part)"
        @pointerdown="onPointerDown($event, part)"
        @pointermove="onPointerMove($event, part)"
        @pointerup="clearPointer($event, part)"
        @pointercancel="clearPointer($event, part)"
      >
        <div class="wheel-track" :style="trackStyle(part)">
          <span class="wheel-item" data-slot="far-prev">{{ displayValue(part, -2) }}</span>
          <span class="wheel-item" data-slot="prev">{{ displayValue(part, -1) }}</span>
          <span class="wheel-item" data-slot="current">{{ displayValue(part) }}</span>
          <span class="wheel-item" data-slot="next">{{ displayValue(part, 1) }}</span>
          <span class="wheel-item" data-slot="far-next">{{ displayValue(part, 2) }}</span>
        </div>
      </div>
      <span v-if="part === 'day' && mode === 'datetime'" class="date-unit date-divider">·</span>
      <span v-else class="date-unit">{{ partUnit(part) }}</span>
    </template>
  </div>
</template>
