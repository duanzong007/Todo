const EXIT_ANIMATION_MS = 320;
const MOVE_ANIMATION_MS = 320;
const ENTER_ANIMATION_MS = 320;
const EASE = "cubic-bezier(0.22, 0.61, 0.36, 1)";
let taskMutationQueue = Promise.resolve();

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
        ${card.can_postpone ? `
          <details class="inline-postpone" data-postpone-panel data-task-id="${escapeHtml(card.id)}">
            <summary>延期</summary>
            <form action="/tasks/${escapeHtml(card.id)}/postpone" method="post" data-postpone-form data-async-task-form>
              <input type="hidden" name="return_date" value="${escapeHtml(card.return_date)}">
              <input type="date" name="target_date" value="${escapeHtml(card.postpone_value)}">
              <button type="submit" class="secondary">确认</button>
            </form>
          </details>
        ` : ""}
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
    <article class="archive-card" data-task-id="${escapeHtml(card.id)}">
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

async function processTaskRequest(request) {
  const immediateExit = request.isComplete || request.isRestore;
  let exitHandle = immediateExit ? detachForExit(actionTargetForRequest(request)) : null;

  try {
    const response = await fetch(request.action, {
      method: "POST",
      body: request.formData,
      headers: {
        "X-Requested-With": "fetch",
      },
    });

    if (!response.ok) {
      throw new Error("request failed");
    }

    const snapshot = await response.json();
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
  } catch (_error) {
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
      enqueueTaskMutation(() => processTaskRequest(request))
        .finally(() => {
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
      form.dataset.submitting = "0";
      if (submitButton) {
        submitButton.disabled = false;
      }
      form.submit();
    }
  });
}

function initializeTaskCards(root = document) {
  root.querySelectorAll("[data-async-task-form], [data-async-focus-form]").forEach((form) => {
    bindAsyncTaskForm(form);
  });
}

window.initializeTaskCards = initializeTaskCards;
document.addEventListener("DOMContentLoaded", () => initializeTaskCards(document));
