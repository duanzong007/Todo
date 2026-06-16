<script setup lang="ts">
import { onMounted, ref } from "vue";
import { APIError, fetchAccountData } from "../api/client";
import type { AccountPageData } from "../types";

const state = ref<"loading" | "ready" | "unauthorized" | "error">("loading");
const account = ref<AccountPageData | null>(null);
const errorMessage = ref("");

const systemVersion = "v1.0.0";

async function loadChangelog() {
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
    errorMessage.value = error instanceof Error ? error.message : "更新日志加载失败";
  }
}

onMounted(() => {
  void loadChangelog();
});
</script>

<template>
  <main class="manage-shell changelog-shell">
    <header class="manage-top">
      <a class="manage-user" href="/me">
        <span class="eyebrow">更新日志</span>
        <strong>{{ account?.current_user.display_name || "Todo" }}</strong>
      </a>
      <div class="manage-top-actions">
        <a class="soft-button compact" href="/">返回首页</a>
        <a class="soft-button compact" href="/me">返回菜单</a>
      </div>
    </header>

    <p v-if="errorMessage" class="inline-error">{{ errorMessage }}</p>
    <p v-if="state === 'unauthorized'" class="inline-error">请先登录</p>

    <section class="changelog-panel">
      <article class="changelog-entry">
        <time datetime="2026-06-16">2026-06-16</time>
        <div>
          <h1>{{ systemVersion }}</h1>
          <p>开启正式版本管理和更新日志。</p>
          <ul>
            <li>统一菜单入口，任务管理、好友管理、更新日志、设置集中管理。</li>
            <li>任务管理支持筛选、批量编辑、共享和取消共享。</li>
            <li>好友管理支持邮箱搜索、申请和处理好友请求。</li>
            <li>首页支持 AI 添加任务、手动添加、短信导入和 ICS 导入。</li>
            <li>安卓壳支持内置登录、小组件和分层返回。</li>
          </ul>
        </div>
      </article>
    </section>
  </main>
</template>
