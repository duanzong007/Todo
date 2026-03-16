const EXIT_ANIMATION_MS = 320;
const MOVE_ANIMATION_MS = 320;
const ENTER_ANIMATION_MS = 320;
const EASE = "cubic-bezier(0.22, 0.61, 0.36, 1)";
let taskMutationQueue = Promise.resolve();
let pendingTaskRequestCount = 0;
let requiresSnapshotResync = false;

function wait(ms) {
  return new Promise((resolve) => {
    window.setTimeout(resolve, ms);
  });
}

function afterTransition(duration, callback) {
  window.setTimeout(callback, duration);
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function focusCounterElements() {
  return Array.from(document.querySelectorAll(".focus-counter"));
}

function updatePendingCounterState() {
  focusCounterElements().forEach((element) => {
    element.classList.toggle("is-pending", pendingTaskRequestCount > 0);
    element.setAttribute("data-pending-count", String(pendingTaskRequestCount));
  });
}

function beginPendingTaskRequest(options = {}) {
  const { needsResync = true } = options;
  if (needsResync && pendingTaskRequestCount > 0) {
    requiresSnapshotResync = true;
  }
  pendingTaskRequestCount += 1;
  updatePendingCounterState();
}

function currentFocusDateValue() {
  const fromComposer = document.querySelector(".composer-panel input[name='return_date']");
  if (fromComposer?.value) {
    return fromComposer.value;
  }

  const url = new URL(window.location.href);
  const fromQuery = (url.searchParams.get("date") || "").trim();
  if (fromQuery) {
    return fromQuery;
  }

  return "";
}

async function resyncDashboardSnapshot() {
  const url = new URL("/dashboard/snapshot", window.location.origin);
  const focusDate = currentFocusDateValue();
  if (focusDate) {
    url.searchParams.set("date", focusDate);
  }

  const response = await fetch(url.pathname + url.search, {
    headers: {
      "X-Requested-With": "fetch",
    },
  });
  if (!response.ok) {
    throw new Error("snapshot sync failed");
  }

  const snapshot = await response.json();
  applyTaskSnapshot(snapshot);
}

function endPendingTaskRequest() {
  pendingTaskRequestCount = Math.max(0, pendingTaskRequestCount - 1);
  updatePendingCounterState();

  if (pendingTaskRequestCount !== 0 || !requiresSnapshotResync) {
    return Promise.resolve();
  }

  requiresSnapshotResync = false;
  return resyncDashboardSnapshot().catch(() => {
    window.location.reload();
  });
}

function cssEscape(value) {
  if (window.CSS && typeof window.CSS.escape === "function") {
    return window.CSS.escape(value);
  }
  return String(value ?? "").replace(/["\\]/g, "\\$&");
}

function createElementFromHTML(html) {
  const template = document.createElement("template");
  template.innerHTML = html.trim();
  return template.content.firstElementChild;
}

function actionTargetFor(form) {
  if (form.hasAttribute("data-restore-form")) {
    return form.closest(".archive-card");
  }
  return form.closest("[data-task-card]");
}

function taskIDForForm(form) {
  return actionTargetFor(form)?.getAttribute("data-task-id") || "";
}

function captureRects(root, selector) {
  const rects = new Map();
  root.querySelectorAll(selector).forEach((element) => {
    const taskID = element.getAttribute("data-task-id");
    if (!taskID) {
      return;
    }
    rects.set(taskID, element.getBoundingClientRect());
  });
  return rects;
}

function resetInlineAnimation(element) {
  if (!element) {
    return;
  }

  element.classList.remove("is-leaving", "is-restoring");
  element.style.position = "";
  element.style.left = "";
  element.style.top = "";
  element.style.width = "";
  element.style.zIndex = "";
  element.style.boxSizing = "";
  element.style.margin = "";
  element.style.height = "";
  element.style.overflow = "";
  element.style.opacity = "";
  element.style.transform = "";
  element.style.filter = "";
  element.style.paddingTop = "";
  element.style.paddingBottom = "";
  element.style.borderTopWidth = "";
  element.style.borderBottomWidth = "";
  element.style.transition = "";
}

function animateHeightMutation(element, mutate) {
  if (!element) {
    return;
  }

  const startHeight = element.getBoundingClientRect().height;
  element.style.height = `${startHeight}px`;
  element.style.overflow = "hidden";
  element.style.transition = "none";
  mutate();
  const endHeight = element.scrollHeight;

  if (Math.abs(endHeight - startHeight) < 0.5) {
    element.style.height = "";
    element.style.overflow = "";
    element.style.transition = "";
    return;
  }

  element.getBoundingClientRect();
  window.requestAnimationFrame(() => {
    element.style.transition = `height ${MOVE_ANIMATION_MS}ms ${EASE}`;
    element.style.height = `${endHeight}px`;
  });

  afterTransition(MOVE_ANIMATION_MS, () => {
    element.style.height = "";
    element.style.overflow = "";
    element.style.transition = "";
  });
}

function detachForExit(element) {
  if (!element) {
    return null;
  }

  const rect = element.getBoundingClientRect();
  const styles = window.getComputedStyle(element);
  const placeholder = document.createElement("div");
  placeholder.className = "task-flow-placeholder";
  placeholder.style.height = `${rect.height}px`;
  placeholder.style.opacity = "1";
  placeholder.style.overflow = "hidden";
  placeholder.style.marginBottom = styles.marginBottom;
  placeholder.style.transition = [
    `height ${EXIT_ANIMATION_MS}ms ${EASE}`,
    `opacity ${EXIT_ANIMATION_MS}ms ${EASE}`,
    `margin-bottom ${EXIT_ANIMATION_MS}ms ${EASE}`,
  ].join(", ");

  element.replaceWith(placeholder);
  document.body.appendChild(element);
  resetInlineAnimation(element);
  element.style.position = "fixed";
  element.style.left = `${rect.left}px`;
  element.style.top = `${rect.top}px`;
  element.style.width = `${rect.width}px`;
  element.style.height = `${rect.height}px`;
  element.style.zIndex = "40";
  element.style.margin = "0";
  element.style.boxSizing = "border-box";
  element.style.pointerEvents = "none";
  element.style.transition = [
    `opacity ${EXIT_ANIMATION_MS}ms ${EASE}`,
    `transform ${EXIT_ANIMATION_MS}ms ${EASE}`,
    `filter ${EXIT_ANIMATION_MS}ms ease`,
  ].join(", ");

  const cleanup = () => {
    placeholder.remove();
    element.remove();
  };

  const restore = () => {
    if (!placeholder.isConnected) {
      return;
    }
    resetInlineAnimation(element);
    placeholder.replaceWith(element);
  };

  window.requestAnimationFrame(() => {
    placeholder.style.height = "0px";
    placeholder.style.opacity = "0";
    placeholder.style.marginBottom = "0px";
    element.style.opacity = "0";
    element.style.transform = "translateY(-8px) scale(0.985)";
    element.style.filter = "blur(0.9px)";
  });

  return {
    restore,
    done: wait(EXIT_ANIMATION_MS).then(() => {
      cleanup();
    }),
  };
}

function clearEnterMetrics(element) {
  if (!element) {
    return;
  }

  delete element.dataset.enterHeight;
  delete element.dataset.enterPaddingTop;
  delete element.dataset.enterPaddingBottom;
  delete element.dataset.enterBorderTopWidth;
  delete element.dataset.enterBorderBottomWidth;
  delete element.dataset.enterMarginBottom;
}

function stageEnter(element, collapseSpace = false) {
  if (!element) {
    return;
  }

  if (collapseSpace) {
    const rect = element.getBoundingClientRect();
    const styles = window.getComputedStyle(element);
    element.dataset.enterHeight = `${rect.height}`;
    element.dataset.enterPaddingTop = styles.paddingTop;
    element.dataset.enterPaddingBottom = styles.paddingBottom;
    element.dataset.enterBorderTopWidth = styles.borderTopWidth;
    element.dataset.enterBorderBottomWidth = styles.borderBottomWidth;
    element.dataset.enterMarginBottom = styles.marginBottom;
    element.style.height = "0px";
    element.style.overflow = "hidden";
    element.style.paddingTop = "0px";
    element.style.paddingBottom = "0px";
    element.style.borderTopWidth = "0px";
    element.style.borderBottomWidth = "0px";
    element.style.marginBottom = "0px";
  }

  element.style.opacity = "0";
  element.style.transform = "translateY(8px) scale(0.985)";
  element.style.filter = "blur(0.8px)";
}

function expandEnterSpace(element) {
  if (!element || !element.dataset.enterHeight) {
    return;
  }

  window.requestAnimationFrame(() => {
    element.style.transition = [
      `height ${MOVE_ANIMATION_MS}ms ${EASE}`,
      `padding-top ${MOVE_ANIMATION_MS}ms ${EASE}`,
      `padding-bottom ${MOVE_ANIMATION_MS}ms ${EASE}`,
      `border-top-width ${MOVE_ANIMATION_MS}ms ${EASE}`,
      `border-bottom-width ${MOVE_ANIMATION_MS}ms ${EASE}`,
      `margin-bottom ${MOVE_ANIMATION_MS}ms ${EASE}`,
    ].join(", ");
    element.style.height = element.dataset.enterHeight;
    element.style.paddingTop = element.dataset.enterPaddingTop;
    element.style.paddingBottom = element.dataset.enterPaddingBottom;
    element.style.borderTopWidth = element.dataset.enterBorderTopWidth;
    element.style.borderBottomWidth = element.dataset.enterBorderBottomWidth;
    element.style.marginBottom = element.dataset.enterMarginBottom;
  });
}

function revealEnter(element, collapseSpace = false) {
  if (!element) {
    return;
  }

  window.requestAnimationFrame(() => {
    element.style.transition = [
      collapseSpace ? element.style.transition : "",
      `opacity ${ENTER_ANIMATION_MS}ms ${EASE}`,
      `transform ${ENTER_ANIMATION_MS}ms ${EASE}`,
      `filter ${ENTER_ANIMATION_MS}ms ease`,
    ].filter(Boolean).join(", ");
    element.style.opacity = "1";
    element.style.transform = "translateY(0) scale(1)";
    element.style.filter = "blur(0)";
  });

  element.addEventListener("transitionend", () => {
    resetInlineAnimation(element);
    clearEnterMetrics(element);
  }, { once: true });
}

function animateMovedElements(container, selector, previousRects) {
  if (!container) {
    return;
  }

  container.querySelectorAll(selector).forEach((element) => {
    const taskID = element.getAttribute("data-task-id");
    if (!taskID) {
      return;
    }

    const before = previousRects.get(taskID);
    if (!before) {
      return;
    }

    const after = element.getBoundingClientRect();
    const deltaX = before.left - after.left;
    const deltaY = before.top - after.top;
    if (Math.abs(deltaX) < 0.5 && Math.abs(deltaY) < 0.5) {
      return;
    }

    element.style.transition = "none";
    element.style.transform = `translate(${deltaX}px, ${deltaY}px)`;
    element.getBoundingClientRect();

    window.requestAnimationFrame(() => {
      element.style.transition = `transform ${MOVE_ANIMATION_MS}ms ${EASE}`;
      element.style.transform = "";
      element.addEventListener("transitionend", () => {
        element.style.transition = "";
      }, { once: true });
    });
  });
}

function buildFocusTaskCardHTML(card) {
  return `
    <article class="focus-card" data-task-card data-task-id="${escapeHtml(card.id)}" data-kind-label="${escapeHtml(card.kind_label)}" data-kind-class="${escapeHtml(card.kind_class)}">
      <div class="focus-card-main">
        <span class="task-kind task-kind-${escapeHtml(card.kind_class)}">${escapeHtml(card.kind_label)}</span>
        <div class="task-body">
          <h3>${escapeHtml(card.title)}</h3>
          ${card.status_line ? `<p class="status">${escapeHtml(card.status_line)}</p>` : ""}
          ${card.note ? `<p class="note">${escapeHtml(card.note)}</p>` : ""}
        </div>
      </div>
      <div class="task-actions focus-actions">
        ${card.can_postpone ? buildPostponeControlHTML(card) : ""}
        ${card.can_complete ? `
          <form action="/tasks/${escapeHtml(card.id)}/complete" method="post" data-complete-form data-async-task-form>
            <input type="hidden" name="return_date" value="${escapeHtml(card.return_date)}">
            <button type="submit" class="complete-toggle" aria-label="确认完成" title="确认完成">
              <span class="visually-hidden">确认完成</span>
            </button>
          </form>
        ` : ""}
      </div>
    </article>
  `;
}

function buildCompletedTaskCardHTML(card) {
  return `
    <article class="archive-card" data-task-id="${escapeHtml(card.id)}" data-kind-label="${escapeHtml(card.kind_label)}" data-kind-class="${escapeHtml(card.kind_class)}" data-status-line="${escapeHtml(card.status_line || "")}" data-can-postpone="${card.can_postpone ? "1" : "0"}" data-postpone-mode="${escapeHtml(card.postpone_mode || "")}" data-postpone-value="${escapeHtml(card.postpone_value || "")}" data-postpone-min-value="${escapeHtml(card.postpone_min_value || "")}">
      <div class="archive-card-main">
        <span class="task-kind task-kind-${escapeHtml(card.kind_class)}">${escapeHtml(card.kind_label)}</span>
        <div class="task-body">
          <h3>${escapeHtml(card.title)}</h3>
          <p class="status">${escapeHtml(card.finished_line)}</p>
          ${card.note ? `<p class="note">${escapeHtml(card.note)}</p>` : ""}
        </div>
        <form action="/tasks/${escapeHtml(card.id)}/restore" method="post" class="archive-actions" data-restore-form data-async-task-form>
          <input type="hidden" name="return_date" value="${escapeHtml(card.return_date)}">
          <button type="submit" class="secondary archive-restore">撤销</button>
        </form>
      </div>
    </article>
  `;
}

function buildWheelColumnHTML(part, ariaLabel) {
  return `
    <div class="wheel-column" data-part="${escapeHtml(part)}" tabindex="0" role="spinbutton" aria-label="${escapeHtml(ariaLabel)}">
      <div class="wheel-track">
        <span class="wheel-item" data-slot="far-prev"></span>
        <span class="wheel-item" data-slot="prev"></span>
        <span class="wheel-item" data-slot="current"></span>
        <span class="wheel-item" data-slot="next"></span>
        <span class="wheel-item" data-slot="far-next"></span>
      </div>
    </div>
  `;
}

function buildPostponeControlHTML(card) {
  const mode = card.postpone_mode === "datetime" ? "datetime" : "date";
  const minValue = card.postpone_min_value || card.postpone_value || "";

  return `
    <details class="inline-postpone" data-postpone-panel data-task-id="${escapeHtml(card.id)}">
      <summary>延期</summary>
      <form action="/tasks/${escapeHtml(card.id)}/postpone" method="post" class="postpone-form-panel" data-postpone-form data-async-task-form>
        <input type="hidden" name="return_date" value="${escapeHtml(card.return_date)}">
        <div class="wheel-date-picker postpone-wheel-picker${mode === "datetime" ? " is-datetime" : ""}" data-postpone-picker data-picker-mode="${mode}" data-initial-value="${escapeHtml(card.postpone_value || minValue)}" data-min-value="${escapeHtml(minValue)}">
          <input type="hidden" name="target_value" value="${escapeHtml(card.postpone_value || minValue)}">
          ${buildWheelColumnHTML("year", "年份")}
          <span class="date-unit">年</span>
          ${buildWheelColumnHTML("month", "月份")}
          <span class="date-unit">月</span>
          ${buildWheelColumnHTML("day", "日期")}
          <span class="date-unit">日</span>
          ${mode === "datetime" ? `
            <span class="date-unit date-divider">·</span>
            ${buildWheelColumnHTML("hour", "小时")}
            <span class="date-unit">时</span>
            ${buildWheelColumnHTML("minute", "分钟")}
            <span class="date-unit">分</span>
          ` : ""}
        </div>
        <button type="submit" class="secondary">确认</button>
      </form>
    </details>
  `;
}

function padNumber(value) {
  return String(value).padStart(2, "0");
}

function formatOptimisticCompletedLine(kindClass) {
  const now = new Date();
  const dateText = `${now.getMonth() + 1}月${now.getDate()}日`;
  if (kindClass === "ddl") {
    return `完成于 ${dateText} ${padNumber(now.getHours())}:${padNumber(now.getMinutes())}`;
  }
  return `完成于 ${dateText}`;
}

function extractCardText(element, selector) {
  return element?.querySelector(selector)?.textContent?.trim() || "";
}

function extractCardKind(element) {
  const label = element?.getAttribute("data-kind-label") || extractCardText(element, ".task-kind");
  const kindClass = element?.getAttribute("data-kind-class") || "";
  return {
    label,
    kindClass,
  };
}

function extractReturnDate(element) {
  return element?.querySelector("input[name='return_date']")?.value || currentFocusDateValue();
}

function nextDateValue(baseValue) {
  const base = baseValue ? new Date(`${baseValue}T00:00:00`) : new Date();
  base.setDate(base.getDate() + 1);
  return `${base.getFullYear()}-${padNumber(base.getMonth() + 1)}-${padNumber(base.getDate())}`;
}

function nextDateTimeValue(baseValue) {
  const base = baseValue ? new Date(baseValue) : new Date();
  base.setMinutes(base.getMinutes() + 1, 0, 0);
  return `${base.getFullYear()}-${padNumber(base.getMonth() + 1)}-${padNumber(base.getDate())}T${padNumber(base.getHours())}:${padNumber(base.getMinutes())}`;
}

function syncFocusCounterFromDOM() {
  const count = document.querySelectorAll(".focus-list [data-task-card]").length;
  focusCounterElements().forEach((element) => {
    element.textContent = String(count);
  });
  updatePendingCounterState();
}

function syncArchiveCounterFromDOM() {
  const count = document.querySelectorAll("[data-archive-list] .archive-card").length;
  document.querySelectorAll("[data-archive-count]").forEach((element) => {
    element.textContent = String(count);
  });
}

function insertElementWithMotion(container, selector, element, method = "prepend") {
  const previousRects = captureRects(container, selector);
  stageEnter(element, false);

  if (method === "append") {
    container.appendChild(element);
  } else {
    container.prepend(element);
  }

  initializeTaskCards(container);
  animateMovedElements(container, selector, previousRects);
  window.requestAnimationFrame(() => {
    revealEnter(element, false);
  });

  return element;
}

function buildOptimisticCompletedCard(sourceElement, request) {
  const { label, kindClass } = extractCardKind(sourceElement);
  const postponePicker = sourceElement?.querySelector("[data-postpone-picker]");
  const postponeValue = sourceElement?.querySelector("[data-postpone-form] input[name='target_value']")?.value || "";
  return {
    id: request.taskID,
    title: extractCardText(sourceElement, "h3"),
    kind_label: label,
    kind_class: kindClass,
    finished_line: formatOptimisticCompletedLine(kindClass),
    status_line: extractCardText(sourceElement, ".status"),
    note: extractCardText(sourceElement, ".note"),
    can_postpone: Boolean(sourceElement?.querySelector("[data-postpone-panel]")),
    postpone_mode: postponePicker?.getAttribute("data-picker-mode") || "",
    postpone_value: postponeValue,
    postpone_min_value: postponePicker?.getAttribute("data-min-value") || postponeValue,
    return_date: extractReturnDate(sourceElement),
  };
}

function buildOptimisticFocusCard(sourceElement, request) {
  const { label, kindClass } = extractCardKind(sourceElement);
  const returnDate = extractReturnDate(sourceElement);
  const statusLine = sourceElement?.getAttribute("data-status-line") || "";
  const postponeMode = sourceElement?.getAttribute("data-postpone-mode") || (kindClass === "ddl" ? "datetime" : "date");
  const postponeValue = sourceElement?.getAttribute("data-postpone-value") || (postponeMode === "datetime" ? nextDateTimeValue() : nextDateValue(returnDate));
  const canPostpone = sourceElement?.getAttribute("data-can-postpone") === "1";
  const postponeMinValue = sourceElement?.getAttribute("data-postpone-min-value") || postponeValue;
  return {
    id: request.taskID,
    title: extractCardText(sourceElement, "h3"),
    kind_label: label,
    kind_class: kindClass,
    status_line: statusLine,
    note: extractCardText(sourceElement, ".note"),
    can_postpone: canPostpone,
    can_complete: true,
    postpone_mode: postponeMode,
    postpone_value: postponeValue,
    postpone_min_value: postponeMinValue,
    return_date: returnDate,
  };
}

function insertOptimisticArchiveCard(request) {
  const section = document.querySelector("[data-archive-section]");
  const sourceElement = actionTargetForRequest(request);
  if (!section || !sourceElement) {
    return null;
  }

  const cardData = buildOptimisticCompletedCard(sourceElement, request);
  let insertedElement = null;
  const applyInsert = () => {
    section.classList.remove("is-empty");
    const list = ensureArchiveList(section);
    const element = createElementFromHTML(buildCompletedTaskCardHTML(cardData));
    insertedElement = insertElementWithMotion(list, ".archive-card", element, "prepend");
    syncArchiveCounterFromDOM();
  };

  animateHeightMutation(section, applyInsert);
  return insertedElement;
}

function insertOptimisticFocusCard(request) {
  const panel = document.querySelector(".focus-panel");
  const sourceElement = actionTargetForRequest(request);
  if (!panel || !sourceElement) {
    return null;
  }

  const cardData = buildOptimisticFocusCard(sourceElement, request);
  const currentEmpty = panel.querySelector(".focus-empty");
  if (currentEmpty) {
    const overlay = currentEmpty.cloneNode(true);
    overlay.classList.add("focus-empty-overlay");
    panel.classList.add("has-empty-overlay");
    panel.appendChild(overlay);

    let insertedElement = null;
    const list = document.createElement("div");
    list.className = "focus-list";
    const applyInsert = () => {
      const element = createElementFromHTML(buildFocusTaskCardHTML(cardData));
      stageEnter(element, false);
      list.appendChild(element);
      insertedElement = element;
      currentEmpty.replaceWith(list);
      initializeTaskCards(panel);
      syncFocusCounterFromDOM();
      overlay.classList.add("is-leaving");
    };

    animateHeightMutation(panel, applyInsert);
    window.setTimeout(() => {
      if (insertedElement) {
        revealEnter(insertedElement, false);
      }
    }, Math.round(MOVE_ANIMATION_MS * 0.55));
    afterTransition(MOVE_ANIMATION_MS, () => {
      overlay.remove();
      panel.classList.remove("has-empty-overlay");
    });
    return insertedElement;
  }

  let insertedElement = null;
  const applyInsert = () => {
    const list = ensureFocusList(panel);
    const element = createElementFromHTML(buildFocusTaskCardHTML(cardData));
    insertedElement = insertElementWithMotion(list, "[data-task-card]", element, "prepend");
    syncFocusCounterFromDOM();
  };

  animateHeightMutation(panel, applyInsert);
  return insertedElement;
}

function buildFocusEmptyHTML(snapshot) {
  const quote = snapshot.empty_quote;
  if (quote?.text) {
    return `
      <div class="focus-empty">
        <div class="empty-quote-block">
          <p class="empty-quote">${escapeHtml(quote.text)}</p>
          ${quote.has_meta ? `
            <p class="empty-quote-meta">
              ${quote.author ? `<span class="empty-quote-author">${escapeHtml(quote.author)}</span>` : ""}
              ${quote.author && quote.source ? `<span class="empty-quote-sep">·</span>` : ""}
              ${quote.source ? `<span class="empty-quote-source">${escapeHtml(quote.source)}</span>` : ""}
            </p>
          ` : ""}
        </div>
      </div>
    `;
  }

  return `
    <div class="focus-empty">
      <p>这一天没有需要出现的任务。</p>
      <p class="note">如果只是想先安静一点，现在这个视图就是空白的。</p>
    </div>
  `;
}

function reconcileList(container, nextCards, options) {
  const {
    selector,
    buildHTML,
    updateElement,
    collapseEnteredSpace = false,
  } = options;

  const previousRects = captureRects(container, selector);
  const existingMap = new Map();
  container.querySelectorAll(selector).forEach((element) => {
    const taskID = element.getAttribute("data-task-id");
    if (taskID) {
      existingMap.set(taskID, element);
    }
  });

  const entered = [];
  const nextNodes = [];

  nextCards.forEach((card) => {
    let element = existingMap.get(card.id);
    if (!element) {
      element = createElementFromHTML(buildHTML(card));
      stageEnter(element, collapseEnteredSpace);
      entered.push(element);
    } else {
      updateElement(element, card);
      existingMap.delete(card.id);
    }
    nextNodes.push(element);
  });

  nextNodes.forEach((element) => {
    container.appendChild(element);
  });

  existingMap.forEach((element) => {
    element.remove();
  });

  initializeTaskCards(container);
  animateMovedElements(container, selector, previousRects);
  if (entered.length > 0) {
    if (collapseEnteredSpace) {
      entered.forEach((element) => expandEnterSpace(element));
    }
    window.setTimeout(() => {
      entered.forEach((element) => revealEnter(element, collapseEnteredSpace));
    }, MOVE_ANIMATION_MS);
  }
}

function updateFocusTaskElement(element, card) {
  const fresh = createElementFromHTML(buildFocusTaskCardHTML(card));
  element.replaceChildren(...fresh.childNodes);
  Array.from(fresh.attributes).forEach((attribute) => {
    element.setAttribute(attribute.name, attribute.value);
  });
}

function updateCompletedTaskElement(element, card) {
  const fresh = createElementFromHTML(buildCompletedTaskCardHTML(card));
  element.replaceChildren(...fresh.childNodes);
  Array.from(fresh.attributes).forEach((attribute) => {
    element.setAttribute(attribute.name, attribute.value);
  });
}

function ensureFocusList(panel) {
  let list = panel.querySelector(".focus-list");
  if (list) {
    return list;
  }

  const empty = panel.querySelector(".focus-empty");
  list = document.createElement("div");
  list.className = "focus-list";
  if (empty) {
    empty.replaceWith(list);
  } else {
    panel.appendChild(list);
  }
  return list;
}

function setFocusEmpty(panel, snapshot) {
  const list = panel.querySelector(".focus-list");
  const nextEmpty = createElementFromHTML(buildFocusEmptyHTML(snapshot));
  if (list) {
    list.replaceWith(nextEmpty);
  } else {
    const currentEmpty = panel.querySelector(".focus-empty");
    if (currentEmpty) {
      currentEmpty.replaceWith(nextEmpty);
    } else {
      panel.appendChild(nextEmpty);
    }
  }
}

function updateFocusPanel(snapshot) {
  const panel = document.querySelector(".focus-panel");
  if (!panel) {
    return;
  }

  const counter = panel.querySelector(".focus-counter");
  if (counter) {
    counter.textContent = String(snapshot.focus_tasks.length);
  }
  const currentList = panel.querySelector(".focus-list");
  const existingIDs = new Set();
  currentList?.querySelectorAll("[data-task-card]").forEach((element) => {
    const taskID = element.getAttribute("data-task-id");
    if (taskID) {
      existingIDs.add(taskID);
    }
  });
  const hasEnteredCards = snapshot.focus_tasks.some((card) => !existingIDs.has(card.id));

  const applyUpdate = () => {
    if (snapshot.focus_tasks.length === 0) {
      setFocusEmpty(panel, snapshot);
      return;
    }

    const list = ensureFocusList(panel);
    reconcileList(list, snapshot.focus_tasks, {
      selector: "[data-task-card]",
      buildHTML: buildFocusTaskCardHTML,
      updateElement: updateFocusTaskElement,
      collapseEnteredSpace: false,
    });
  };

  if (snapshot.focus_tasks.length === 0) {
    if (panel.querySelector(".focus-list")) {
      animateHeightMutation(panel, applyUpdate);
      return;
    }
    applyUpdate();
    return;
  }

  if (panel.querySelector(".focus-empty")) {
    animateHeightMutation(panel, applyUpdate);
    return;
  }

  if (currentList && hasEnteredCards) {
    const startHeight = panel.getBoundingClientRect().height;
    panel.style.height = `${startHeight}px`;
    panel.style.overflow = "hidden";
    panel.style.transition = "none";
    applyUpdate();
    const endHeight = panel.scrollHeight;

    if (Math.abs(endHeight - startHeight) < 0.5) {
      panel.style.height = "";
      panel.style.overflow = "";
      panel.style.transition = "";
      return;
    }

    panel.getBoundingClientRect();
    window.requestAnimationFrame(() => {
      panel.style.transition = `height ${MOVE_ANIMATION_MS}ms ${EASE}`;
      panel.style.height = `${endHeight}px`;
    });

    afterTransition(MOVE_ANIMATION_MS, () => {
      panel.style.height = "";
      panel.style.overflow = "";
      panel.style.transition = "";
    });
    return;
  }

  applyUpdate();
}

function ensureArchiveList(section) {
  let list = section.querySelector("[data-archive-list]");
  if (list) {
    return list;
  }

  list = document.createElement("div");
  list.className = "archive-list";
  list.setAttribute("data-archive-list", "");
  section.appendChild(list);
  return list;
}

function updateArchiveSection(snapshot) {
  const section = document.querySelector("[data-archive-section]");
  if (!section) {
    return;
  }

  const wasEmpty = section.classList.contains("is-empty");
  const count = section.querySelector("[data-archive-count]");
  if (count) {
    count.textContent = String(snapshot.completed_tasks.length);
  }

  if (snapshot.completed_tasks.length === 0) {
    if (wasEmpty) {
      const list = section.querySelector("[data-archive-list]");
      if (list) {
        list.innerHTML = "";
      }
      return;
    }

    const startHeight = section.getBoundingClientRect().height;
    const list = section.querySelector("[data-archive-list]");
    if (list) {
      list.innerHTML = "";
    }

    section.style.height = `${startHeight}px`;
    section.style.overflow = "hidden";
    section.style.transition = "none";
    section.getBoundingClientRect();
    window.requestAnimationFrame(() => {
      section.style.transition = [
        `height ${MOVE_ANIMATION_MS}ms ${EASE}`,
        `opacity ${ENTER_ANIMATION_MS}ms ${EASE}`,
        `padding-top ${MOVE_ANIMATION_MS}ms ${EASE}`,
        `border-top-width ${MOVE_ANIMATION_MS}ms ${EASE}`,
      ].join(", ");
      section.style.height = "0px";
      section.style.opacity = "0";
      section.style.paddingTop = "0px";
      section.style.borderTopWidth = "0px";
    });

    afterTransition(MOVE_ANIMATION_MS, () => {
      section.classList.add("is-empty");
      section.style.height = "";
      section.style.overflow = "";
      section.style.opacity = "";
      section.style.paddingTop = "";
      section.style.borderTopWidth = "";
      section.style.transition = "";
    });
    return;
  }

  if (wasEmpty) {
    section.classList.remove("is-empty");
    section.style.height = "0px";
    section.style.opacity = "0";
    section.style.paddingTop = "0px";
    section.style.borderTopWidth = "0px";
    section.style.overflow = "hidden";
    section.style.transition = "none";
  } else {
    section.style.height = `${section.getBoundingClientRect().height}px`;
    section.style.overflow = "hidden";
    section.style.transition = "none";
  }

  const list = ensureArchiveList(section);
  reconcileList(list, snapshot.completed_tasks, {
    selector: ".archive-card",
    buildHTML: buildCompletedTaskCardHTML,
    updateElement: updateCompletedTaskElement,
  });

  const targetHeight = section.scrollHeight;
  section.getBoundingClientRect();
  window.requestAnimationFrame(() => {
    section.style.transition = [
      `height ${MOVE_ANIMATION_MS}ms ${EASE}`,
      `opacity ${ENTER_ANIMATION_MS}ms ${EASE}`,
      `padding-top ${MOVE_ANIMATION_MS}ms ${EASE}`,
      `border-top-width ${MOVE_ANIMATION_MS}ms ${EASE}`,
    ].join(", ");
    section.style.height = `${targetHeight}px`;
    section.style.opacity = "1";
    section.style.paddingTop = "";
    section.style.borderTopWidth = "";
  });

  afterTransition(MOVE_ANIMATION_MS, () => {
    section.style.height = "";
    section.style.overflow = "";
    section.style.opacity = "";
    section.style.paddingTop = "";
    section.style.borderTopWidth = "";
    section.style.transition = "";
  });
}

function applyTaskSnapshot(snapshot) {
  updateFocusPanel(snapshot);
  updateArchiveSection(snapshot);
  updatePendingCounterState();
}

function enqueueTaskMutation(run) {
  const queued = taskMutationQueue.then(run, run);
  taskMutationQueue = queued.catch(() => {});
  return queued;
}

function buildTaskRequest(form) {
  return {
    action: form.action,
    formData: new FormData(form),
    taskID: taskIDForForm(form),
    isComplete: form.hasAttribute("data-complete-form"),
    isRestore: form.hasAttribute("data-restore-form"),
    isPostpone: form.hasAttribute("data-postpone-form"),
  };
}

function applyOptimisticTaskMutation(request) {
  if (request.isComplete) {
    request.optimistic = true;
    request.optimisticInsertedElement = insertOptimisticArchiveCard(request);
    request.exitHandle = detachForExit(actionTargetForRequest(request));
    syncFocusCounterFromDOM();
    return;
  }

  if (request.isRestore) {
    request.optimistic = true;
    request.optimisticInsertedElement = insertOptimisticFocusCard(request);
    request.exitHandle = detachForExit(actionTargetForRequest(request));
    syncArchiveCounterFromDOM();
  }
}

function fetchTaskRequestSnapshot(request) {
  return fetch(request.action, {
    method: "POST",
    body: request.formData,
    headers: {
      "X-Requested-With": "fetch",
    },
  }).then(async (response) => {
    if (!response.ok) {
      throw new Error("request failed");
    }
    return response.json();
  });
}

function actionTargetForRequest(request) {
  if (!request.taskID) {
    return null;
  }

  if (request.isRestore) {
    return document.querySelector(`.archive-card[data-task-id="${cssEscape(request.taskID)}"]`);
  }

  return document.querySelector(`[data-task-card][data-task-id="${cssEscape(request.taskID)}"]`);
}

function currentFormForRequest(request) {
  const target = actionTargetForRequest(request);
  if (!target) {
    return null;
  }

  if (request.isRestore) {
    return target.querySelector("[data-restore-form]");
  }
  if (request.isComplete) {
    return target.querySelector("[data-complete-form]");
  }
  if (request.isPostpone) {
    return target.querySelector("[data-postpone-form]");
  }
  return null;
}

function shouldAnimateTaskExitRequest(request, snapshot) {
  if (request.isComplete || request.isRestore) {
    return true;
  }

  if (request.isPostpone) {
    return !snapshot.focus_tasks.some((card) => card.id === request.taskID);
  }

  return false;
}

async function processTaskRequest(request, responsePromise) {
  let exitHandle = request.exitHandle || null;
  try {
    const snapshot = await responsePromise;
    if (request.optimistic) {
      if (exitHandle) {
        await exitHandle.done;
      }
      if (request.isComplete && snapshot.focus_tasks.length === 0) {
        updateFocusPanel(snapshot);
      }
      return snapshot;
    }

    if (!exitHandle && shouldAnimateTaskExitRequest(request, snapshot)) {
      exitHandle = detachForExit(actionTargetForRequest(request));
    }

    if (request.isRestore) {
      updateFocusPanel(snapshot);
      if (snapshot.completed_tasks.length > 0) {
        updateArchiveSection(snapshot);
        if (exitHandle) {
          await exitHandle.done;
        }
      } else if (exitHandle) {
        await exitHandle.done;
        updateArchiveSection(snapshot);
      } else {
        updateArchiveSection(snapshot);
      }
      return;
    }

    if (request.isComplete) {
      updateArchiveSection(snapshot);
      if (snapshot.focus_tasks.length > 0) {
        updateFocusPanel(snapshot);
        if (exitHandle) {
          await exitHandle.done;
        }
      } else if (exitHandle) {
        await exitHandle.done;
        updateFocusPanel(snapshot);
      } else {
        updateFocusPanel(snapshot);
      }
      return;
    }

    if (shouldAnimateTaskExitRequest(request, snapshot) && exitHandle) {
      if (snapshot.focus_tasks.length > 0) {
        updateFocusPanel(snapshot);
        await exitHandle.done;
      } else {
        await exitHandle.done;
        updateFocusPanel(snapshot);
      }
      return;
    }

    applyTaskSnapshot(snapshot);
    return snapshot;
  } catch (_error) {
    if (request.optimistic) {
      window.location.reload();
      return null;
    }

    if (exitHandle) {
      exitHandle.restore();
    } else {
      resetInlineAnimation(actionTargetForRequest(request));
    }

    const currentForm = currentFormForRequest(request);
    if (currentForm) {
      currentForm.submit();
      return;
    }

    window.location.reload();
    return null;
  }
}

function bindAsyncTaskForm(form) {
  if (form.dataset.bound === "1") {
    return;
  }
  form.dataset.bound = "1";

  form.addEventListener("submit", async (event) => {
    if (form.dataset.submitting === "1") {
      return;
    }

    event.preventDefault();
    form.dataset.submitting = "1";

    const submitButton = form.querySelector("button");
    if (submitButton) {
      submitButton.disabled = true;
    }

    const isTaskForm = form.hasAttribute("data-async-task-form");
    if (isTaskForm) {
      const request = buildTaskRequest(form);
      if (request.isComplete || request.isRestore) {
        applyOptimisticTaskMutation(request);
      }

      const needsResync = !(request.isComplete || request.isRestore);
      beginPendingTaskRequest({
        needsResync,
      });
      const responsePromise = fetchTaskRequestSnapshot(request);
      enqueueTaskMutation(() => processTaskRequest(request, responsePromise))
        .finally(async () => {
          await endPendingTaskRequest();
          form.dataset.submitting = "0";
          if (submitButton && submitButton.isConnected) {
            submitButton.disabled = false;
          }
        });
      return;
    }

    const preservedState = window.captureFocusPageState
      ? window.captureFocusPageState(document)
      : null;
    beginPendingTaskRequest();

    try {
      const response = await fetch(form.action, {
        method: "POST",
        body: new FormData(form),
        headers: {
          "X-Requested-With": "fetch",
        },
      });

      if (!response.ok) {
        throw new Error("request failed");
      }

      if (response.redirected && window.loadFocusPage) {
        const nextURL = new URL(response.url);
        await window.loadFocusPage(nextURL.pathname + nextURL.search, "replace", {
          state: preservedState,
        });
        return;
      }

      if (window.reloadFocusPage) {
        await window.reloadFocusPage({
          state: preservedState,
        });
        return;
      }

      window.location.reload();
    } catch (_error) {
      await endPendingTaskRequest();
      form.dataset.submitting = "0";
      if (submitButton) {
        submitButton.disabled = false;
      }
      form.submit();
    }
  });
}

function initializeTaskCards(root = document) {
  if (window.initializePostponePickers) {
    window.initializePostponePickers(root);
  }
  updatePendingCounterState();
  root.querySelectorAll("[data-async-task-form], [data-async-focus-form]").forEach((form) => {
    bindAsyncTaskForm(form);
  });
}

window.initializeTaskCards = initializeTaskCards;
document.addEventListener("DOMContentLoaded", () => initializeTaskCards(document));
