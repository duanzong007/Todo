<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { APIError, fetchAccountData } from "../api/client";
import type { AccountPageData } from "../types";
import { isAndroidShell } from "../utils/androidShell";

const state = ref<"loading" | "ready" | "unauthorized" | "error">("loading");
const account = ref<AccountPageData | null>(null);
const errorMessage = ref("");

type ChangelogKind = "app" | "android";

interface ChangelogEntry {
  version: string;
  date: string;
  summary: string;
  items: string[];
}

const androidShell = ref(false);
const activeKind = ref<ChangelogKind>("app");
const appEntries: ChangelogEntry[] = [
  {
    version: "v1.2.0",
    date: "2026-06-21",
    summary: "完善更新日志分类和云端小组件偏好。",
    items: [
      "更新日志区分应用更新和安卓壳功能更新。",
      "新增小组件双列显示云端设置，默认开启。",
      "设置页的安卓专属功能只在安卓壳环境显示。",
    ],
  },
  {
    version: "v1.1.0",
    date: "2026-06-16",
    summary: "增加安卓壳在线更新服务支持。",
    items: [
      "新增安卓壳版本查询接口和发布配置。",
      "设置页支持和安卓原生更新能力联动。",
    ],
  },
  {
    version: "v1.0.0",
    date: "2026-06-16",
    summary: "开启正式版本管理和更新日志。",
    items: [
      "统一菜单入口，任务管理、好友管理、更新日志、设置集中管理。",
      "任务管理支持筛选、批量编辑、共享和取消共享。",
      "好友管理支持邮箱搜索、申请和处理好友请求。",
      "首页支持 AI 添加任务、手动添加、短信导入和 ICS 导入。",
    ],
  },
];
const androidEntries: ChangelogEntry[] = [
  {
    version: "v1.2.0",
    date: "2026-06-21",
    summary: "增加小组件单列和双列显示偏好。",
    items: [
      "设置页可以手动开启或关闭小组件双列显示。",
      "关闭后，小组件始终按单列向下排列。",
      "设置保存到云端，并在小组件刷新时自动同步。",
    ],
  },
  {
    version: "v1.1.0",
    date: "2026-06-16",
    summary: "增加安卓壳更新检查和下载安装能力。",
    items: [
      "安卓壳会自动检查新版本，也可以在设置页手动检查。",
      "使用内置下载器显示进度，不依赖系统下载管理器。",
      "安装前校验 SHA256，校验通过后打开系统安装界面。",
    ],
  },
  {
    version: "v1.0.0",
    date: "2026-06-16",
    summary: "安卓壳进入正式版本管理。",
    items: [
      "支持内置 SSO 登录、系统返回分层、小组件和短信读取。",
    ],
  },
];
const visibleEntries = computed(() => activeKind.value === "android" ? androidEntries : appEntries);
const visibleTitle = computed(() => {
  if (!androidShell.value) {
    return "软件更新";
  }
  return activeKind.value === "android" ? "安卓壳功能更新" : "应用更新";
});

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
  androidShell.value = isAndroidShell();
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

    <nav v-if="androidShell" class="changelog-tabs" aria-label="更新日志分类">
      <button type="button" :class="{ active: activeKind === 'app' }" @click="activeKind = 'app'">应用更新</button>
      <button type="button" :class="{ active: activeKind === 'android' }" @click="activeKind = 'android'">安卓壳功能更新</button>
    </nav>

    <section class="changelog-panel">
      <header class="changelog-panel-title">
        <span class="eyebrow">版本记录</span>
        <h1>{{ visibleTitle }}</h1>
      </header>
      <article v-for="entry in visibleEntries" :key="`${activeKind}-${entry.version}`" class="changelog-entry">
        <time :datetime="entry.date">{{ entry.date }}</time>
        <div>
          <h1>{{ entry.version }}</h1>
          <p>{{ entry.summary }}</p>
          <ul>
            <li v-for="item in entry.items" :key="item">{{ item }}</li>
          </ul>
        </div>
      </article>
    </section>
  </main>
</template>
