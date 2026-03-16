function cssEscape(value) {
  if (window.CSS && typeof window.CSS.escape === "function") {
    return window.CSS.escape(value);
  }
  return String(value).replace(/["\\]/g, "\\$&");
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
  if (window.initializeTaskCards) {
    window.initializeTaskCards(root);
  }
  initializeFocusNavigation(root);
}

function collectTaskIDs(root, selector) {
  const ids = new Set();
  root.querySelectorAll(selector).forEach((element) => {
    const taskID = element.getAttribute("data-task-id");
    if (taskID) {
      ids.add(taskID);
    }
  });
  return ids;
}

function prepareEnterAnimation(element) {
  const rect = element.getBoundingClientRect();
  if (rect.height <= 0) {
    return;
  }

  const styles = window.getComputedStyle(element);
  element.dataset.enterHeight = `${rect.height}`;
  element.dataset.enterPaddingTop = styles.paddingTop;
  element.dataset.enterPaddingBottom = styles.paddingBottom;
  element.dataset.enterBorderTopWidth = styles.borderTopWidth;
  element.dataset.enterBorderBottomWidth = styles.borderBottomWidth;

  element.style.height = "0px";
  element.style.overflow = "hidden";
  element.style.opacity = "0";
  element.style.transform = "translateY(8px) scale(0.985)";
  element.style.filter = "blur(0.8px)";
  element.style.paddingTop = "0px";
  element.style.paddingBottom = "0px";
  element.style.borderTopWidth = "0px";
  element.style.borderBottomWidth = "0px";
}

function playEnterAnimations(root, selector, previousIDs) {
  const entering = [];
  root.querySelectorAll(selector).forEach((element) => {
    const taskID = element.getAttribute("data-task-id");
    if (!taskID || previousIDs.has(taskID)) {
      return;
    }
    prepareEnterAnimation(element);
    entering.push(element);
  });

  if (entering.length === 0) {
    return;
  }

  window.requestAnimationFrame(() => {
    entering.forEach((element) => {
      element.style.height = element.dataset.enterHeight || "";
      element.style.opacity = "1";
      element.style.transform = "translateY(0) scale(1)";
      element.style.filter = "blur(0)";
      element.style.paddingTop = element.dataset.enterPaddingTop || "";
      element.style.paddingBottom = element.dataset.enterPaddingBottom || "";
      element.style.borderTopWidth = element.dataset.enterBorderTopWidth || "";
      element.style.borderBottomWidth = element.dataset.enterBorderBottomWidth || "";

      const cleanup = () => {
        element.style.height = "";
        element.style.overflow = "";
        element.style.opacity = "";
        element.style.transform = "";
        element.style.filter = "";
        element.style.paddingTop = "";
        element.style.paddingBottom = "";
        element.style.borderTopWidth = "";
        element.style.borderBottomWidth = "";
        delete element.dataset.enterHeight;
        delete element.dataset.enterPaddingTop;
        delete element.dataset.enterPaddingBottom;
        delete element.dataset.enterBorderTopWidth;
        delete element.dataset.enterBorderBottomWidth;
      };

      element.addEventListener("transitionend", cleanup, { once: true });
    });
  });
}

function patchFocusPage(currentMain, nextMain) {
  const currentPanel = currentMain.querySelector(".focus-panel");
  const nextPanel = nextMain.querySelector(".focus-panel");
  const currentDrawers = currentMain.querySelector(".focus-drawers");
  const nextDrawers = nextMain.querySelector(".focus-drawers");
  const currentHero = currentMain.querySelector(".focus-hero");
  const nextHero = nextMain.querySelector(".focus-hero");

  if (!currentPanel || !nextPanel || !currentDrawers || !nextDrawers || !currentHero || !nextHero) {
    return false;
  }

  const previousFocusIDs = collectTaskIDs(currentPanel, "[data-task-card]");
  const previousArchiveIDs = collectTaskIDs(currentDrawers, ".archive-card");

  currentHero.replaceWith(nextHero);
  currentPanel.replaceWith(nextPanel);
  currentDrawers.replaceWith(nextDrawers);
  playEnterAnimations(nextPanel, "[data-task-card]", previousFocusIDs);
  playEnterAnimations(nextDrawers, ".archive-card", previousArchiveIDs);
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
  const state = options.state || captureFocusPageState(document);
  const mode = options.mode || "full";
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
      if (!patchFocusPage(currentMain, nextMain)) {
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
  root.querySelectorAll("[data-focus-nav-link]").forEach((link) => {
    if (link.dataset.bound === "1") {
      return;
    }
    link.dataset.bound = "1";

    link.addEventListener("click", (event) => {
      event.preventDefault();
      loadFocusPage(link.href, "push");
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

      loadFocusPage(url.pathname + url.search, "push");
    });
  });
}

window.loadFocusPage = loadFocusPage;
window.reloadFocusPage = reloadFocusPage;
window.initializeFocusNavigation = initializeFocusNavigation;
window.captureFocusPageState = captureFocusPageState;

document.addEventListener("DOMContentLoaded", () => {
  hydrateFocusPage(document);
});

window.addEventListener("popstate", () => {
  loadFocusPage(window.location.pathname + window.location.search, "replace");
});
