(() => {
  const MIN_YEAR = 1000
  const MAX_YEAR = 9999
  const ROW_HEIGHT = 32
  const DRAG_THRESHOLD = 4
  const WHEEL_SENSITIVITY = 0.32
  const WHEEL_MIN_DELTA = 0.35
  const MAX_OFFSET = ROW_HEIGHT * 1.45
  const SNAP_DELAY_MS = 90
  const EASE_ACTIVE = 0.34
  const EASE_SETTLE = 0.2
  const PARTS = ["year", "month", "day"]

  function clamp(value, min, max) {
    return Math.min(max, Math.max(min, value))
  }

  function pad2(value) {
    return String(value).padStart(2, "0")
  }

  function daysInMonth(year, month) {
    return new Date(year, month, 0, 12).getDate()
  }

  function buildDate(year, month, day) {
    return new Date(year, month - 1, day, 12)
  }

  function normalizeWheelDelta(event) {
    if (event.deltaMode === 1) {
      return event.deltaY * 16
    }
    if (event.deltaMode === 2) {
      return event.deltaY * ROW_HEIGHT
    }
    return event.deltaY
  }

  function formatPart(part, value) {
    if (part === "year") {
      return String(value)
    }
    return pad2(value)
  }

  function normalizeState(state) {
    const now = new Date()
    const year = clamp(Number.parseInt(state.year, 10) || now.getFullYear(), MIN_YEAR, MAX_YEAR)
    const month = clamp(Number.parseInt(state.month, 10) || now.getMonth() + 1, 1, 12)
    const day = clamp(Number.parseInt(state.day, 10) || now.getDate(), 1, daysInMonth(year, month))

    return { year, month, day }
  }

  function getHiddenInputs(form) {
    return {
      year: form.querySelector('input[name="year"]'),
      month: form.querySelector('input[name="month"]'),
      day: form.querySelector('input[name="day"]'),
    }
  }

  function shiftMonth(year, month, delta) {
    const rawIndex = year * 12 + (month - 1) + delta
    const minIndex = MIN_YEAR * 12
    const maxIndex = MAX_YEAR * 12 + 11
    const index = clamp(rawIndex, minIndex, maxIndex)

    return {
      year: Math.floor(index / 12),
      month: (index % 12) + 1,
    }
  }

  function applyDelta(state, part, delta) {
    if (part === "year") {
      return normalizeState({
        year: state.year + delta,
        month: state.month,
        day: state.day,
      })
    }

    if (part === "month") {
      const shifted = shiftMonth(state.year, state.month, delta)
      return normalizeState({
        year: shifted.year,
        month: shifted.month,
        day: state.day,
      })
    }

    const shifted = buildDate(state.year, state.month, state.day)
    shifted.setDate(shifted.getDate() + delta)

    if (shifted.getFullYear() < MIN_YEAR) {
      return { year: MIN_YEAR, month: 1, day: 1 }
    }
    if (shifted.getFullYear() > MAX_YEAR) {
      return { year: MAX_YEAR, month: 12, day: 31 }
    }

    return {
      year: shifted.getFullYear(),
      month: shifted.getMonth() + 1,
      day: shifted.getDate(),
    }
  }

  function getPicker(form) {
    if (form._datePicker) {
      return form._datePicker
    }

    const hiddenInputs = getHiddenInputs(form)
    const columns = {}
    const motions = {}

    form.querySelectorAll(".wheel-column").forEach((column) => {
      const part = column.dataset.part
      columns[part] = column
      motions[part] = {
        offset: 0,
        targetOffset: 0,
        rafId: 0,
        snapTimer: 0,
        dragging: false,
      }
    })

    const picker = {
      form,
      hiddenInputs,
      columns,
      motions,
      state: normalizeState({
        year: hiddenInputs.year.value,
        month: hiddenInputs.month.value,
        day: hiddenInputs.day.value,
      }),
    }

    form._datePicker = picker
    return picker
  }

  function syncHiddenInputs(picker) {
    picker.hiddenInputs.year.value = String(picker.state.year)
    picker.hiddenInputs.month.value = pad2(picker.state.month)
    picker.hiddenInputs.day.value = pad2(picker.state.day)
  }

  function syncColumn(column, part, state, offset) {
    const prevTwo = applyDelta(state, part, -2)
    const prevOne = applyDelta(state, part, -1)
    const nextOne = applyDelta(state, part, 1)
    const nextTwo = applyDelta(state, part, 2)

    column.querySelector('[data-slot="far-prev"]').textContent = formatPart(part, prevTwo[part])
    column.querySelector('[data-slot="prev"]').textContent = formatPart(part, prevOne[part])
    column.querySelector('[data-slot="current"]').textContent = formatPart(part, state[part])
    column.querySelector('[data-slot="next"]').textContent = formatPart(part, nextOne[part])
    column.querySelector('[data-slot="far-next"]').textContent = formatPart(part, nextTwo[part])
    column.querySelector(".wheel-track").style.transform = `translateY(${offset}px)`

    column.setAttribute("aria-valuenow", String(state[part]))
    column.setAttribute("aria-valuetext", `${formatPart(part, state[part])}${part === "year" ? "年" : part === "month" ? "月" : "日"}`)

    if (part === "year") {
      column.setAttribute("aria-valuemin", String(MIN_YEAR))
      column.setAttribute("aria-valuemax", String(MAX_YEAR))
    } else if (part === "month") {
      column.setAttribute("aria-valuemin", "1")
      column.setAttribute("aria-valuemax", "12")
    } else {
      column.setAttribute("aria-valuemin", "1")
      column.setAttribute("aria-valuemax", String(daysInMonth(state.year, state.month)))
    }
  }

  function render(picker) {
    syncHiddenInputs(picker)
    PARTS.forEach((part) => {
      syncColumn(picker.columns[part], part, picker.state, picker.motions[part].offset)
    })
  }

  function scheduleSnap(picker, part) {
    const motion = picker.motions[part]
    if (motion.snapTimer) {
      window.clearTimeout(motion.snapTimer)
    }
    motion.snapTimer = window.setTimeout(() => {
      motion.targetOffset = 0
      startMotion(picker, part)
    }, SNAP_DELAY_MS)
  }

  function stepStateIfNeeded(picker, part) {
    const motion = picker.motions[part]
    let changed = false

    while (motion.offset <= -ROW_HEIGHT) {
      picker.state = applyDelta(picker.state, part, 1)
      motion.offset += ROW_HEIGHT
      motion.targetOffset += ROW_HEIGHT
      changed = true
    }

    while (motion.offset >= ROW_HEIGHT) {
      picker.state = applyDelta(picker.state, part, -1)
      motion.offset -= ROW_HEIGHT
      motion.targetOffset -= ROW_HEIGHT
      changed = true
    }

    if (changed) {
      syncHiddenInputs(picker)
    }
  }

  function stopMotionIfSettled(picker, part) {
    const motion = picker.motions[part]
    if (Math.abs(motion.targetOffset - motion.offset) > 0.18 || Math.abs(motion.offset) > 0.18) {
      return false
    }

    motion.offset = 0
    motion.targetOffset = 0
    motion.rafId = 0
    render(picker)
    return true
  }

  function tickMotion(picker, part) {
    const motion = picker.motions[part]
    const ease = motion.dragging ? EASE_ACTIVE : EASE_SETTLE

    motion.offset += (motion.targetOffset - motion.offset) * ease
    stepStateIfNeeded(picker, part)
    render(picker)

    if (stopMotionIfSettled(picker, part)) {
      return
    }

    motion.rafId = window.requestAnimationFrame(() => tickMotion(picker, part))
  }

  function startMotion(picker, part) {
    const motion = picker.motions[part]
    if (motion.rafId) {
      return
    }
    motion.rafId = window.requestAnimationFrame(() => tickMotion(picker, part))
  }

  function nudge(picker, part, delta) {
    const motion = picker.motions[part]
    motion.targetOffset = clamp(
      motion.targetOffset + (delta > 0 ? -ROW_HEIGHT : ROW_HEIGHT),
      -MAX_OFFSET,
      MAX_OFFSET,
    )
    startMotion(picker, part)
    scheduleSnap(picker, part)
  }

  function bindColumn(picker, column) {
    const part = column.dataset.part
    const motion = picker.motions[part]
    const dragState = {
      active: false,
      pointerId: null,
      lastY: 0,
      moved: false,
    }

    column.addEventListener("wheel", (event) => {
      event.preventDefault()
      const delta = normalizeWheelDelta(event)
      if (Math.abs(delta) < WHEEL_MIN_DELTA) {
        return
      }

      motion.targetOffset = clamp(
        motion.targetOffset - delta * WHEEL_SENSITIVITY,
        -MAX_OFFSET,
        MAX_OFFSET,
      )
      startMotion(picker, part)
      scheduleSnap(picker, part)
    }, { passive: false })

    column.addEventListener("keydown", (event) => {
      if (event.key === "ArrowUp" || event.key === "PageUp") {
        event.preventDefault()
        nudge(picker, part, -1)
      }
      if (event.key === "ArrowDown" || event.key === "PageDown") {
        event.preventDefault()
        nudge(picker, part, 1)
      }
    })

    column.addEventListener("pointerdown", (event) => {
      dragState.active = true
      dragState.pointerId = event.pointerId
      dragState.lastY = event.clientY
      dragState.moved = false
      motion.dragging = true
      if (motion.snapTimer) {
        window.clearTimeout(motion.snapTimer)
        motion.snapTimer = 0
      }
      column.setPointerCapture(event.pointerId)
      column.focus()
      startMotion(picker, part)
    })

    column.addEventListener("pointermove", (event) => {
      if (!dragState.active || dragState.pointerId !== event.pointerId) {
        return
      }

      const deltaY = event.clientY - dragState.lastY
      if (Math.abs(deltaY) < DRAG_THRESHOLD) {
        return
      }

      motion.targetOffset = clamp(motion.targetOffset + deltaY, -MAX_OFFSET, MAX_OFFSET)
      dragState.lastY = event.clientY
      dragState.moved = true
      startMotion(picker, part)
    })

    function clearPointer(event) {
      if (dragState.pointerId !== event.pointerId) {
        return
      }

      dragState.active = false
      dragState.pointerId = null
      motion.dragging = false
      scheduleSnap(picker, part)
    }

    column.addEventListener("pointerup", clearPointer)
    column.addEventListener("pointercancel", clearPointer)

    column.addEventListener("click", (event) => {
      if (dragState.moved) {
        dragState.moved = false
        return
      }

      const rect = column.getBoundingClientRect()
      const offsetY = event.clientY - rect.top
      if (offsetY < ROW_HEIGHT) {
        nudge(picker, part, -1)
      } else if (offsetY > ROW_HEIGHT * 2) {
        nudge(picker, part, 1)
      }
    })
  }

  document.querySelectorAll("[data-date-picker]").forEach((form) => {
    const picker = getPicker(form)
    render(picker)

    PARTS.forEach((part) => {
      bindColumn(picker, picker.columns[part])
    })
  })
})()
