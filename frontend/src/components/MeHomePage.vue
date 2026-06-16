<script setup lang="ts">
import { onMounted, ref } from "vue";
import { APIError, fetchAccountData } from "../api/client";
import type { AccountPageData } from "../types";

const state = ref<"loading" | "ready" | "unauthorized" | "error">("loading");
const account = ref<AccountPageData | null>(null);
const errorMessage = ref("");

async function loadMe() {
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
    errorMessage.value = error instanceof Error ? error.message : "菜单页加载失败";
  }
}

onMounted(() => {
  void loadMe();
});
</script>

<template>
  <main class="manage-shell me-home-shell">
    <header class="manage-top">
      <a class="manage-user" href="/">
        <span class="eyebrow">菜单</span>
        <strong>{{ account?.current_user.display_name || "Todo" }}</strong>
      </a>
      <div class="manage-top-actions">
        <a class="soft-button compact" href="/">返回首页</a>
      </div>
    </header>

    <p v-if="errorMessage" class="inline-error">{{ errorMessage }}</p>
    <p v-if="state === 'unauthorized'" class="inline-error">请先登录</p>

    <section class="me-menu-panel" aria-label="个人功能">
      <a class="me-menu-item" href="/me/tasks">
        <span>
          <strong>任务管理</strong>
          <small>筛选、批量编辑、共享、删除</small>
        </span>
      </a>

      <a class="me-menu-item" href="/me/friends">
        <span>
          <strong>好友管理</strong>
          <small>添加好友、处理申请、查看好友</small>
        </span>
      </a>

      <a class="me-menu-item" href="/me/changelog">
        <span>
          <strong>更新日志</strong>
          <small>新功能增加后可以从这里找到</small>
        </span>
      </a>

      <a class="me-menu-item" href="/me/settings">
        <span>
          <strong>设置</strong>
          <small>偏好、开关和接口配置会放在这里</small>
        </span>
      </a>
    </section>

    <section class="me-logout-panel">
      <form method="post" action="/logout">
        <button class="danger-button" type="submit">退出登录</button>
      </form>
    </section>
  </main>
</template>
