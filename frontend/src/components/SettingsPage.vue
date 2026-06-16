<script setup lang="ts">
import { onMounted, ref } from "vue";
import { APIError, fetchAccountData } from "../api/client";
import type { AccountPageData } from "../types";

const state = ref<"loading" | "ready" | "unauthorized" | "error">("loading");
const account = ref<AccountPageData | null>(null);
const errorMessage = ref("");

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
});
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
      <span class="eyebrow">预留</span>
      <h1>设置</h1>
      <p>后续的偏好、开关和接口配置会放在这里。</p>
    </section>
  </main>
</template>
