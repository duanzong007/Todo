<script setup lang="ts">
import { computed, ref } from "vue";
import AccountPage from "./components/AccountPage.vue";
import DashboardPage from "./components/DashboardPage.vue";
import FriendsPage from "./components/FriendsPage.vue";
import NativeSMSPage from "./components/NativeSMSPage.vue";

const currentPath = ref(window.location.pathname);

const isDashboardRoute = computed(() => currentPath.value === "/" || currentPath.value === "");
const isNativeSMSRoute = computed(() => currentPath.value.startsWith("/sms/native"));
const isFriendsRoute = computed(() => currentPath.value === "/me/friends");
</script>

<template>
  <DashboardPage v-if="isDashboardRoute" />
  <NativeSMSPage v-else-if="isNativeSMSRoute" />
  <FriendsPage v-else-if="isFriendsRoute" />
  <AccountPage v-else />
</template>
