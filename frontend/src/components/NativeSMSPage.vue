<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { APIError, fetchNativeSMSData, importNativeSMSMessages, importNativeSMSPaste } from "../api/client";
import type { NativeSMSMessage, NativeSMSPageData } from "../types";

type SMSMode = "new" | "history";
type StatusKind = "" | "info" | "success" | "error";

interface NativeSMSStatus {
  kind: StatusKind;
  text: string;
}

interface SmsBridgePlugin {
  readPickupMessages: () => Promise<{ ok?: boolean; reason?: string; messages?: unknown[] }>;
}

const HISTORY_KEY_PREFIX = "todo-native-sms-history-v1";
const CURRENT_CACHE_KEY_PREFIX = "todo-native-sms-current-v1";
const MAX_VISIBLE_MESSAGES = 100;
const MAX_CURRENT_CACHE_MESSAGES = 300;
const THREE_MONTHS_MS = 1000 * 60 * 60 * 24 * 90;

const pageData = ref<NativeSMSPageData | null>(null);
const mode = ref<SMSMode>("new");
const currentMessages = ref<NativeSMSMessage[]>([]);
const historyMessages = ref<NativeSMSMessage[]>([]);
const selectedIDs = ref<Set<string>>(new Set());
const pending = ref(false);
const status = ref<NativeSMSStatus>({ kind: "", text: "" });
const pasteOpen = ref(false);
const pasteInput = ref("");
const pasteInputRef = ref<HTMLTextAreaElement | null>(null);
let statusTimer = 0;
let refreshTimer = 0;
let loadSeq = 0;

const activeMessages = computed(() => (mode.value === "history" ? historyMessages.value : currentList()));
const selectedCount = computed(() => activeMessages.value.filter((message) => selectedIDs.value.has(message.id)).length);
const countLabel = computed(() => (mode.value === "history" ? `历史记录 ${activeMessages.value.length} 条` : `新短信 ${activeMessages.value.length} 条`));
const selectionLabel = computed(() => (selectedCount.value > 0 ? `已选 ${selectedCount.value} 条` : "未选择"));
const historyButtonLabel = computed(() => (mode.value === "history" ? "返回新短信" : "历史记录"));

watch(pasteOpen, async (open) => {
  if (!open) {
    return;
  }
  await nextTick();
  pasteInputRef.value?.focus();
});

function plugin(): SmsBridgePlugin | null {
  const candidate = (window as unknown as { Capacitor?: { Plugins?: { SmsBridge?: SmsBridgePlugin } } }).Capacitor?.Plugins?.SmsBridge;
  return candidate || null;
}

function isNativeAvailable() {
  return Boolean(plugin());
}

function storageKey() {
  return `${HISTORY_KEY_PREFIX}:${pageData.value?.user_id || "anonymous"}`;
}

function currentCacheKey() {
  return `${CURRENT_CACHE_KEY_PREFIX}:${pageData.value?.user_id || "anonymous"}`;
}

function normalizeMessage(message: unknown): NativeSMSMessage | null {
  if (!message || typeof message !== "object") {
    return null;
  }
  const source = message as Record<string, unknown>;
  const body = String(source.body || "").trim();
  if (!body) {
    return null;
  }
  const id = String(source.id || "").trim();
  const dateValue = Number(source.date || 0);
  return {
    id,
    address: String(source.address || "短信").trim() || "短信",
    body,
    date: Number.isFinite(dateValue) ? dateValue : 0,
  };
}

function trimMessages(entries: unknown[], limit: number) {
  const cutoff = Date.now() - THREE_MONTHS_MS;
  const normalized: NativeSMSMessage[] = [];
  const seen = new Set<string>();

  entries.forEach((entry) => {
    const item = normalizeMessage(entry);
    if (!item || !item.id || item.date < cutoff || seen.has(item.id)) {
      return;
    }
    seen.add(item.id);
    normalized.push(item);
  });

  normalized.sort((left, right) => right.date - left.date);
  return normalized.slice(0, limit);
}

function loadHistory() {
  try {
    const raw = window.localStorage.getItem(storageKey());
    return raw ? trimMessages(JSON.parse(raw), MAX_VISIBLE_MESSAGES) : [];
  } catch (_error) {
    return [];
  }
}

