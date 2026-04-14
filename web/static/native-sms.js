(() => {
  const HISTORY_KEY_PREFIX = "todo-native-sms-history-v1";
  const CURRENT_CACHE_KEY_PREFIX = "todo-native-sms-current-v1";
  const MAX_VISIBLE_MESSAGES = 100;
  const MAX_CURRENT_CACHE_MESSAGES = 300;
  const THREE_MONTHS_MS = 1000 * 60 * 60 * 24 * 90;

  const state = {
    mode: "new",
    currentMessages: [],
    historyMessages: [],
    selectedIDs: new Set(),
    pending: false,
    statusTimer: 0,
    refreshTimer: 0,
    loadSeq: 0,
  };

  function root() {
    return document.querySelector("[data-native-sms-page]");
  }

  function plugin() {
    return window.Capacitor?.Plugins?.SmsBridge || null;
  }

  function isNativeAvailable() {
    return !!plugin();
  }

  function storageKey() {
    const userID = root()?.getAttribute("data-user-id") || "anonymous";
    return `${HISTORY_KEY_PREFIX}:${userID}`;
  }

  function currentCacheKey() {
    const userID = root()?.getAttribute("data-user-id") || "anonymous";
    return `${CURRENT_CACHE_KEY_PREFIX}:${userID}`;
  }

  function trimMessages(entries, limit) {
    const cutoff = Date.now() - THREE_MONTHS_MS;
    const normalized = [];
    const seen = new Set();

    entries.forEach((entry) => {
      const item = normalizeMessage(entry);
      if (!item || !item.id || item.date < cutoff) {
        return;
      }
      if (seen.has(item.id)) {
        return;
      }
      seen.add(item.id);
      normalized.push(item);
    });

    normalized.sort((left, right) => right.date - left.date);
    return normalized.slice(0, limit);
  }

  function trimHistory(entries) {
    return trimMessages(entries, MAX_VISIBLE_MESSAGES);
  }

  function loadHistory() {
    try {
      const raw = window.localStorage.getItem(storageKey());
      if (!raw) {
        return [];
      }
      return trimHistory(JSON.parse(raw));
    } catch (_error) {
      return [];
    }
  }

  function saveHistory(entries) {
    const trimmed = trimHistory(entries);
    window.localStorage.setItem(storageKey(), JSON.stringify(trimmed));
    state.historyMessages = trimmed;
  }

  function loadCurrentCache() {
    try {
      const raw = window.localStorage.getItem(currentCacheKey());
      if (!raw) {
        return [];
      }
      return trimMessages(JSON.parse(raw), MAX_CURRENT_CACHE_MESSAGES);
    } catch (_error) {
      return [];
    }
  }

  function saveCurrentCache(entries) {
    const trimmed = trimMessages(entries, MAX_CURRENT_CACHE_MESSAGES);
    window.localStorage.setItem(currentCacheKey(), JSON.stringify(trimmed));
  }

  function normalizeMessage(message) {
    if (!message || typeof message !== "object") {
      return null;
    }
    const body = String(message.body || "").trim();
    if (!body) {
      return null;
    }
    const id = String(message.id || "").trim();
    const dateValue = Number(message.date || 0);
    return {
      id,
      address: String(message.address || "短信").trim() || "短信",
      body,
      date: Number.isFinite(dateValue) ? dateValue : 0,
    };
  }

  function currentList() {
    const handledIDs = new Set(state.historyMessages.map((message) => message.id));
    return state.currentMessages
      .filter((message) => !handledIDs.has(message.id))
      .slice(0, MAX_VISIBLE_MESSAGES);
  }

  function activeMessages() {
    return state.mode === "history" ? state.historyMessages : currentList();
  }

  function setStatus(kind, text) {
    const node = document.querySelector("[data-native-sms-status]");
    if (!node) {
      return;
    }

    if (state.statusTimer) {
      window.clearTimeout(state.statusTimer);
      state.statusTimer = 0;
    }

    if (!text) {
      node.hidden = true;
      node.textContent = "";
      node.className = "native-sms-status";
      return;
    }
    node.hidden = false;
    node.textContent = text;
    node.className = `native-sms-status is-${kind}`;

    if (kind === "success" || kind === "error") {
      state.statusTimer = window.setTimeout(() => {
        setStatus("", "");
      }, kind === "success" ? 2600 : 4200);
    }
  }

  function formatDateTime(timestamp) {
    if (!timestamp) {
      return "";
    }
    return new Intl.DateTimeFormat("zh-CN", {
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    }).format(new Date(timestamp));
  }

  function updateMeta() {
    const countLabel = document.querySelector("[data-native-count-label]");
    const selectionLabel = document.querySelector("[data-native-selection-label]");
    const historyToggle = document.querySelector("[data-native-history-toggle]");
    const confirmButton = document.querySelector("[data-native-confirm]");

    const messages = activeMessages();
    const selectedCount = Array.from(state.selectedIDs).filter((id) => messages.some((message) => message.id === id)).length;

    if (countLabel) {
      countLabel.textContent = state.mode === "history" ? `历史记录 ${messages.length} 条` : `新短信 ${messages.length} 条`;
    }
    if (selectionLabel) {
      selectionLabel.textContent = selectedCount > 0 ? `已选 ${selectedCount} 条` : "未选择";
    }
    if (historyToggle) {
      historyToggle.textContent = state.mode === "history" ? "返回新短信" : "历史记录";
    }
    if (confirmButton) {
      confirmButton.disabled = state.pending;
    }
  }

  function clearActionFocus() {
    if (document.activeElement instanceof HTMLElement) {
      document.activeElement.blur();
    }
  }

  function toggleSelection(id) {
    if (!id) {
      return false;
    }
    if (state.selectedIDs.has(id)) {
      state.selectedIDs.delete(id);
      return false;
    }
    state.selectedIDs.add(id);
    return true;
  }

  function renderList() {
    const list = document.querySelector("[data-native-sms-list]");
    const template = document.querySelector("[data-native-sms-item-template]");
    if (!list || !(template instanceof HTMLTemplateElement)) {
      return;
    }

    const messages = activeMessages();
    const validIDs = new Set(messages.map((message) => message.id));
    state.selectedIDs.forEach((id) => {
      if (!validIDs.has(id)) {
        state.selectedIDs.delete(id);
      }
    });

    list.innerHTML = "";
    if (messages.length === 0) {
      const empty = document.createElement("div");
      empty.className = "native-sms-empty";
      empty.textContent = state.mode === "history" ? "最近没有历史记录。" : "最近没有新的短信。";
      list.appendChild(empty);
      updateMeta();
      return;
    }

    messages.forEach((message) => {
      const fragment = template.content.cloneNode(true);
      const item = fragment.querySelector("[data-native-sms-item]");
      const sender = fragment.querySelector("[data-native-sms-sender]");
      const time = fragment.querySelector("[data-native-sms-time]");
      const body = fragment.querySelector("[data-native-sms-body]");

      if (item) {
        const syncItemState = (selected) => {
          item.classList.toggle("is-selected", selected);
          item.setAttribute("aria-pressed", selected ? "true" : "false");
        };
        syncItemState(state.selectedIDs.has(message.id));
        item.addEventListener("click", () => {
          const selected = toggleSelection(message.id);
          syncItemState(selected);
          updateMeta();
        });
        item.addEventListener("keydown", (event) => {
          if (event.key !== "Enter" && event.key !== " ") {
            return;
          }
          event.preventDefault();
          const selected = toggleSelection(message.id);
          syncItemState(selected);
          updateMeta();
        });
      }
      if (sender) {
        sender.textContent = message.address || "短信";
      }
      if (time) {
        time.textContent = formatDateTime(message.date);
      }
      if (body) {
        body.textContent = message.body;
      }

      list.appendChild(fragment);
    });

    updateMeta();
  }

  async function copyToClipboard(text) {
    const value = String(text || "").trim();
    if (!value) {
      return false;
    }

    try {
      await navigator.clipboard.writeText(value);
      return true;
    } catch (_error) {
      const textarea = document.createElement("textarea");
      textarea.value = value;
      textarea.setAttribute("readonly", "readonly");
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      let copied = false;
      try {
        copied = document.execCommand("copy");
      } catch (_ignored) {
        copied = false;
      }
      textarea.remove();
      return copied;
    }
  }

  function applyCurrentMessages(messages) {
    state.currentMessages = trimMessages(messages, MAX_CURRENT_CACHE_MESSAGES);
    saveCurrentCache(state.currentMessages);
  }

  function scheduleRefresh() {
    if (state.pending) {
      return;
    }
    if (state.refreshTimer) {
      window.clearTimeout(state.refreshTimer);
    }
    state.refreshTimer = window.setTimeout(() => {
      state.refreshTimer = 0;
      loadMessages({ silent: true });
    }, 320);
  }

  async function loadMessages(options = {}) {
    const { silent = false } = options;
    if (!isNativeAvailable()) {
      setStatus("error", "当前不是 Android 壳环境，无法读取本地短信。");
      renderList();
      return;
    }

    const seq = ++state.loadSeq;
    if (!silent) {
      setStatus("info", "正在读取本地短信…");
    }
    try {
      const result = await plugin().readPickupMessages();
      if (seq !== state.loadSeq) {
        return;
      }
      if (!result || result.ok === false) {
        const reason = result?.reason === "permission_denied" ? "请允许短信权限后再试。" : "短信读取失败。";
        const cached = loadCurrentCache();
        if (cached.length > 0) {
          state.currentMessages = cached;
          state.historyMessages = loadHistory();
          setStatus("error", `${reason} 已显示最近一次缓存。`);
          renderList();
          return;
        }
        setStatus("error", reason);
        renderList();
        return;
      }

      applyCurrentMessages(Array.isArray(result.messages) ? result.messages : []);
      state.historyMessages = loadHistory();
      setStatus("", "");
      renderList();
    } catch (_error) {
      if (seq !== state.loadSeq) {
        return;
      }
      const cached = loadCurrentCache();
      if (cached.length > 0) {
        state.currentMessages = cached;
        state.historyMessages = loadHistory();
        setStatus("error", "短信读取失败，已显示最近一次缓存。");
        renderList();
        return;
      }
      setStatus("error", "短信读取失败。");
      renderList();
    }
  }

  function persistAcceptedMessages(messages) {
    const existing = loadHistory();
    saveHistory(messages.concat(existing));
  }

  async function submitSelection() {
    if (state.pending) {
      return;
    }

    const visibleMessages = activeMessages();
    const messages = visibleMessages.filter((message) => state.selectedIDs.has(message.id));
    const archivedUnchecked = state.mode === "new"
      ? visibleMessages.filter((message) => !state.selectedIDs.has(message.id))
      : [];

    if (state.mode === "new" && messages.length === 0) {
      if (archivedUnchecked.length > 0) {
        persistAcceptedMessages(archivedUnchecked);
        state.historyMessages = loadHistory();
      }
      state.selectedIDs.clear();
      setStatus("success", archivedUnchecked.length > 0 ? `已归档 ${archivedUnchecked.length} 条短信。` : "当前没有可归档的新短信。");
      renderList();
      clearActionFocus();
      return;
    }

    if (messages.length === 0) {
      updateMeta();
      clearActionFocus();
      return;
    }

    state.pending = true;
    updateMeta();
    setStatus("info", "正在提交短信…");

    try {
      const response = await fetch("/tasks/parse-sms/native", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Requested-With": "fetch",
        },
        body: JSON.stringify({
          messages: messages.map((message) => ({
            id: message.id,
            body: message.body,
          })),
        }),
      });

      const payload = await response.json().catch(() => ({}));
      if (!response.ok) {
        setStatus("error", payload.error || "短信提交失败。");
        return;
      }

      const acceptedIDs = new Set(Array.isArray(payload.accepted_ids) ? payload.accepted_ids : []);
      const unsupportedIDs = new Set(Array.isArray(payload.unsupported_ids) ? payload.unsupported_ids : []);
      const acceptedMessages = messages.filter((message) => acceptedIDs.has(message.id));
      const unsupportedMessages = messages.filter((message) => unsupportedIDs.has(message.id));

      const historyEntries = acceptedMessages.concat(archivedUnchecked);
      if (historyEntries.length > 0) {
        persistAcceptedMessages(historyEntries);
      }

      state.selectedIDs.clear();
      state.historyMessages = loadHistory();

      if (unsupportedMessages.length > 0) {
        await copyToClipboard(unsupportedMessages.map((message) => message.body).join("\n\n"));
        setStatus("error", "有短信暂时无法识别，已保留在新短信里，并复制到剪贴板，请发给管理员处理。");
      } else if (acceptedMessages.length > 0) {
        setStatus("success", `已导入 ${acceptedMessages.length} 条短信。`);
      } else {
        setStatus("error", "没有识别到可导入的快递短信，已复制到剪贴板，请发给管理员处理。");
        await copyToClipboard(messages.map((message) => message.body).join("\n\n"));
      }

      renderList();
    } catch (_error) {
      if (archivedUnchecked.length > 0) {
        persistAcceptedMessages(archivedUnchecked);
        state.historyMessages = loadHistory();
        renderList();
      }
      setStatus("error", "短信提交失败。");
    } finally {
      state.pending = false;
      updateMeta();
      clearActionFocus();
    }
  }

  function bindEvents() {
    const historyToggle = document.querySelector("[data-native-history-toggle]");
    const confirmButton = document.querySelector("[data-native-confirm]");

    if (historyToggle) {
      historyToggle.addEventListener("click", (event) => {
        if (event.currentTarget instanceof HTMLElement) {
          event.currentTarget.blur();
        }
        state.mode = state.mode === "history" ? "new" : "history";
        state.selectedIDs.clear();
        setStatus("", "");
        renderList();
        clearActionFocus();
      });
    }

    if (confirmButton) {
      confirmButton.addEventListener("click", (event) => {
        if (event.currentTarget instanceof HTMLElement) {
          event.currentTarget.blur();
        }
        submitSelection();
      });
    }

    document.addEventListener("visibilitychange", () => {
      if (!document.hidden) {
        scheduleRefresh();
      }
    });

    window.addEventListener("focus", scheduleRefresh);
    window.addEventListener("pageshow", scheduleRefresh);
  }

  function initializeNativeSMSPage() {
    if (!root()) {
      return;
    }

    state.historyMessages = loadHistory();
    state.currentMessages = loadCurrentCache();
    bindEvents();
    renderList();
    loadMessages();
  }

  document.addEventListener("DOMContentLoaded", initializeNativeSMSPage);
})();
