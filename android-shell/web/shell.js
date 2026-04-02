(() => {
  const STORAGE_KEY = "todo-android-shell-config";
  const DEFAULT_REMOTE_URL = "";
  const FRAME_READY_TIMEOUT_MS = 18000;

  const state = {
    remoteUrl: "",
    frameReady: false,
    loadTimer: 0,
  };

  function shellRoot() {
    return document.querySelector(".shell-page");
  }

  function normalizeUrl(rawValue) {
    const trimmed = String(rawValue || "").trim();
    if (!trimmed) {
      return "";
    }

    try {
      const url = new URL(trimmed);
      return url.toString().replace(/\/+$/, "");
    } catch (_error) {
      return "";
    }
  }

  function readConfig() {
    try {
      const raw = window.localStorage.getItem(STORAGE_KEY);
      if (!raw) {
        return { remoteUrl: DEFAULT_REMOTE_URL };
      }
      const parsed = JSON.parse(raw);
      return {
        remoteUrl: normalizeUrl(parsed.remoteUrl) || DEFAULT_REMOTE_URL,
      };
    } catch (_error) {
      return { remoteUrl: DEFAULT_REMOTE_URL };
    }
  }

  function writeConfig(nextConfig) {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(nextConfig));
  }

  function statusDot() {
    return document.querySelector("[data-shell-status-dot]");
  }

  function setStatus(kind, text, detail = "") {
    const dot = statusDot();
    const textNode = document.querySelector("[data-shell-status-text]");
    const urlNode = document.querySelector("[data-shell-current-url]");
    const toolbarText = document.querySelector("[data-shell-toolbar-status]");

    if (dot) {
      dot.classList.toggle("is-ready", kind === "ready");
      dot.classList.toggle("is-error", kind === "error");
    }
    if (textNode) {
      textNode.textContent = text;
    }
    if (toolbarText) {
      toolbarText.textContent = text;
    }
    if (urlNode) {
      urlNode.textContent = detail || state.remoteUrl || "未设置服务地址";
    }
  }

  function showSettings(shouldOpen) {
    const modal = document.querySelector("[data-shell-settings]");
    if (!modal) {
      return;
    }
    modal.hidden = !shouldOpen;
  }

  function setLoading(active, text = "正在载入远端 Todo…") {
    const loading = document.querySelector("[data-shell-loading]");
    const loadingText = document.querySelector("[data-shell-loading-text]");
    if (loadingText) {
      loadingText.textContent = text;
    }
    if (loading) {
      loading.classList.toggle("is-hidden", !active);
    }
  }

  function frameElement() {
    return document.querySelector("[data-shell-frame]");
  }

  function clearLoadTimer() {
    if (state.loadTimer) {
      window.clearTimeout(state.loadTimer);
      state.loadTimer = 0;
    }
  }

  async function getSmsCapability() {
    const capacitor = window.Capacitor;
    const plugin = capacitor?.Plugins?.SmsBridge;
    if (!plugin || typeof plugin.status !== "function") {
      return {
        available: false,
        source: "stub",
      };
    }

    try {
      return await plugin.status();
    } catch (_error) {
      return {
        available: false,
        source: "error",
      };
    }
  }

  async function postCapabilitiesToFrame(targetWindow) {
    const capability = await getSmsCapability();
    targetWindow.postMessage({
      type: "todo-native:capabilities",
      payload: {
        platform: "android-shell",
        nativeApp: true,
        sms: capability,
      },
    }, "*");
  }

  function connectFrame() {
    const frame = frameElement();
    if (!frame) {
      return;
    }

    clearLoadTimer();
    state.frameReady = false;
    frame.classList.remove("is-ready");

    if (!state.remoteUrl) {
      setStatus("error", "尚未配置服务地址", "请先设置 Todo 服务地址");
      setLoading(false);
      showSettings(true);
      return;
    }

    setStatus("loading", "正在连接", state.remoteUrl);
    setLoading(true, "正在载入远端 Todo…");
    frame.src = state.remoteUrl;

    state.loadTimer = window.setTimeout(() => {
      if (state.frameReady) {
        return;
      }
      setStatus("error", "连接较慢", state.remoteUrl);
      setLoading(true, "连接时间较长，可能是当前网络或服务地址不可达。");
    }, FRAME_READY_TIMEOUT_MS);
  }

  function onFrameReady() {
    const frame = frameElement();
    if (!frame) {
      return;
    }

    clearLoadTimer();
    state.frameReady = true;
    frame.classList.add("is-ready");
    setLoading(false);
    setStatus("ready", "已连接", state.remoteUrl);

    if (frame.contentWindow) {
      postCapabilitiesToFrame(frame.contentWindow);
    }
  }

  function bindSettings() {
    const input = document.querySelector("[data-shell-url-input]");
    const form = document.querySelector("[data-shell-settings-form]");
    const resetButton = document.querySelector("[data-shell-reset-url]");

    if (input) {
      input.value = state.remoteUrl;
    }

    document.querySelectorAll("[data-shell-open-settings]").forEach((button) => {
      button.addEventListener("click", () => {
        if (input) {
          input.value = state.remoteUrl;
        }
        showSettings(true);
      });
    });

    document.querySelectorAll("[data-shell-close-settings]").forEach((button) => {
      button.addEventListener("click", () => {
        showSettings(false);
      });
    });

    if (resetButton) {
      resetButton.addEventListener("click", () => {
        if (input) {
          input.value = DEFAULT_REMOTE_URL;
        }
      });
    }

    if (form) {
      form.addEventListener("submit", (event) => {
        event.preventDefault();
        if (!(input instanceof HTMLInputElement)) {
          return;
        }
        const nextUrl = normalizeUrl(input.value);
        if (!nextUrl) {
          input.focus();
          return;
        }
        state.remoteUrl = nextUrl;
        writeConfig({ remoteUrl: nextUrl });
        showSettings(false);
        connectFrame();
      });
    }
  }

  function bindToolbar() {
    document.querySelectorAll("[data-shell-reload]").forEach((button) => {
      button.addEventListener("click", () => {
        connectFrame();
      });
    });
  }

  function bindFrame() {
    const frame = frameElement();
    if (!frame) {
      return;
    }

    frame.addEventListener("load", () => {
      onFrameReady();
    });

    window.addEventListener("message", async (event) => {
      const payload = event.data;
      if (!payload || typeof payload !== "object") {
        return;
      }

      if (payload.type === "todo-native:ping" && event.source && typeof event.source.postMessage === "function") {
        await postCapabilitiesToFrame(event.source);
        return;
      }

      if (payload.type === "todo-native:open-settings") {
        showSettings(true);
      }
    });
  }

  function exposeBridge() {
    window.TodoNativeBridge = {
      isNativeApp() {
        return !!window.Capacitor;
      },
      async getCapabilities() {
        return {
          platform: "android-shell",
          nativeApp: !!window.Capacitor,
          sms: await getSmsCapability(),
        };
      },
      async requestSmsSync() {
        const plugin = window.Capacitor?.Plugins?.SmsBridge;
        if (!plugin || typeof plugin.readPickupMessages !== "function") {
          return {
            ok: false,
            reason: "unavailable",
          };
        }
        return plugin.readPickupMessages();
      },
    };
  }

  function initializeShell() {
    if (!shellRoot()) {
      return;
    }

    const config = readConfig();
    state.remoteUrl = config.remoteUrl;

    exposeBridge();
    bindSettings();
    bindToolbar();
    bindFrame();
    connectFrame();
  }

  document.addEventListener("DOMContentLoaded", initializeShell);
})();
