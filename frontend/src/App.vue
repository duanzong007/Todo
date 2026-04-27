<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { APIError, fetchDashboardSnapshot, openDashboardEvents } from "./api/client";
import type { ConnectionState, DashboardSnapshot } from "./types";

const state = ref<ConnectionState>("idle");
const snapshot = ref<DashboardSnapshot | null>(null);
const errorMessage = ref("");
const lastSyncedAt = ref<Date | null>(null);
const eventStatus = ref<"未连接" | "已连接" | "已断开">("未连接");

let eventStream: EventSource | null = null;
let syncTimer = 0;

const taskTotal = computed(() => snapshot.value?.focus_tasks.length ?? 0);
const completedTotal = computed(() => snapshot.value?.completed_tasks.length ?? 0);
const quote = computed(() => snapshot.value?.empty_quote);

async function loadSnapshot() {
  state.value = "loading";
  errorMessage.value = "";

  try {
    snapshot.value = await fetchDashboardSnapshot();
    lastSyncedAt.value = new Date();
    state.value = "ready";
  } catch (error) {
    if (error instanceof APIError && error.status === 401) {
      state.value = "unauthorized";
      errorMessage.value = "当前浏览器没有可用登录态。先在 Go 站点登录，再回到这里刷新。";
      return;
    }

    state.value = "error";
    errorMessage.value = "无法读取后端快照。确认 Go 服务正在运行，Vite 代理目标正确。";
  }
}

function scheduleSnapshotReload() {
  window.clearTimeout(syncTimer);
  syncTimer = window.setTimeout(() => {
    void loadSnapshot();
  }, 180);
}

function connectEvents() {
  if (eventStream) {
    eventStream.close();
    eventStream = null;
  }

  try {
    eventStream = openDashboardEvents(scheduleSnapshotReload);
    eventStream.onopen = () => {
      eventStatus.value = "已连接";
    };
    eventStream.onerror = () => {
      eventStatus.value = "已断开";
    };
  } catch (_error) {
    eventStatus.value = "已断开";
  }
}

onMounted(() => {
  void loadSnapshot();
  connectEvents();
});

onBeforeUnmount(() => {
  window.clearTimeout(syncTimer);
  if (eventStream) {
    eventStream.close();
  }
});
</script>

<template>
  <main class="app-shell">
    <section class="hero">
      <p class="eyebrow">Todo Vue Migration</p>
      <h1>前端迁移准备页</h1>
      <p class="summary">
        这个页面只用于验证 Vue + Vite 工程、后端接口边界和实时同步通道，不替换现有生产页面。
      </p>
    </section>

    <section class="status-grid" aria-label="迁移状态">
      <article class="status-card">
        <span class="label">后端快照</span>
        <strong>{{ state }}</strong>
      </article>
      <article class="status-card">
        <span class="label">实时通道</span>
        <strong>{{ eventStatus }}</strong>
      </article>
      <article class="status-card">
        <span class="label">当前任务</span>
        <strong>{{ taskTotal }}</strong>
      </article>
      <article class="status-card">
        <span class="label">已完成</span>
        <strong>{{ completedTotal }}</strong>
      </article>
    </section>

    <section class="panel">
      <div class="panel-head">
        <div>
          <p class="eyebrow">Backend Contract</p>
          <h2>Dashboard Snapshot</h2>
        </div>
        <button type="button" @click="loadSnapshot">刷新</button>
      </div>

      <p v-if="errorMessage" class="message is-error">{{ errorMessage }}</p>
      <p v-else-if="lastSyncedAt" class="message">
        最近同步 {{ lastSyncedAt.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", second: "2-digit" }) }}
      </p>

      <div v-if="snapshot" class="task-preview">
        <article v-for="task in snapshot.focus_tasks.slice(0, 5)" :key="task.id" class="task-row">
          <span class="kind">{{ task.kind_label }}</span>
          <strong>{{ task.title }}</strong>
          <small>{{ task.status_line || `${task.importance} 级` }}</small>
        </article>

        <article v-if="snapshot.focus_tasks.length === 0 && quote" class="quote-block">
          <p>{{ quote.text }}</p>
          <small v-if="quote.has_meta">{{ quote.meta_line }}</small>
        </article>

        <p v-if="snapshot.focus_tasks.length > 5" class="message">
          这里只显示前 5 条，用于接口验证。
        </p>
      </div>
    </section>
  </main>
</template>
