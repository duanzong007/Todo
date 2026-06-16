<script setup lang="ts">
import { computed, ref, watchEffect } from "vue";
import AccountPage from "./components/AccountPage.vue";
import ChangelogPage from "./components/ChangelogPage.vue";
import DashboardPage from "./components/DashboardPage.vue";
import FriendsPage from "./components/FriendsPage.vue";
import MeHomePage from "./components/MeHomePage.vue";
import NativeSMSPage from "./components/NativeSMSPage.vue";
import SettingsPage from "./components/SettingsPage.vue";

const currentPath = ref(window.location.pathname);

const isDashboardRoute = computed(() => currentPath.value === "/" || currentPath.value === "");
const isNativeSMSRoute = computed(() => currentPath.value.startsWith("/sms/native"));
const isMeHomeRoute = computed(() => currentPath.value === "/me");
const isTasksRoute = computed(() => currentPath.value === "/me/tasks");
const isFriendsRoute = computed(() => currentPath.value === "/me/friends");
const isChangelogRoute = computed(() => currentPath.value === "/me/changelog");
const isSettingsRoute = computed(() => currentPath.value === "/me/settings");

const pageTitle = computed(() => {
  if (isDashboardRoute.value) return "Todo";
  if (isNativeSMSRoute.value) return "短信导入 - Todo";
  if (isTasksRoute.value) return "任务管理 - Todo";
  if (isFriendsRoute.value) return "好友 - Todo";
  if (isChangelogRoute.value) return "更新日志 - Todo";
  if (isSettingsRoute.value) return "设置 - Todo";
  return "菜单 - Todo";
});

watchEffect(() => {
  document.title = pageTitle.value;
});
</script>

<template>
  <DashboardPage v-if="isDashboardRoute" />
  <NativeSMSPage v-else-if="isNativeSMSRoute" />
  <MeHomePage v-else-if="isMeHomeRoute" />
  <AccountPage v-else-if="isTasksRoute" />
  <FriendsPage v-else-if="isFriendsRoute" />
  <ChangelogPage v-else-if="isChangelogRoute" />
  <SettingsPage v-else-if="isSettingsRoute" />
  <MeHomePage v-else />
</template>
