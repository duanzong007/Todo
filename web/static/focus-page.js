function cssEscape(value) {
  if (window.CSS && typeof window.CSS.escape === "function") {
    return window.CSS.escape(value);
  }
  return String(value).replace(/["\\]/g, "\\$&");
}

const PAGE_SWAP_OUT_MS = 120;
const PAGE_SWAP_IN_MS = 240;
const PAGE_SWAP_EASE = "cubic-bezier(0.22, 0.61, 0.36, 1)";
const USE_PATCH_NAVIGATION = false;
const DRAWER_TOGGLE_MS = 260;
const TODAY_REFRESH_GUARD_KEY = "todo-today-view-refresh";

function focusPageRoot() {
  return document.querySelector("main.focus-page");
}

function explicitFocusDateFromURL() {
  const url = new URL(window.location.href);
  const explicitDate = (url.searchParams.get("date") || "").trim();
  if (explicitDate) {
    return explicitDate;
  }

  const year = (url.searchParams.get("year") || "").trim();
  const month = (url.searchParams.get("month") || "").trim();
  const day = (url.searchParams.get("day") || "").trim();
  if (!year && !month && !day) {
    return "";
  }

  const normalizedYear = year.padStart(4, "0");
  const normalizedMonth = month.padStart(2, "0");
  const normalizedDay = day.padStart(2, "0");
  if (!/^\d{4}$/.test(normalizedYear) || !/^\d{2}$/.test(normalizedMonth) || !/^\d{2}$/.test(normalizedDay)) {
    return "";
  }

  return `${normalizedYear}-${normalizedMonth}-${normalizedDay}`;
}

function renderedFocusDate() {
  const root = focusPageRoot();
  const fromDataset = (root?.dataset.focusDate || "").trim();
  if (fromDataset) {
    return fromDataset;
  }

  const input = document.querySelector(".composer-panel input[name='return_date']");
  return (input?.value || "").trim();
}

function appTimeZone() {
  const root = focusPageRoot();
  return (root?.dataset.appTimezone || "").trim() || Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
}

function formatDateParts(parts) {
  const year = parts.find((part) => part.type === "year")?.value || "";
  const month = parts.find((part) => part.type === "month")?.value || "";
  const day = parts.find((part) => part.type === "day")?.value || "";
  if (!year || !month || !day) {
    return "";
  }
  return `${year}-${month}-${day}`;
}

function todayISOInAppTimeZone() {
  try {
    const formatter = new Intl.DateTimeFormat("en-CA", {
      timeZone: appTimeZone(),
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    });
    if (typeof formatter.formatToParts === "function") {
      return formatDateParts(formatter.formatToParts(new Date()));
    }
    const fallback = formatter.format(new Date());
    const normalized = fallback.replaceAll("/", "-");
    if (/^\d{4}-\d{2}-\d{2}$/.test(normalized)) {
      return normalized;
    }
  } catch (_error) {
    // Fall through to local date fallback.
  }

  const now = new Date();
  const year = String(now.getFullYear());
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const day = String(now.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function focusDateForSync() {
  const explicitFocusDate = explicitFocusDateFromURL();
  if (explicitFocusDate) {
    return explicitFocusDate;
  }
  return todayISOInAppTimeZone();
}

function resetTodayRefreshGuard() {
  try {
    window.sessionStorage.removeItem(TODAY_REFRESH_GUARD_KEY);
  } catch (_error) {
    // Ignore session storage failures.
  }
}

function shouldRefreshImplicitTodayView() {
  if (explicitFocusDateFromURL()) {
    resetTodayRefreshGuard();
    return false;
  }

  const rendered = renderedFocusDate();
  const today = todayISOInAppTimeZone();
  if (!rendered || rendered === today) {
    resetTodayRefreshGuard();
    return false;
  }

  return true;
}

function ensureImplicitTodayViewIsFresh() {
  if (!shouldRefreshImplicitTodayView()) {
    return false;
  }

  const today = todayISOInAppTimeZone();
  try {
    if (window.sessionStorage.getItem(TODAY_REFRESH_GUARD_KEY) === today) {
      return false;
    }
    window.sessionStorage.setItem(TODAY_REFRESH_GUARD_KEY, today);
  } catch (_error) {
    // Ignore session storage failures.
  }

  window.location.reload();
  return true;
}

function captureFocusPageState(root = document) {
  const openDetails = {};
  root.querySelectorAll("[data-preserve-open]").forEach((element) => {
    const key = element.getAttribute("data-preserve-open");
    if (!key) {
      return;
    }
    openDetails[key] = element.hasAttribute("open");
  });

  const openPostponeTaskIDs = [];
  root.querySelectorAll("[data-postpone-panel][open]").forEach((element) => {
    const taskID = element.getAttribute("data-task-id");
    if (taskID) {
      openPostponeTaskIDs.push(taskID);
    }
  });

  return {
    openDetails,
    openPostponeTaskIDs,
  };
}

function restoreFocusPageState(root, state) {
  if (!root || !state) {
    return;
  }

  root.querySelectorAll("[data-preserve-open]").forEach((element) => {
    const key = element.getAttribute("data-preserve-open");
    if (!key) {
      return;
    }

    if (state.openDetails[key]) {
      element.setAttribute("open", "");
    } else {
      element.removeAttribute("open");
    }
  });

  (state.openPostponeTaskIDs || []).forEach((taskID) => {
    const panel = root.querySelector(`[data-postpone-panel][data-task-id="${cssEscape(taskID)}"]`);
    if (panel) {
      panel.setAttribute("open", "");
    }
  });
}

function hydrateFocusPage(root = document) {
  if (window.initializeDatePickers) {
    window.initializeDatePickers(root);
  }
  if (window.initializeComposerPanels) {
    window.initializeComposerPanels(root);
  }
  if (window.initializeTaskCards) {
    window.initializeTaskCards(root);
  }
  initializeAnimatedDrawers(root);
  initializeFocusNavigation(root);
  initializeFocusRefresh(root);
}

function syncDrawerState(details) {
  const summary = details.querySelector("summary");
  if (!summary) {
    return;
  }
  summary.setAttribute("aria-expanded", details.hasAttribute("open") ? "true" : "false");
}

function finishDrawerAnimation(details, keepOpen) {
  if (!keepOpen) {
    details.removeAttribute("open");
  }
  details.classList.remove("is-opening", "is-opening-active", "is-closing");
  details.style.height = "";
  details.style.overflow = "";
  details.style.transition = "";
  details.dataset.animating = "0";
  syncDrawerState(details);
}

function bindAnimatedDrawer(details) {
  if (details.dataset.drawerBound === "1") {
    syncDrawerState(details);
    return;
  }
  details.dataset.drawerBound = "1";
  details.dataset.animating = "0";
  syncDrawerState(details);

  const summary = details.querySelector("summary");
  if (!summary) {
    return;
  }

  summary.addEventListener("click", (event) => {
    event.preventDefault();
    if (details.dataset.animating === "1") {
      return;
    }

    const summaryHeight = summary.getBoundingClientRect().height;
    const isOpen = details.hasAttribute("open");
    details.dataset.animating = "1";

    if (isOpen) {
      const startHeight = details.getBoundingClientRect().height;
      details.classList.add("is-closing");
      details.style.height = `${startHeight}px`;
      details.style.overflow = "hidden";
      details.style.transition = "none";
      details.getBoundingClientRect();

      window.requestAnimationFrame(() => {
        details.style.transition = `height ${DRAWER_TOGGLE_MS}ms ${PAGE_SWAP_EASE}`;
        details.style.height = `${summaryHeight}px`;
      });

      window.setTimeout(() => {
        finishDrawerAnimation(details, false);
      }, DRAWER_TOGGLE_MS);
      return;
    }

    details.setAttribute("open", "");
    syncDrawerState(details);
    const endHeight = details.getBoundingClientRect().height;
    details.classList.add("is-opening");
    details.style.height = `${summaryHeight}px`;
    details.style.overflow = "hidden";
    details.style.transition = "none";
    details.getBoundingClientRect();

    window.requestAnimationFrame(() => {
      details.style.transition = `height ${DRAWER_TOGGLE_MS}ms ${PAGE_SWAP_EASE}`;
      details.style.height = `${endHeight}px`;
      details.classList.add("is-opening-active");
    });

    window.setTimeout(() => {
      finishDrawerAnimation(details, true);
    }, DRAWER_TOGGLE_MS);
  });
}

function initializeAnimatedDrawers(root = document) {
  root.querySelectorAll("[data-animated-drawer]").forEach((details) => {
    bindAnimatedDrawer(details);
  });
}

function animatePageSwapOut(sections) {
  sections.forEach((section) => {
    if (!section) {
      return;
    }
    section.classList.add("is-page-exit");
  });

  return new Promise((resolve) => {
    window.setTimeout(resolve, PAGE_SWAP_OUT_MS);
  });
}

function replaceSectionChildren(currentSection, nextSection) {
  currentSection.replaceChildren(...Array.from(nextSection.childNodes).map((node) => node.cloneNode(true)));
}

async function transitionSectionContent(currentSection, nextSection) {
  if (!currentSection || !nextSection) {
    return false;
  }

  const startHeight = currentSection.getBoundingClientRect().height;
  currentSection.style.height = `${startHeight}px`;
  currentSection.style.overflow = "hidden";
  currentSection.style.transition = "none";
  currentSection.classList.add("is-page-exit");

  await new Promise((resolve) => {
    window.setTimeout(resolve, PAGE_SWAP_OUT_MS);
  });

  currentSection.classList.remove("is-page-exit");
  currentSection.classList.add("is-page-enter");
  replaceSectionChildren(currentSection, nextSection);

  const endHeight = currentSection.scrollHeight;
  currentSection.getBoundingClientRect();

  window.requestAnimationFrame(() => {
    currentSection.style.transition = `height ${PAGE_SWAP_IN_MS}ms ${PAGE_SWAP_EASE}`;
    currentSection.style.height = `${endHeight}px`;
    currentSection.classList.add("is-page-enter-active");
  });

  await new Promise((resolve) => {
    window.setTimeout(resolve, PAGE_SWAP_IN_MS);
  });

  currentSection.classList.remove("is-page-enter", "is-page-enter-active");
  currentSection.style.height = "";
  currentSection.style.overflow = "";
  currentSection.style.transition = "";
  return true;
}

async function patchFocusPage(currentMain, nextMain) {
  const currentPanel = currentMain.querySelector(".focus-panel");
  const nextPanel = nextMain.querySelector(".focus-panel");
  const currentDrawers = currentMain.querySelector(".focus-drawers");
  const nextDrawers = nextMain.querySelector(".focus-drawers");
  const currentHero = currentMain.querySelector(".focus-hero");
  const nextHero = nextMain.querySelector(".focus-hero");

  if (!currentPanel || !nextPanel || !currentDrawers || !nextDrawers || !currentHero || !nextHero) {
    return false;
  }

  const startHeight = currentMain.getBoundingClientRect().height;
  currentMain.style.minHeight = `${startHeight}px`;

  await Promise.all([
    transitionSectionContent(currentHero, nextHero),
    transitionSectionContent(currentPanel, nextPanel),
    transitionSectionContent(currentDrawers, nextDrawers),
  ]);

  currentMain.style.minHeight = "";
  return true;
}

function setFocusPageLoading(loading) {
  const main = document.querySelector("main.focus-page");
  if (!main) {
    return;
  }

  main.classList.toggle("is-loading", loading);
  main.querySelectorAll(".focus-panel, .focus-drawers").forEach((section) => {
    section.classList.toggle("is-reloading", loading);
  });
}

async function loadFocusPage(url, historyMode = "push", options = {}) {
  if (!USE_PATCH_NAVIGATION) {
    if (historyMode === "replace") {
      window.location.replace(url);
      return;
    }
    window.location.assign(url);
    return;
  }

  const state = options.state || captureFocusPageState(document);
  const mode = options.mode || "patch";
  setFocusPageLoading(true);

  try {
    const response = await fetch(url, {
      headers: {
        "X-Requested-With": "fetch",
      },
    });
    const html = await response.text();
    const doc = new DOMParser().parseFromString(html, "text/html");
    const nextMain = doc.querySelector("main.focus-page");
    const currentMain = document.querySelector("main.focus-page");

    if (!nextMain || !currentMain) {
      window.location.assign(url);
      return;
    }

    restoreFocusPageState(nextMain, state);

    if (mode === "patch") {
      if (!await patchFocusPage(currentMain, nextMain)) {
        currentMain.replaceWith(nextMain);
      }
    } else {
      currentMain.replaceWith(nextMain);
    }
    document.title = doc.title;

    if (historyMode === "push") {
      window.history.pushState({}, "", url);
    } else if (historyMode === "replace") {
      window.history.replaceState({}, "", url);
    }

    hydrateFocusPage(document);
  } catch (_error) {
    window.location.assign(url);
  } finally {
    setFocusPageLoading(false);
  }
}

function reloadFocusPage(options = {}) {
  return loadFocusPage(window.location.pathname + window.location.search, "replace", {
    mode: "patch",
    ...options,
  });
}

function initializeFocusNavigation(root = document) {
  if (!USE_PATCH_NAVIGATION) {
    return;
  }

  root.querySelectorAll("[data-focus-nav-link]").forEach((link) => {
    if (link.dataset.bound === "1") {
      return;
    }
    link.dataset.bound = "1";

    link.addEventListener("click", (event) => {
      event.preventDefault();
      loadFocusPage(link.href, "push", {
        mode: "patch",
      });
    });
  });

  root.querySelectorAll("[data-focus-nav-form]").forEach((form) => {
    if (form.dataset.bound === "1") {
      return;
    }
    form.dataset.bound = "1";

    form.addEventListener("submit", (event) => {
      event.preventDefault();

      const action = form.getAttribute("action") || window.location.pathname;
      const url = new URL(action, window.location.origin);
      const formData = new FormData(form);
      for (const [key, value] of formData.entries()) {
        if (String(value).trim() !== "") {
          url.searchParams.set(key, value);
        }
      }

      loadFocusPage(url.pathname + url.search, "push", {
        mode: "patch",
      });
    });
  });
}

function initializeFocusRefresh(root = document) {
  root.querySelectorAll("[data-focus-refresh]").forEach((button) => {
    if (button.dataset.bound === "1") {
      return;
    }
    button.dataset.bound = "1";

    button.addEventListener("click", async () => {
      if (button.disabled) {
        return;
      }

      button.disabled = true;
      try {
        if (window.reloadFocusPage) {
          await window.reloadFocusPage();
          return;
        }
        window.location.reload();
      } finally {
        window.setTimeout(() => {
          button.disabled = false;
        }, 320);
      }
    });
  });
}

window.loadFocusPage = loadFocusPage;
window.reloadFocusPage = reloadFocusPage;
window.initializeFocusNavigation = initializeFocusNavigation;
window.captureFocusPageState = captureFocusPageState;
window.getFocusDateForSync = focusDateForSync;
window.ensureImplicitTodayViewIsFresh = ensureImplicitTodayViewIsFresh;

document.addEventListener("DOMContentLoaded", () => {
  if (ensureImplicitTodayViewIsFresh()) {
    return;
  }
  hydrateFocusPage(document);
});

document.addEventListener("visibilitychange", () => {
  if (document.visibilityState !== "visible") {
    return;
  }
  ensureImplicitTodayViewIsFresh();
});

window.addEventListener("pageshow", () => {
  ensureImplicitTodayViewIsFresh();
});

window.addEventListener("focus", () => {
  ensureImplicitTodayViewIsFresh();
});

window.addEventListener("popstate", () => {
  if (!USE_PATCH_NAVIGATION) {
    return;
  }

  loadFocusPage(window.location.pathname + window.location.search, "replace", {
    mode: "patch",
  });
});
