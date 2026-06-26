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
    version: "v1.2.2",
    date: "2026-06-26",
    summary: "修复手机端 DDL 剩余时间显示。",
    items: [],
  },
  {
    version: "v1.2.1",
    date: "2026-06-21",
    summary: "精简设置页和更新日志界面。",
    items: [],
  },
  {
    version: "v1.2.0",
    date: "2026-06-21",
    summary: "新增小组件单双列设置，并区分应用与安卓壳更新日志。",
    items: [
      "小组件布局设置可以随账号同步。",
    ],
  },
  {
    version: "v1.1.0",
    date: "2026-06-16",
    summary: "新增安卓壳在线更新服务。",
    items: [],
  },
  {
    version: "v1.0.0",
    date: "2026-06-16",
    summary: "新增菜单、任务管理、好友管理和 AI 添加。",
    items: [
      "支持任务共享、短信导入和 ICS 导入。",
    ],
  },
];
const androidEntries: ChangelogEntry[] = [
  {
    version: "v1.2.0",
    date: "2026-06-21",
    summary: "新增小组件单双列切换，设置随账号同步。",
    items: [],
  },
  {
    version: "v1.1.0",
    date: "2026-06-16",
    summary: "新增自动检查更新、手动检查和应用内下载。",
    items: [
      "安装前会校验安装包完整性。",
    ],
  },
  {
    version: "v1.0.0",
    date: "2026-06-16",
    summary: "新增内置 SSO 登录、小组件、短信读取和分层返回。",
    items: [],
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
          <ul v-if="entry.items.length">
            <li v-for="item in entry.items" :key="item">{{ item }}</li>
          </ul>
        </div>
      </article>
    </section>
  </main>
</template>
