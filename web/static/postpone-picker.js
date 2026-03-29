(() => {
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
  const PARTS_BY_MODE = {
    date: ["year", "month", "day"],
    datetime: ["year", "month", "day", "hour", "minute"],
  };
  let dismissBound = false;

  function clamp(value, min, max) {
    return Math.min(max, Math.max(min, value));
  }

  function pad2(value) {
    return String(value).padStart(2, "0");
  }

  function daysInMonth(year, month) {
    return new Date(year, month, 0, 12).getDate();
  }

  function normalizeWheelDelta(event) {
    if (event.deltaMode === 1) {
      return event.deltaY * 16;
    }
    if (event.deltaMode === 2) {
      return event.deltaY * ROW_HEIGHT;
    }
    return event.deltaY;
  }

  function formatPart(part, value) {
    if (part === "year") {
      return String(value);
    }
    return pad2(value);
  }

  function buildDateFromState(mode, state) {
    if (mode === "datetime") {
      return new Date(state.year, state.month - 1, state.day, state.hour, state.minute, 0, 0);
    }
    return new Date(state.year, state.month - 1, state.day, 12, 0, 0, 0);
  }

  function dateToState(dateValue) {
    return {
      year: dateValue.getFullYear(),
      month: dateValue.getMonth() + 1,
      day: dateValue.getDate(),
      hour: dateValue.getHours(),
      minute: dateValue.getMinutes(),
    };
  }

  function parseValue(mode, rawValue) {
    const raw = String(rawValue || "").trim();
    if (!raw) {
      return null;
    }

    if (mode === "datetime") {
      const match = raw.match(/^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/);
      if (!match) {
        return null;
      }
      return new Date(
        Number.parseInt(match[1], 10),
        Number.parseInt(match[2], 10) - 1,
        Number.parseInt(match[3], 10),
        Number.parseInt(match[4], 10),
        Number.parseInt(match[5], 10),
        0,
        0,
      );
    }

    const match = raw.match(/^(\d{4})-(\d{2})-(\d{2})$/);
    if (!match) {
      return null;
    }
    return new Date(
      Number.parseInt(match[1], 10),
      Number.parseInt(match[2], 10) - 1,
      Number.parseInt(match[3], 10),
      12,
      0,
      0,
      0,
    );
  }

  function currentMinimumForMode(mode) {
    const now = new Date();
    if (mode === "datetime") {
      const rounded = new Date(
        now.getFullYear(),
        now.getMonth(),
        now.getDate(),
        now.getHours(),
        now.getMinutes(),
        0,
        0,
      );
      if (rounded <= now) {
        rounded.setMinutes(rounded.getMinutes() + 1);
      }
      return rounded;
    }

    const nextDay = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 12, 0, 0, 0);
    nextDay.setDate(nextDay.getDate() + 1);
    return nextDay;
  }

  function formatValue(mode, state) {
    if (mode === "datetime") {
      return `${state.year}-${pad2(state.month)}-${pad2(state.day)}T${pad2(state.hour)}:${pad2(state.minute)}`;
    }
    return `${state.year}-${pad2(state.month)}-${pad2(state.day)}`;
  }

  function normalizeState(mode, rawState, minDate) {
    const now = new Date();
    const base = {
      year: clamp(Number.parseInt(rawState.year, 10) || now.getFullYear(), MIN_YEAR, MAX_YEAR),
      month: clamp(Number.parseInt(rawState.month, 10) || now.getMonth() + 1, 1, 12),
      day: 1,
      hour: clamp(Number.parseInt(rawState.hour, 10) || 0, 0, 23),
      minute: clamp(Number.parseInt(rawState.minute, 10) || 0, 0, 59),
    };

    base.day = clamp(Number.parseInt(rawState.day, 10) || now.getDate(), 1, daysInMonth(base.year, base.month));

    let normalized = buildDateFromState(mode, base);
    if (normalized.getFullYear() < MIN_YEAR) {
      normalized = buildDateFromState(mode, { year: MIN_YEAR, month: 1, day: 1, hour: 0, minute: 0 });
    }
    if (normalized.getFullYear() > MAX_YEAR) {
      normalized = buildDateFromState(mode, { year: MAX_YEAR, month: 12, day: 31, hour: 23, minute: 59 });
    }
    if (minDate && normalized < minDate) {
      normalized = new Date(minDate.getTime());
    }
    return dateToState(normalized);
  }

  function getPicker(root) {
    if (root._postponePicker) {
      return root._postponePicker;
    }

    const mode = root.dataset.pickerMode === "datetime" ? "datetime" : "date";
    const parts = PARTS_BY_MODE[mode];
    const targetInput = root.querySelector("[data-picker-value]") || root.querySelector("input[type='hidden']");
    const columns = {};
    const motions = {};
    const inputBuffers = {};
    const inputTimers = {};

    root.querySelectorAll(".wheel-column").forEach((column) => {
      const part = column.dataset.part;
      columns[part] = column;
      motions[part] = {
        offset: 0,
        targetOffset: 0,
        rafId: 0,
        snapTimer: 0,
        dragging: false,
      };
      inputBuffers[part] = "";
      inputTimers[part] = 0;
    });

    const enforceCurrentMin = root.hasAttribute("data-postpone-picker") || root.dataset.enforceCurrentMin === "1";
    const allowEmpty = root.dataset.allowEmpty === "1";
    const configuredMinDate = parseValue(mode, root.dataset.minValue);
    const currentMinDate = enforceCurrentMin ? currentMinimumForMode(mode) : null;
    let minDate = configuredMinDate || null;
    if (currentMinDate && (!minDate || currentMinDate > minDate)) {
      minDate = currentMinDate;
    }
    const parsedInitialDate = parseValue(mode, root.dataset.initialValue || targetInput?.value || "");
    const initialDate = parsedInitialDate || minDate || new Date();
    if (minDate) {
      root.dataset.minValue = formatValue(mode, dateToState(minDate));
    }

    const picker = {
      root,
      mode,
      parts,
      targetInput,
      columns,
      motions,
      inputBuffers,
      inputTimers,
      collapseTimer: 0,
      minDate,
      allowEmpty,
      emptyLabel: root.dataset.emptyLabel || "未设置",
      hasValue: !allowEmpty || parsedInitialDate !== null,
      state: normalizeState(mode, dateToState(initialDate), minDate),
    };

    root._postponePicker = picker;
    return picker;
  }

  function syncHiddenInput(picker) {
    if (picker.targetInput) {
      picker.targetInput.value = picker.hasValue ? formatValue(picker.mode, picker.state) : "";
    }
    picker.root.dataset.initialValue = picker.hasValue ? formatValue(picker.mode, picker.state) : "";
  }

  function activatePickerValue(picker) {
    if (!picker.allowEmpty || picker.hasValue) {
      return;
    }
    picker.hasValue = true;
    syncHiddenInput(picker);
  }

  function setExpanded(picker, expanded) {
    if (picker.collapseTimer) {
      window.clearTimeout(picker.collapseTimer);
      picker.collapseTimer = 0;
    }
    picker.root.classList.toggle("is-expanded", expanded);
  }

  function clearInputTimer(picker, part) {
    if (picker.inputTimers[part]) {
      window.clearTimeout(picker.inputTimers[part]);
      picker.inputTimers[part] = 0;
    }
  }

  function clearInputBuffer(picker, part) {
    clearInputTimer(picker, part);
    picker.inputBuffers[part] = "";
    picker.columns[part].classList.remove("is-buffering");
  }

  function getBufferedDisplay(picker, part) {
    const buffer = picker.inputBuffers[part];
    if (!buffer) {
      return formatPart(part, picker.state[part]);
    }
    return buffer;
  }

  function hasActiveMotion(picker) {
    return picker.parts.some((part) => {
      const motion = picker.motions[part];
      return (
        motion.dragging ||
        motion.rafId !== 0 ||
        motion.snapTimer !== 0 ||
        Math.abs(motion.offset) > 0.18 ||
        Math.abs(motion.targetOffset) > 0.18
      );
    });
  }

  function scheduleCollapse(picker) {
    if (picker.collapseTimer) {
      window.clearTimeout(picker.collapseTimer);
    }

    picker.collapseTimer = window.setTimeout(() => {
      picker.collapseTimer = 0;
      if (picker.parts.some((part) => picker.inputBuffers[part])) {
        scheduleCollapse(picker);
        return;
      }
      if (hasActiveMotion(picker)) {
        scheduleCollapse(picker);
        return;
      }
      setExpanded(picker, false);
    }, COLLAPSE_DELAY_MS);
  }

  function applyDelta(picker, state, part, delta) {
    let nextDate = buildDateFromState(picker.mode, state);

    if (part === "year") {
      const targetYear = clamp(state.year + delta, MIN_YEAR, MAX_YEAR);
      const targetDay = clamp(state.day, 1, daysInMonth(targetYear, state.month));
      nextDate = buildDateFromState(picker.mode, {
        year: targetYear,
        month: state.month,
        day: targetDay,
        hour: state.hour,
        minute: state.minute,
      });
    } else if (part === "month") {
      const rawIndex = state.year * 12 + (state.month - 1) + delta;
      const minIndex = MIN_YEAR * 12;
      const maxIndex = MAX_YEAR * 12 + 11;
      const index = clamp(rawIndex, minIndex, maxIndex);
      const year = Math.floor(index / 12);
      const month = (index % 12) + 1;
      const day = clamp(state.day, 1, daysInMonth(year, month));
      nextDate = buildDateFromState(picker.mode, {
        year,
        month,
        day,
        hour: state.hour,
        minute: state.minute,
      });
    } else if (part === "day") {
      nextDate.setDate(nextDate.getDate() + delta);
    } else if (part === "hour") {
      nextDate.setHours(nextDate.getHours() + delta);
    } else if (part === "minute") {
      nextDate.setMinutes(nextDate.getMinutes() + delta);
    }

    return normalizeState(picker.mode, dateToState(nextDate), picker.minDate);
  }

  function syncColumn(picker, column, part, state, offset) {
    const prevTwo = applyDelta(picker, state, part, -2);
    const prevOne = applyDelta(picker, state, part, -1);
    const nextOne = applyDelta(picker, state, part, 1);
    const nextTwo = applyDelta(picker, state, part, 2);

    column.querySelector('[data-slot="far-prev"]').textContent = formatPart(part, prevTwo[part]);
    column.querySelector('[data-slot="prev"]').textContent = formatPart(part, prevOne[part]);
    column.querySelector('[data-slot="current"]').textContent = getBufferedDisplay(picker, part);
    column.querySelector('[data-slot="next"]').textContent = formatPart(part, nextOne[part]);
    column.querySelector('[data-slot="far-next"]').textContent = formatPart(part, nextTwo[part]);
    column.querySelector(".wheel-track").style.transform = `translateY(${offset}px)`;

    column.setAttribute("aria-valuenow", String(state[part]));
    column.setAttribute("aria-valuetext", `${formatPart(part, state[part])}`);
  }

  function render(picker) {
    syncHiddenInput(picker);
    picker.root.dataset.emptyLabel = picker.emptyLabel;
    picker.root.classList.toggle("is-empty", picker.allowEmpty && !picker.hasValue);
    picker.parts.forEach((part) => {
      syncColumn(picker, picker.columns[part], part, picker.state, picker.motions[part].offset);
    });
  }

  function scheduleSnap(picker, part) {
    const motion = picker.motions[part];
    if (motion.snapTimer) {
      window.clearTimeout(motion.snapTimer);
    }
    motion.snapTimer = window.setTimeout(() => {
      motion.snapTimer = 0;
      motion.targetOffset = 0;
      startMotion(picker, part);
    }, SNAP_DELAY_MS);
  }

  function stepStateIfNeeded(picker, part) {
    const motion = picker.motions[part];
    let changed = false;

    while (motion.offset <= -ROW_HEIGHT) {
      picker.state = applyDelta(picker, picker.state, part, 1);
      motion.offset += ROW_HEIGHT;
      motion.targetOffset += ROW_HEIGHT;
      changed = true;
    }

    while (motion.offset >= ROW_HEIGHT) {
      picker.state = applyDelta(picker, picker.state, part, -1);
      motion.offset -= ROW_HEIGHT;
      motion.targetOffset -= ROW_HEIGHT;
      changed = true;
    }

    if (changed) {
      syncHiddenInput(picker);
    }
  }

  function stopMotionIfSettled(picker, part) {
    const motion = picker.motions[part];
    if (Math.abs(motion.targetOffset - motion.offset) > 0.18 || Math.abs(motion.offset) > 0.18) {
      return false;
    }

    motion.offset = 0;
    motion.targetOffset = 0;
    motion.rafId = 0;
    render(picker);
    scheduleCollapse(picker);
    return true;
  }

  function tickMotion(picker, part) {
    const motion = picker.motions[part];
    const ease = motion.dragging ? EASE_ACTIVE : EASE_SETTLE;

    motion.offset += (motion.targetOffset - motion.offset) * ease;
    stepStateIfNeeded(picker, part);
    render(picker);

    if (stopMotionIfSettled(picker, part)) {
      return;
    }

    motion.rafId = window.requestAnimationFrame(() => tickMotion(picker, part));
  }

  function startMotion(picker, part) {
    const motion = picker.motions[part];
    if (motion.rafId) {
      return;
    }
    motion.rafId = window.requestAnimationFrame(() => tickMotion(picker, part));
  }

  function nudge(picker, part, delta) {
    const motion = picker.motions[part];
    activatePickerValue(picker);
    clearInputBuffer(picker, part);
    setExpanded(picker, true);
    motion.targetOffset = clamp(
      motion.targetOffset + (delta > 0 ? -ROW_HEIGHT : ROW_HEIGHT),
      -MAX_OFFSET,
      MAX_OFFSET,
    );
    startMotion(picker, part);
    scheduleSnap(picker, part);
  }

  function commitInputBuffer(picker, part) {
    const buffer = picker.inputBuffers[part];
    clearInputTimer(picker, part);
    if (!buffer) {
      return;
    }

    const parsed = Number.parseInt(buffer, 10);
    if (Number.isNaN(parsed)) {
      clearInputBuffer(picker, part);
      render(picker);
      return;
    }

    if (part === "year" && buffer.length < 4) {
      clearInputBuffer(picker, part);
      render(picker);
      scheduleCollapse(picker);
      return;
    }

    picker.state = normalizeState(
      picker.mode,
      {
        ...picker.state,
        [part]: parsed,
      },
      picker.minDate,
    );

    clearInputBuffer(picker, part);
    render(picker);
    scheduleCollapse(picker);
  }

  function queueInputCommit(picker, part) {
    clearInputTimer(picker, part);
    picker.inputTimers[part] = window.setTimeout(() => {
      picker.inputTimers[part] = 0;
      commitInputBuffer(picker, part);
    }, INPUT_COMMIT_DELAY_MS);
  }

  function bindColumn(picker, column) {
    const part = column.dataset.part;
    const motion = picker.motions[part];
    const dragState = {
      active: false,
      pointerId: null,
      lastY: 0,
      moved: false,
    };

    column.addEventListener("wheel", (event) => {
      event.preventDefault();
      const delta = normalizeWheelDelta(event);
      if (Math.abs(delta) < WHEEL_MIN_DELTA) {
        return;
      }

      clearInputBuffer(picker, part);
      activatePickerValue(picker);
      setExpanded(picker, true);
      motion.targetOffset = clamp(
        motion.targetOffset - delta * WHEEL_SENSITIVITY,
        -MAX_OFFSET,
        MAX_OFFSET,
      );
      startMotion(picker, part);
      scheduleSnap(picker, part);
    }, { passive: false });

    column.addEventListener("keydown", (event) => {
      if (event.key === "ArrowUp" || event.key === "PageUp") {
        event.preventDefault();
        nudge(picker, part, -1);
        return;
      }
      if (event.key === "ArrowDown" || event.key === "PageDown") {
        event.preventDefault();
        nudge(picker, part, 1);
        return;
      }
      if (/^\d$/.test(event.key)) {
        event.preventDefault();
        const maxLength = part === "year" ? 4 : 2;
        const nextBuffer = (picker.inputBuffers[part] + event.key).slice(-maxLength);
        activatePickerValue(picker);
        picker.inputBuffers[part] = nextBuffer;
        column.classList.add("is-buffering");
        render(picker);
        if (nextBuffer.length >= maxLength) {
          commitInputBuffer(picker, part);
          return;
        }
        queueInputCommit(picker, part);
        return;
      }
      if (event.key === "Backspace") {
        event.preventDefault();
        if (!picker.inputBuffers[part]) {
          return;
        }
        picker.inputBuffers[part] = picker.inputBuffers[part].slice(0, -1);
        if (!picker.inputBuffers[part]) {
          clearInputBuffer(picker, part);
        } else {
          queueInputCommit(picker, part);
        }
        render(picker);
        return;
      }
      if (event.key === "Enter") {
        event.preventDefault();
        commitInputBuffer(picker, part);
      }
    });

    column.addEventListener("pointerdown", (event) => {
      dragState.active = true;
      dragState.pointerId = event.pointerId;
      dragState.lastY = event.clientY;
      dragState.moved = false;
      column.setPointerCapture(event.pointerId);
      column.focus();
    });

    column.addEventListener("pointermove", (event) => {
      if (!dragState.active || dragState.pointerId !== event.pointerId) {
        return;
      }

      const deltaY = event.clientY - dragState.lastY;
      if (Math.abs(deltaY) < DRAG_THRESHOLD) {
        return;
      }

      if (!motion.dragging) {
        motion.dragging = true;
        activatePickerValue(picker);
        clearInputBuffer(picker, part);
        setExpanded(picker, true);
        if (motion.snapTimer) {
          window.clearTimeout(motion.snapTimer);
          motion.snapTimer = 0;
        }
        startMotion(picker, part);
      }

      motion.targetOffset = clamp(motion.targetOffset + deltaY, -MAX_OFFSET, MAX_OFFSET);
      dragState.lastY = event.clientY;
      dragState.moved = true;
      startMotion(picker, part);
    });

    function clearPointer(event) {
      if (dragState.pointerId !== event.pointerId) {
        return;
      }

      dragState.active = false;
      dragState.pointerId = null;
      if (motion.dragging) {
        motion.dragging = false;
        scheduleSnap(picker, part);
      }
    }

    column.addEventListener("pointerup", clearPointer);
    column.addEventListener("pointercancel", clearPointer);
    column.addEventListener("focus", () => {
      column.classList.add("is-focused");
    });
    column.addEventListener("blur", () => {
      column.classList.remove("is-focused");
      commitInputBuffer(picker, part);
    });

    column.addEventListener("click", (event) => {
      if (dragState.moved) {
        dragState.moved = false;
        return;
      }

      if (!picker.root.classList.contains("is-expanded")) {
        activatePickerValue(picker);
        setExpanded(picker, true);
        scheduleCollapse(picker);
        return;
      }

      const rect = column.getBoundingClientRect();
      const offsetY = event.clientY - rect.top;
      if (offsetY < ROW_HEIGHT) {
        nudge(picker, part, -1);
      } else if (offsetY > ROW_HEIGHT * 2) {
        nudge(picker, part, 1);
      }
    });
  }

  function initPostponePickers(root = document) {
    root.querySelectorAll("[data-postpone-picker], [data-composer-picker]").forEach((element) => {
      if (element.dataset.postponePickerBound === "1") {
        return;
      }
      element.dataset.postponePickerBound = "1";

      const picker = getPicker(element);
      render(picker);
      picker.parts.forEach((part) => {
        bindColumn(picker, picker.columns[part]);
      });
    });

    if (!dismissBound) {
      dismissBound = true;
      document.addEventListener("pointerdown", (event) => {
        const target = event.target;
        document.querySelectorAll("[data-postpone-panel][open]").forEach((panel) => {
          if (target instanceof Node && panel.contains(target)) {
            return;
          }
          panel.removeAttribute("open");
        });
      });

      document.addEventListener("keydown", (event) => {
        if (event.key !== "Escape") {
          return;
        }
        document.querySelectorAll("[data-postpone-panel][open]").forEach((panel) => {
          panel.removeAttribute("open");
        });
      });
    }
  }

  function setWheelPickerValue(root, rawValue) {
    if (!root) {
      return false;
    }
    const picker = getPicker(root);
    if (String(rawValue || "").trim() === "" && picker.allowEmpty) {
      picker.hasValue = false;
      render(picker);
      scheduleCollapse(picker);
      return true;
    }
    const parsed = parseValue(picker.mode, rawValue);
    if (!parsed) {
      return false;
    }
    picker.hasValue = true;
    picker.state = normalizeState(picker.mode, dateToState(parsed), picker.minDate);
    render(picker);
    scheduleCollapse(picker);
    return true;
  }

  window.initializePostponePickers = initPostponePickers;
  window.setWheelPickerValue = setWheelPickerValue;
  document.addEventListener("DOMContentLoaded", () => {
    initPostponePickers(document);
  });
})();
