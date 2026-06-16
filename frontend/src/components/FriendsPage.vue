<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { APIError, fetchAccountData, requestFriend, respondFriendRequest } from "../api/client";
import type { AccountPageData } from "../types";

const state = ref<"loading" | "ready" | "unauthorized" | "error">("loading");
const account = ref<AccountPageData | null>(null);
const friendEmail = ref("");
const friendQuery = ref("");
const loadingMessage = ref("");
const errorMessage = ref("");
const noticeMessage = ref("");

let noticeTimer = 0;

const friends = computed(() => account.value?.share_users ?? []);
const pendingRequests = computed(() => account.value?.friend_requests ?? []);
const filteredFriends = computed(() => {
  const query = friendQuery.value.trim().toLowerCase();
  if (!query) {
    return friends.value;
  }
  return friends.value.filter((friend) => {
    const name = friend.display_name.toLowerCase();
    const email = friend.email.toLowerCase();
    return name.includes(query) || email.includes(query);
  });
});

async function loadFriends() {
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
    errorMessage.value = error instanceof Error ? error.message : "好友数据加载失败";
  }
}

async function submitFriendRequest() {
  const email = friendEmail.value.trim();
  if (!email) {
    return;
  }
  loadingMessage.value = "处理中";
  errorMessage.value = "";
  try {
    const response = await requestFriend(email);
    friendEmail.value = "";
    showNotice(response.message || "好友申请已发送");
    await loadFriends();
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "好友申请失败";
  } finally {
    loadingMessage.value = "";
  }
}

async function decideFriendRequest(userID: string, accept: boolean) {
  loadingMessage.value = "处理中";
  errorMessage.value = "";
  try {
    const response = await respondFriendRequest(userID, accept);
    showNotice(response.message || "已处理好友申请");
    await loadFriends();
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "好友申请处理失败";
  } finally {
    loadingMessage.value = "";
  }
}

function showNotice(message: string) {
  noticeMessage.value = message;
  window.clearTimeout(noticeTimer);
  noticeTimer = window.setTimeout(() => {
    noticeMessage.value = "";
  }, 2200);
}

onMounted(() => {
  void loadFriends();
});
</script>

<template>
  <main class="manage-shell friends-shell">
    <header v-if="account" class="manage-top">
      <a class="manage-user" href="/me">
        <span class="eyebrow">好友</span>
        <strong>{{ account.current_user.display_name }}</strong>
      </a>
      <div class="manage-top-actions">
        <a class="soft-button compact" href="/">返回首页</a>
        <a class="soft-button compact" href="/me">返回菜单</a>
      </div>
    </header>

    <p v-if="errorMessage" class="inline-error">{{ errorMessage }}</p>
    <p v-else-if="noticeMessage" class="inline-notice">{{ noticeMessage }}</p>
    <p v-if="state === 'unauthorized'" class="inline-error">请先登录</p>

    <section class="friends-panel">
      <form class="friend-request-form" @submit.prevent="submitFriendRequest">
        <label class="field wide">
          <span>添加好友</span>
          <input v-model="friendEmail" type="email" placeholder="输入邮箱，精准匹配" autocomplete="off" />
        </label>
        <button class="primary-button" type="submit" :disabled="!friendEmail.trim() || Boolean(loadingMessage)">
          {{ loadingMessage || "申请" }}
        </button>
      </form>
    </section>

    <section v-if="pendingRequests.length" class="friends-panel">
      <div class="friends-panel-head">
        <p>好友申请</p>
        <span>{{ pendingRequests.length }}</span>
      </div>
      <div class="friend-request-list">
        <article v-for="request in pendingRequests" :key="request.id" class="friend-request-item">
          <span>
            <strong>{{ request.display_name }}</strong>
            <small>{{ request.email }}</small>
          </span>
          <div>
            <button type="button" class="soft-button compact" :disabled="Boolean(loadingMessage)"
              @click="decideFriendRequest(request.id, true)">
              接受
            </button>
            <button type="button" class="soft-button compact" :disabled="Boolean(loadingMessage)"
              @click="decideFriendRequest(request.id, false)">
              忽略
            </button>
          </div>
        </article>
      </div>
    </section>

    <section class="friends-panel">
      <div class="friends-panel-head">
        <p>好友列表</p>
        <span>{{ friends.length }}</span>
      </div>
      <label class="field wide friend-search-field">
        <span>筛选</span>
        <input v-model="friendQuery" type="search" placeholder="按名称或邮箱筛选" autocomplete="off" />
      </label>
      <div v-if="state === 'loading'" class="empty-state">正在加载好友</div>
      <div v-else class="share-list friend-list">
        <article v-for="friend in filteredFriends" :key="friend.id" class="share-user friend-card">
          <span>
            <strong>{{ friend.display_name }}</strong>
            <small>{{ friend.email }}</small>
          </span>
        </article>
        <p v-if="filteredFriends.length === 0" class="share-empty">没有符合条件的好友</p>
      </div>
    </section>

    <div v-if="loadingMessage" class="sync-chip active">{{ loadingMessage }}</div>
  </main>
</template>