function saveHistory(entries: NativeSMSMessage[]) {
  const trimmed = trimMessages(entries, MAX_VISIBLE_MESSAGES);
  window.localStorage.setItem(storageKey(), JSON.stringify(trimmed));
  historyMessages.value = trimmed;
}

function loadCurrentCache() {
  try {
    const raw = window.localStorage.getItem(currentCacheKey());
    return raw ? trimMessages(JSON.parse(raw), MAX_CURRENT_CACHE_MESSAGES) : [];
  } catch (_error) {
    return [];
  }
}

function saveCurrentCache(entries: NativeSMSMessage[]) {
  window.localStorage.setItem(currentCacheKey(), JSON.stringify(trimMessages(entries, MAX_CURRENT_CACHE_MESSAGES)));
}

function currentList() {
  const handledIDs = new Set(historyMessages.value.map((message) => message.id));
  return currentMessages.value.filter((message) => !handledIDs.has(message.id)).slice(0, MAX_VISIBLE_MESSAGES);
}

function setStatus(kind: StatusKind, text: string) {
  if (statusTimer) {
    window.clearTimeout(statusTimer);
    statusTimer = 0;
  }
  status.value = { kind, text };
  if (kind === "success" || kind === "error") {
    statusTimer = window.setTimeout(() => {
      status.value = { kind: "", text: "" };
    }, kind === "success" ? 2600 : 4200);
  }
}

