<script setup lang="ts">
import { onMounted, ref } from "vue";
import { APIError, fetchAccountData, fetchUserPreferences, updateUserPreferences } from "../api/client";
import type { AccountPageData } from "../types";
import { getAndroidShellPlugin, isAndroidShell } from "../utils/androidShell";

const state = ref<"loading" | "ready" | "unauthorized" | "error">("loading");
const account = ref<AccountPageData | null>(null);
const errorMessage = ref("");
const updateMessage = ref("");
const updateLoading = ref(false);
const androidUpdateAvailable = ref(false);
const widgetDualColumn = ref(true);
const widgetPreferenceLoading = ref(false);
const widgetPreferenceMessage = ref("");

async function loadSettings() {
  errorMessage.value = "";
  try {
    account.value = await fetchAccountData("");
    state.value = "ready";
  } catch (error) {
    if (error instanceof APIError && error.status === 401) {
      state.value = "unauthorized";
      errorMessage.value = "当前浏览器没有可用登录态。";
      return;
    }
    state.value = "error";
    errorMessage.value = error instanceof Error ? error.message : "设置页加载失败";
  }
}

onMounted(() => {
  void loadSettings();
  void initializeAndroidSettings();
});

async function initializeAndroidSettings() {
  const plugin = getAndroidShellPlugin();
  if (!plugin) {
    androidUpdateAvailable.value = false;
    return;
  }
  androidUpdateAvailable.value = true;
  try {
    await plugin.status?.();
  } catch (_error) {
    androidUpdateAvailable.value = false;
    return;
  }
  try {
    const preferences = await fetchUserPreferences();
    widgetDualColumn.value = preferences.widget_dual_column;
  } catch (error) {
    widgetPreferenceMessage.value = error instanceof Error ? error.message : "小组件设置加载失败";
  }
}

async function checkAndroidUpdate() {
  const plugin = getAndroidShellPlugin();
  if (!plugin || updateLoading.value) {
    return;
  }
  updateLoading.value = true;
  updateMessage.value = "";
  try {
    const check = plugin.check;
    if (!check) {
      return;
    }
    const result = await check({ manual: true });
    updateMessage.value = typeof result?.message === "string" ? result.message : "";
  } catch (error) {
    updateMessage.value = error instanceof Error ? error.message : "检查更新失败";
  } finally {
    updateLoading.value = false;
  }
}

async function saveWidgetDualColumn() {
  if (!isAndroidShell() || widgetPreferenceLoading.value) {
    return;
  }
  const previous = widgetDualColumn.value;
  const next = !previous;
  widgetDualColumn.value = next;
  widgetPreferenceLoading.value = true;
  widgetPreferenceMessage.value = "";
  try {
    const preferences = await updateUserPreferences({ widget_dual_column: next });
    widgetDualColumn.value = preferences.widget_dual_column;
    await getAndroidShellPlugin()?.refreshWidgets?.();
    widgetPreferenceMessage.value = "已同步到云端";
  } catch (error) {
    widgetDualColumn.value = previous;
    widgetPreferenceMessage.value = error instanceof Error ? error.message : "设置保存失败";
  } finally {
    widgetPreferenceLoading.value = false;
  }
}
</script>

<template>
  <main class="manage-shell settings-shell">
    <header class="manage-top">
      <a class="manage-user" href="/me">
        <span class="eyebrow">设置</span>
        <strong>{{ account?.current_user.display_name || "Todo" }}</strong>
      </a>
      <div class="manage-top-actions">
        <a class="soft-button compact" href="/">返回首页</a>
        <a class="soft-button compact" href="/me">返回菜单</a>
      </div>
    </header>

    <p v-if="errorMessage" class="inline-error">{{ errorMessage }}</p>
    <p v-if="state === 'unauthorized'" class="inline-error">请先登录</p>

    <section class="settings-empty-panel">
      <span class="eyebrow">设置</span>
      <h1>偏好</h1>
      <p>后续的偏好、开关和接口配置会放在这里。</p>
    </section>

    <section v-if="androidUpdateAvailable" class="settings-empty-panel settings-action-panel">
      <span class="eyebrow">安卓壳</span>
      <h1>应用更新</h1>
      <p>检查安卓壳新版本，下载完成后会自动打开系统安装界面。</p>
      <button class="primary-button" type="button" :disabled="updateLoading" @click="checkAndroidUpdate">
        {{ updateLoading ? "正在检查" : "检查更新" }}
      </button>
      <p v-if="updateMessage" class="settings-action-message">{{ updateMessage }}</p>
    </section>

    <section v-if="androidUpdateAvailable" class="settings-empty-panel settings-action-panel">
      <span class="eyebrow">桌面小组件</span>
      <div class="settings-toggle-row">
        <span>
          <strong>双列显示</strong>
          <small>关闭后，小组件任务会始终按单列向下排列。</small>
        </span>
        <button type="button" class="settings-switch" :class="{ active: widgetDualColumn }"
          :aria-pressed="widgetDualColumn" :disabled="widgetPreferenceLoading" @click="saveWidgetDualColumn">
          <span></span>
        </button>
      </div>
      <p v-if="widgetPreferenceMessage" class="settings-action-message">{{ widgetPreferenceMessage }}</p>
    </section>
  </main>
</template>
