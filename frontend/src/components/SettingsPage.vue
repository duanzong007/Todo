<script setup lang="ts">
import { onMounted, ref } from "vue";
import { APIError, fetchAccountData, fetchUserPreferences, updateUserPreferences } from "../api/client";
import type { AccountPageData } from "../types";
import { getAndroidShellPlugin, isAndroidShell } from "../utils/androidShell";
import SettingItem from "./SettingItem.vue";

const state = ref<"loading" | "ready" | "unauthorized" | "error">("loading");
const account = ref<AccountPageData | null>(null);
const errorMessage = ref("");
const updateLoading = ref(false);
const androidUpdateAvailable = ref(false);
const widgetDualColumn = ref(true);
const widgetPreferenceLoading = ref(false);
const widgetPreferenceError = ref("");

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
    widgetPreferenceError.value = error instanceof Error ? error.message : "小组件设置加载失败";
  }
}

async function checkAndroidUpdate() {
  const plugin = getAndroidShellPlugin();
  if (!plugin || updateLoading.value) {
    return;
  }
  updateLoading.value = true;
  try {
    const check = plugin.check;
    if (!check) {
      return;
    }
    await check({ manual: true });
  } catch (_error) {
    // Native update dialogs already report errors.
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
  widgetPreferenceError.value = "";
  try {
    const preferences = await updateUserPreferences({ widget_dual_column: next });
    widgetDualColumn.value = preferences.widget_dual_column;
    await getAndroidShellPlugin()?.refreshWidgets?.();
  } catch (error) {
    widgetDualColumn.value = previous;
    widgetPreferenceError.value = error instanceof Error ? error.message : "设置保存失败";
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

    <div v-if="androidUpdateAvailable" class="settings-list">
      <SettingItem category="安卓壳" title="应用更新" description="检查安卓壳新版本">
        <button class="soft-button compact" type="button" :disabled="updateLoading" @click="checkAndroidUpdate">
          {{ updateLoading ? "正在检查" : "检查更新" }}
        </button>
      </SettingItem>

      <SettingItem category="桌面小组件" title="双列显示" description="关闭后始终使用单列">
        <button type="button" class="settings-switch" :class="{ active: widgetDualColumn }"
          :aria-pressed="widgetDualColumn" :disabled="widgetPreferenceLoading" @click="saveWidgetDualColumn">
          <span></span>
        </button>
        <template #message>
          <small v-if="widgetPreferenceError" class="setting-item-error">{{ widgetPreferenceError }}</small>
        </template>
      </SettingItem>
    </div>
  </main>
</template>
