<script setup lang="ts">
import { onMounted, ref } from "vue";
import { APIError, fetchAccountData } from "../api/client";
import type { AccountPageData } from "../types";

const state = ref<"loading" | "ready" | "unauthorized" | "error">("loading");
const account = ref<AccountPageData | null>(null);
const errorMessage = ref("");
const updateMessage = ref("");
const updateLoading = ref(false);
const androidUpdateAvailable = ref(false);

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
  void detectAndroidUpdate();
});

async function detectAndroidUpdate() {
  const plugin = androidUpdatePlugin();
  if (!plugin) {
    androidUpdateAvailable.value = false;
    return;
  }
  androidUpdateAvailable.value = true;
  try {
    await plugin.status?.();
  } catch (_error) {
    androidUpdateAvailable.value = false;
  }
}

async function checkAndroidUpdate() {
  const plugin = androidUpdatePlugin();
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

function androidUpdatePlugin() {
  const capacitor = (window as unknown as { Capacitor?: { Plugins?: Record<string, unknown>; isNativePlatform?: () => boolean; getPlatform?: () => string } }).Capacitor;
  if (!capacitor) {
    return null;
  }
  const isNative = typeof capacitor.isNativePlatform === "function"
    ? capacitor.isNativePlatform()
    : typeof capacitor.getPlatform === "function"
      ? capacitor.getPlatform() !== "web"
      : false;
  if (!isNative) {
    return null;
  }
  const plugin = capacitor.Plugins?.AndroidUpdate as
    | { status?: () => Promise<unknown>; check?: (options: { manual: boolean }) => Promise<{ message?: string }> }
    | undefined;
  return typeof plugin?.check === "function" ? plugin : null;
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
  </main>
</template>