function formatDateTime(timestamp: number) {
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

function toggleSelection(id: string) {
  const next = new Set(selectedIDs.value);
  if (next.has(id)) {
    next.delete(id);
  } else {
    next.add(id);
  }
  selectedIDs.value = next;
}

function pruneSelection() {
  const validIDs = new Set(activeMessages.value.map((message) => message.id));
  const next = new Set<string>();
  selectedIDs.value.forEach((id) => {
    if (validIDs.has(id)) {
      next.add(id);
    }
  });
  selectedIDs.value = next;
}

async function copyToClipboard(text: string) {
  const value = text.trim();
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

function applyCurrentMessages(messages: unknown[]) {
  currentMessages.value = trimMessages(messages, MAX_CURRENT_CACHE_MESSAGES);
  saveCurrentCache(currentMessages.value);
}

async function loadMessages(options: { silent?: boolean } = {}) {
  if (!isNativeAvailable()) {
    setStatus("error", "当前不是 Android 壳环境，无法读取本地短信。");
    pruneSelection();
    return;
  }

  const seq = ++loadSeq;
  if (!options.silent) {
    setStatus("info", "正在读取本地短信…");
  }

  try {
    const result = await plugin()?.readPickupMessages();
    if (seq !== loadSeq) {
      return;
    }
    if (!result || result.ok === false) {
      const reason = result?.reason === "permission_denied" ? "请允许短信权限后再试。" : "短信读取失败。";
      const cached = loadCurrentCache();
      if (cached.length > 0) {
        currentMessages.value = cached;
        historyMessages.value = loadHistory();
        setStatus("error", `${reason} 已显示最近一次缓存。`);
      } else {
        setStatus("error", reason);
      }
      pruneSelection();
      return;
    }

    applyCurrentMessages(Array.isArray(result.messages) ? result.messages : []);
    historyMessages.value = loadHistory();
    setStatus("", "");
    pruneSelection();
  } catch (_error) {
    if (seq !== loadSeq) {
      return;
    }
    const cached = loadCurrentCache();
    if (cached.length > 0) {
      currentMessages.value = cached;
      historyMessages.value = loadHistory();
      setStatus("error", "短信读取失败，已显示最近一次缓存。");
    } else {
      setStatus("error", "短信读取失败。");
    }
    pruneSelection();
  }
}

function scheduleRefresh() {
  if (pending.value) {
    return;
  }
  if (refreshTimer) {
    window.clearTimeout(refreshTimer);
  }
  refreshTimer = window.setTimeout(() => {
    refreshTimer = 0;
    void loadMessages({ silent: true });
  }, 320);
}

function persistAcceptedMessages(messages: NativeSMSMessage[]) {
  saveHistory(messages.concat(loadHistory()));
}

function clearActionFocus() {
  if (document.activeElement instanceof HTMLElement) {
    document.activeElement.blur();
  }
}

async function submitSelection() {
  if (pending.value) {
    return;
  }

  const visibleMessages = activeMessages.value;
  const messages = visibleMessages.filter((message) => selectedIDs.value.has(message.id));
  const archivedUnchecked = mode.value === "new" ? visibleMessages.filter((message) => !selectedIDs.value.has(message.id)) : [];

  if (mode.value === "new" && messages.length === 0) {
    if (archivedUnchecked.length > 0) {
      persistAcceptedMessages(archivedUnchecked);
      historyMessages.value = loadHistory();
    }
    selectedIDs.value = new Set();
    setStatus("success", archivedUnchecked.length > 0 ? `已归档 ${archivedUnchecked.length} 条短信。` : "当前没有可归档的新短信。");
    clearActionFocus();
    return;
  }

  if (messages.length === 0) {
    clearActionFocus();
    return;
  }

  pending.value = true;
  setStatus("info", "正在提交短信…");

  try {
    const payload = await importNativeSMSMessages(messages.map((message) => ({ id: message.id, body: message.body })));
    const acceptedIDs = new Set(Array.isArray(payload.accepted_ids) ? payload.accepted_ids : []);
    const unsupportedIDs = new Set(Array.isArray(payload.unsupported_ids) ? payload.unsupported_ids : []);
    const acceptedMessages = messages.filter((message) => acceptedIDs.has(message.id));
    const unsupportedMessages = messages.filter((message) => unsupportedIDs.has(message.id));
    const historyEntries = acceptedMessages.concat(archivedUnchecked);

    if (historyEntries.length > 0) {
      persistAcceptedMessages(historyEntries);
    }

    selectedIDs.value = new Set();
    historyMessages.value = loadHistory();

    if (unsupportedMessages.length > 0) {
      await copyToClipboard(unsupportedMessages.map((message) => message.body).join("\n\n"));
      setStatus("error", "有短信暂时无法识别，已保留在新短信里，并复制到剪贴板，请发给管理员处理。");
    } else if (acceptedMessages.length > 0) {
      setStatus("success", `已导入 ${acceptedMessages.length} 条短信。`);
    } else {
      await copyToClipboard(messages.map((message) => message.body).join("\n\n"));
      setStatus("error", "没有识别到可导入的快递短信，已复制到剪贴板，请发给管理员处理。");
    }
  } catch (error) {
    if (archivedUnchecked.length > 0) {
      persistAcceptedMessages(archivedUnchecked);
      historyMessages.value = loadHistory();
    }
    setStatus("error", error instanceof Error ? error.message : "短信提交失败。");
  } finally {
    pending.value = false;
    pruneSelection();
    clearActionFocus();
  }
}

async function submitPastedSMS() {
  const input = pasteInput.value.trim();
  if (!input) {
    setStatus("error", "短信内容不能为空");
    return;
  }
  if (pending.value) {
    return;
  }
  pending.value = true;
  setStatus("info", "正在解析短信…");
  try {
    const payload = await importNativeSMSPaste(input);
    pasteInput.value = "";
    pasteOpen.value = false;
    setStatus("success", payload.created_count > 0 ? `已导入 ${payload.created_count} 条短信。` : "没有识别到可导入的快递短信。");
  } catch (error) {
    setStatus("error", error instanceof Error ? error.message : "短信导入失败。");
  } finally {
    pending.value = false;
    clearActionFocus();
  }
}

function toggleHistory() {
  mode.value = mode.value === "history" ? "new" : "history";
  selectedIDs.value = new Set();
  setStatus("", "");
  clearActionFocus();
}

async function initialize() {
  try {
    pageData.value = await fetchNativeSMSData();
    historyMessages.value = loadHistory();
    currentMessages.value = loadCurrentCache();
    await loadMessages();
  } catch (error) {
    if (error instanceof APIError && error.status === 401) {
      setStatus("error", "当前浏览器没有可用登录态。先在 Go 站点登录，再回到这里刷新。");
      return;
    }
    setStatus("error", "短信导入页初始化失败。");
  }
}

function onVisibilityChange() {
  if (!document.hidden) {
    scheduleRefresh();
  }
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === "Escape" && pasteOpen.value) {
    pasteOpen.value = false;
  }
  if ((event.metaKey || event.ctrlKey) && event.key === "Enter" && pasteOpen.value) {
    event.preventDefault();
    void submitPastedSMS();
  }
}

onMounted(() => {
  void initialize();
  document.addEventListener("visibilitychange", onVisibilityChange);
  window.addEventListener("focus", scheduleRefresh);
  window.addEventListener("pageshow", scheduleRefresh);
  window.addEventListener("keydown", onKeydown);
});

onBeforeUnmount(() => {
  if (statusTimer) {
    window.clearTimeout(statusTimer);
  }
  if (refreshTimer) {
    window.clearTimeout(refreshTimer);
  }
  document.removeEventListener("visibilitychange", onVisibilityChange);
  window.removeEventListener("focus", scheduleRefresh);
  window.removeEventListener("pageshow", scheduleRefresh);
  window.removeEventListener("keydown", onKeydown);
});
</script>

<template>
  <main class="native-sms-page">
    <section class="native-sms-panel">
      <header class="native-sms-header">
        <div class="native-sms-header-copy">
          <h1>短信导入</h1>
          <p class="native-sms-copy">手动勾选快递短信后导入。新短信只显示近 3 个月，最多 100 条。</p>
        </div>

        <div class="native-sms-toolbar">
          <button type="button" class="secondary" @click="toggleHistory">{{ historyButtonLabel }}</button>
          <button type="button" :disabled="pending" @click="submitSelection">确定</button>
          <a :href="pageData?.return_path || '/'" class="native-sms-return">返回</a>
        </div>
      </header>

      <div v-if="status.text" class="native-sms-status" :class="`is-${status.kind}`">{{ status.text }}</div>

      <div class="native-sms-meta">
        <span>{{ countLabel }}</span>
        <span>{{ selectionLabel }}</span>
      </div>

      <div class="native-sms-list">
        <article v-for="message in activeMessages" :key="message.id" class="native-sms-item"
          :class="{ 'is-selected': selectedIDs.has(message.id) }" tabindex="0" role="button"
          :aria-pressed="selectedIDs.has(message.id)" @click="toggleSelection(message.id)"
          @keydown.enter.prevent="toggleSelection(message.id)" @keydown.space.prevent="toggleSelection(message.id)">
          <div class="native-sms-item-main">
            <div class="native-sms-item-head">
              <p class="native-sms-item-sender">{{ message.address || "短信" }}</p>
              <time class="native-sms-item-time">{{ formatDateTime(message.date) }}</time>
            </div>
            <p class="native-sms-item-body">{{ message.body }}</p>
          </div>
        </article>

        <div v-if="activeMessages.length === 0" class="native-sms-empty">
          <p>{{ mode === "history" ? "最近没有历史记录。" : "最近没有新的短信。" }}</p>
          <button v-if="mode === 'new'" type="button" class="secondary native-sms-empty-action"
            @click="pasteOpen = true">粘贴短信</button>
        </div>
      </div>
    </section>

    <div v-if="pasteOpen" class="native-sms-modal-shell is-open">
      <div class="native-sms-modal-backdrop" @click="pasteOpen = false"></div>
      <section class="native-sms-modal" role="dialog" aria-modal="true" aria-labelledby="native-paste-title">
        <div class="native-sms-modal-head">
          <h2 id="native-paste-title">粘贴短信</h2>
        </div>
        <label class="native-sms-paste-field" for="native-paste-input">
          <span>短信内容</span>
          <textarea id="native-paste-input" ref="pasteInputRef" v-model="pasteInput" rows="8"
            placeholder="直接粘贴短信内容；一次贴很多条也可以。"></textarea>
        </label>
        <div class="native-sms-modal-actions">
          <button type="button" class="secondary" @click="pasteOpen = false">取消</button>
          <button type="button" :disabled="pending" @click="submitPastedSMS">导入短信</button>
        </div>
      </section>
    </div>
  </main>
</template>
