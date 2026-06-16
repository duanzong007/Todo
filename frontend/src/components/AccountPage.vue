<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { APIError, applyAccountAction, fetchAccountData, openDashboardEvents } from "../api/client";
import type {
  AccountCheckOption,
  AccountFilterOption,
  AccountPageData,
  ConnectionState,
  ManagedTaskCard,
  ShareableUserCard,
} from "../types";
import AccountSelect from "./AccountSelect.vue";
import WheelDatePicker from "./WheelDatePicker.vue";

type ModalName = "filter" | "edit" | "share" | "";

const ACCOUNT_SCROLL_KEY = "todo-account-scroll-y";

interface FilterDraft {
  query: string;
  status: string;
  scope: string;
  dateField: string;
  sort: string;
  limit: string;
  dateFrom: string;
  dateTo: string;
  types: string[];
  importances: string[];
}

interface EditDraft {
  replaceTitle: string;
  prefix: string;
  suffix: string;
  importance: string;
  scheduleDate: string;
  deadlineValue: string;
}

const state = ref<ConnectionState>("idle");
const account = ref<AccountPageData | null>(null);
const selectedIds = ref<Set<string>>(new Set());
const activeModal = ref<ModalName>("");
const loadingMessage = ref("");
const errorMessage = ref("");
const noticeMessage = ref("");
const shareSelection = ref<Set<string>>(new Set());
const shareQuery = ref("");
const unshareSelection = ref<Set<string>>(new Set());

const filterDraft = reactive<FilterDraft>({
  query: "",
  status: "all",
  scope: "all",
  dateField: "",
  sort: "updated_desc",
  limit: "10",
  dateFrom: "",
  dateTo: "",
  types: [],
  importances: [],
});

const editDraft = reactive<EditDraft>({
  replaceTitle: "",
  prefix: "",
  suffix: "",
  importance: "",
  scheduleDate: "",
  deadlineValue: "",
});

let eventStream: EventSource | null = null;
let reloadTimer = 0;
let noticeTimer = 0;

const tasks = computed(() => account.value?.tasks ?? []);
const selectedTasks = computed(() => tasks.value.filter((task) => selectedIds.value.has(task.id)));
const selectedCount = computed(() => selectedIds.value.size);
const hasSelection = computed(() => selectedCount.value > 0);
const singleSelectedTask = computed(() => (selectedTasks.value.length === 1 ? selectedTasks.value[0] : null));
const allSelectedOnPage = computed(
  () => tasks.value.length > 0 && tasks.value.every((task) => selectedIds.value.has(task.id)),
);
const selectedOwnedOnly = computed(() => selectedTasks.value.length > 0 && selectedTasks.value.every((task) => task.is_owner));
const selectedHasSharing = computed(() =>
  selectedTasks.value.some((task) => task.shared_with_me || (task.is_owner && task.shared_users.length > 0)),
);
const unshareUsers = computed(() => {
  const byID = new Map<string, ShareableUserCard>();
  selectedTasks.value.forEach((task) => {
    if (!task.is_owner) {
      return;
    }
    task.shared_users.forEach((user) => byID.set(user.id, user));
  });
  return Array.from(byID.values());
});
const hasSharedWithMeSelection = computed(() => selectedTasks.value.some((task) => task.shared_with_me));
const canSubmitUnshare = computed(
  () => selectedHasSharing.value && (hasSharedWithMeSelection.value || unshareSelection.value.size > 0),
);
const selectedBlockedCopy = computed(() => {
  if (!hasSelection.value || selectedOwnedOnly.value) {
    return "";
  }
  return "包含共享给我的任务，不能共享或删除";
});
const limitValue = computed(() => filterDraft.limit || selectedValue(account.value?.filter.limit_options ?? [], "10"));
const hasDateFilter = computed(() => filterDraft.dateField.trim() !== "");
const currentFilterSummary = computed(() => {
  const summary = account.value?.filter.summary || "无筛选";
  return summary.trim() || "无筛选";
});
const selectedTypeLabel = computed(() => checkedSummary(account.value?.filter.type_options ?? [], filterDraft.types, "全部类型"));
const selectedImportanceLabel = computed(() =>
  checkedSummary(account.value?.filter.importance_options ?? [], filterDraft.importances, "全部星级"),
);
const filteredShareUsers = computed(() => {
  const query = shareQuery.value.trim().toLowerCase();
  const users = account.value?.share_users ?? [];
  if (!query) {
    return users;
  }
  return users.filter((user) => {
    const name = user.display_name.toLowerCase();
    const email = user.email.toLowerCase();
    return name.includes(query) || email.includes(query);
  });
});

const returnQuery = computed(() => {
  const search = currentSearchParams();
  search.delete("msg");
  search.delete("err");
  return search.toString();
});

async function loadAccount(search = window.location.search, options: { preserveSelection?: boolean; restoreScroll?: boolean } = {}) {
  state.value = account.value ? "ready" : "loading";
  errorMessage.value = "";

  try {
    const next = await fetchAccountData(search);
    account.value = next;
    state.value = "ready";
    syncFilterDraft(next);
    if (!options.preserveSelection) {
      selectedIds.value = new Set();
    } else {
      pruneSelection(next.tasks);
    }
    if (options.restoreScroll) {
      await restoreScrollPosition();
    }
  } catch (error) {
    if (error instanceof APIError && error.status === 401) {
      state.value = "unauthorized";
      errorMessage.value = "当前浏览器没有可用登录态。先在 Go 站点登录，再回到这里刷新。";
      return;
    }
    state.value = "error";
    errorMessage.value = error instanceof Error ? error.message : "任务管理数据加载失败";
  }
}

function saveScrollPosition() {
  try {
    window.sessionStorage.setItem(ACCOUNT_SCROLL_KEY, String(window.scrollY));
  } catch (_error) {
    // Ignore storage failures.
  }
}

async function restoreScrollPosition() {
  await nextTick();
  try {
    const saved = window.sessionStorage.getItem(ACCOUNT_SCROLL_KEY);
    if (!saved) {
      return;
    }
    window.sessionStorage.removeItem(ACCOUNT_SCROLL_KEY);
    const y = Number.parseInt(saved, 10);
    if (Number.isFinite(y)) {
      window.scrollTo({ top: y, behavior: "auto" });
    }
  } catch (_error) {
    // Ignore storage failures.
  }
}

function scheduleSilentReload() {
  window.clearTimeout(reloadTimer);
  reloadTimer = window.setTimeout(() => {
    void loadAccount(window.location.search, { preserveSelection: true });
  }, 250);
}

function connectEvents() {
  if (eventStream) {
    eventStream.close();
  }
  try {
    eventStream = openDashboardEvents(scheduleSilentReload);
  } catch (_error) {
    eventStream = null;
  }
}

function syncFilterDraft(data: AccountPageData) {
  filterDraft.query = data.filter.query;
  filterDraft.status = selectedValue(data.filter.status_options, "all");
  filterDraft.scope = selectedValue(data.filter.scope_options, "all");
  filterDraft.dateField = selectedValue(data.filter.date_field_options, "");
  filterDraft.sort = selectedValue(data.filter.sort_options, "updated_desc");
  filterDraft.limit = selectedValue(data.filter.limit_options, "10");
  filterDraft.dateFrom = data.filter.date_from;
  filterDraft.dateTo = data.filter.date_to;
  filterDraft.types = checkedValues(data.filter.type_options);
  filterDraft.importances = checkedValues(data.filter.importance_options);
}

function selectedValue(options: AccountFilterOption[], fallback: string) {
  return options.find((option) => option.selected)?.value ?? fallback;
}

function checkedValues(options: AccountCheckOption[]) {
  return options.filter((option) => option.checked).map((option) => option.value);
}

function checkedSummary(options: AccountCheckOption[], selected: string[], fallback: string) {
  if (selected.length === 0) {
    return fallback;
  }
  if (selected.length === 1) {
    return options.find((option) => option.value === selected[0])?.label ?? fallback;
  }
  return `${selected.length} 项`;
}

function currentSearchParams() {
  return new URLSearchParams(window.location.search);
}

function updateQuery(params: URLSearchParams) {
  saveScrollPosition();
  const search = params.toString();
  const nextURL = `${window.location.pathname}${search ? `?${search}` : ""}`;
  window.history.pushState(null, "", nextURL);
  void loadAccount(window.location.search, { restoreScroll: true });
}

function buildFilterParams(page = "1") {
  const params = new URLSearchParams();
  setParam(params, "q", filterDraft.query);
  setParam(params, "status", filterDraft.status === "all" ? "" : filterDraft.status);
  setParam(params, "scope", filterDraft.scope === "all" ? "" : filterDraft.scope);
  setParam(params, "date_field", filterDraft.dateField);
  setParam(params, "sort", filterDraft.sort === "updated_desc" ? "" : filterDraft.sort);
  if (hasDateFilter.value) {
    setParam(params, "date_from", filterDraft.dateFrom);
    setParam(params, "date_to", filterDraft.dateTo);
  }
  filterDraft.types.forEach((value) => params.append("type", value));
  filterDraft.importances.forEach((value) => params.append("importance", value));
  setParam(params, "limit", filterDraft.limit === "10" ? "" : filterDraft.limit);
  setParam(params, "page", page === "1" ? "" : page);
  return params;
}

function setParam(params: URLSearchParams, key: string, value: string) {
  const trimmed = value.trim();
  if (trimmed) {
    params.set(key, trimmed);
  }
}

function applyFilters() {
  activeModal.value = "";
  updateQuery(buildFilterParams("1"));
}

function toggleCheckFilter(kind: "types" | "importances", value: string) {
  const values = filterDraft[kind];
  const next = values.includes(value) ? values.filter((item) => item !== value) : [...values, value];
  filterDraft[kind] = next;
}

function resetFilters() {
  const nextLimit = limitValue.value;
  filterDraft.query = "";
  filterDraft.status = "all";
  filterDraft.scope = "all";
  filterDraft.dateField = "";
  filterDraft.sort = "updated_desc";
  filterDraft.limit = nextLimit;
  filterDraft.dateFrom = "";
  filterDraft.dateTo = "";
  filterDraft.types = [];
  filterDraft.importances = [];
  activeModal.value = "";
  const params = new URLSearchParams();
  setParam(params, "limit", nextLimit === "10" ? "" : nextLimit);
  updateQuery(params);
}

function changePage(page: number) {
  const totalPages = account.value?.pagination.total_pages ?? 1;
  const normalized = Math.max(1, Math.min(totalPages, page));
  const params = currentSearchParams();
  if (normalized === 1) {
    params.delete("page");
  } else {
    params.set("page", String(normalized));
  }
  updateQuery(params);
}

function changeLimit(value: string) {
  filterDraft.limit = value || "10";
  const params = currentSearchParams();
  if (value === "10") {
    params.delete("limit");
  } else {
    params.set("limit", value);
  }
  params.delete("page");
  updateQuery(params);
}

function toggleTask(taskID: string, checked: boolean) {
  const next = new Set(selectedIds.value);
  if (checked) {
    next.add(taskID);
  } else {
    next.delete(taskID);
  }
  selectedIds.value = next;
}

function togglePageSelection(checked: boolean) {
  const next = new Set(selectedIds.value);
  tasks.value.forEach((task) => {
    if (checked) {
      next.add(task.id);
    } else {
      next.delete(task.id);
    }
  });
  selectedIds.value = next;
}

function clearSelection() {
  selectedIds.value = new Set();
}

function pruneSelection(nextTasks: ManagedTaskCard[]) {
  const visible = new Set(nextTasks.map((task) => task.id));
  const next = new Set<string>();
  selectedIds.value.forEach((id) => {
    if (visible.has(id)) {
      next.add(id);
    }
  });
  selectedIds.value = next;
}

function openFilter() {
  if (account.value) {
    syncFilterDraft(account.value);
  }
  activeModal.value = "filter";
}

function openEdit() {
  if (!hasSelection.value) {
    return;
  }
  const single = singleSelectedTask.value;
  editDraft.replaceTitle = single?.title ?? "";
  editDraft.prefix = "";
  editDraft.suffix = "";
  editDraft.importance = single ? String(single.importance) : "";
  editDraft.scheduleDate = single?.schedule_value ?? "";
  editDraft.deadlineValue =
    single?.deadline_date && single.deadline_time ? `${single.deadline_date}T${single.deadline_time}` : "";
  unshareSelection.value = new Set(unshareUsers.value.map((user) => user.id));
  activeModal.value = "edit";
}

function openShare() {
  if (!hasSelection.value || !selectedOwnedOnly.value) {
    return;
  }
  shareSelection.value = new Set();
  shareQuery.value = "";
  activeModal.value = "share";
}

function closeModal() {
  activeModal.value = "";
}

function baseActionForm(action = "patch") {
  const formData = new FormData();
  formData.set("action", action);
  formData.set("selected_ids", Array.from(selectedIds.value).join(","));
  formData.set("return_query", returnQuery.value);
  return formData;
}

async function submitEdit() {
  if (!hasSelection.value) {
    return;
  }
  const formData = baseActionForm("patch");
  const single = singleSelectedTask.value;
  if (single) {
    formData.set("replace_title", editDraft.replaceTitle);
    if (single.schedule_mode === "date") {
      formData.set("schedule_date", editDraft.scheduleDate);
    }
    if (single.schedule_mode === "datetime") {
      formData.set("deadline_value", editDraft.deadlineValue);
    }
  } else {
    formData.set("title_prefix", editDraft.prefix);
    formData.set("title_suffix", editDraft.suffix);
  }
  formData.set("importance", editDraft.importance);
  await submitAction(formData);
}

async function submitShare() {
  const formData = baseActionForm("share");
  shareSelection.value.forEach((id) => formData.append("share_user_id", id));
  await submitAction(formData);
}

async function submitUnshare() {
  if (!hasSelection.value || !selectedHasSharing.value) {
    return;
  }
  const formData = baseActionForm("unshare");
  unshareSelection.value.forEach((id) => formData.append("unshare_user_id", id));
  await submitAction(formData);
}

async function submitDelete() {
  if (!hasSelection.value || !selectedOwnedOnly.value) {
    return;
  }
  if (!window.confirm(`确认删除 ${selectedCount.value} 条任务？删除后不会保留记录。`)) {
    return;
  }
  const formData = baseActionForm("delete");
  await submitAction(formData);
}

async function submitAction(formData: FormData) {
  saveScrollPosition();
  loadingMessage.value = "处理中";
  errorMessage.value = "";
  try {
    const response = await applyAccountAction(formData);
    activeModal.value = "";
    selectedIds.value = new Set();
    showNotice(response.message || "已完成");
    await loadAccount(window.location.search, { restoreScroll: true });
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "操作失败";
  } finally {
    loadingMessage.value = "";
  }
}

function toggleShareUser(user: ShareableUserCard, checked: boolean) {
  const next = new Set(shareSelection.value);
  if (checked) {
    next.add(user.id);
  } else {
    next.delete(user.id);
  }
  shareSelection.value = next;
}

function toggleUnshareUser(user: ShareableUserCard, checked: boolean) {
  const next = new Set(unshareSelection.value);
  if (checked) {
    next.add(user.id);
  } else {
    next.delete(user.id);
  }
  unshareSelection.value = next;
}

function showNotice(message: string) {
  noticeMessage.value = message;
  window.clearTimeout(noticeTimer);
  noticeTimer = window.setTimeout(() => {
    noticeMessage.value = "";
  }, 2200);
}

function taskKindClass(task: ManagedTaskCard) {
  return {
    todo: task.kind_class === "todo",
    schedule: task.kind_class === "schedule",
    ddl: task.kind_class === "ddl",
  };
}

function scheduleLabel(task: ManagedTaskCard) {
  if (task.schedule_mode === "date") {
    return "日期";
  }
  if (task.schedule_mode === "datetime") {
    return "截止";
  }
  return "";
}

function setEditImportance(value: string) {
  editDraft.importance = editDraft.importance === value ? "" : value;
}

onMounted(() => {
  void loadAccount();
  connectEvents();
  window.addEventListener("popstate", onPopState);
});

onBeforeUnmount(() => {
  window.clearTimeout(reloadTimer);
  window.clearTimeout(noticeTimer);
  window.removeEventListener("popstate", onPopState);
  if (eventStream) {
    eventStream.close();
  }
});

function onPopState() {
  void loadAccount(window.location.search);
}
</script>

<template>
  <main class="manage-shell">
    <header v-if="account" class="manage-top">
      <a class="manage-user" href="/me">
        <span class="eyebrow">任务管理</span>
        <strong>{{ account.current_user.display_name }}</strong>
      </a>
      <div class="manage-top-actions">
        <a class="soft-button compact" href="/">返回首页</a>
        <a class="soft-button compact" href="/me">返回菜单</a>
      </div>
    </header>

    <section class="manage-filter-surface is-compact" aria-label="任务筛选">
      <button type="button" class="manage-filter-button" @click="openFilter">筛选器</button>
      <div v-if="account" class="manage-filter-meta">
        <span>{{ currentFilterSummary }}</span>
        <button type="button" @click="resetFilters">清空</button>
      </div>
    </section>

    <p v-if="errorMessage" class="inline-error">{{ errorMessage }}</p>
    <p v-else-if="noticeMessage" class="inline-notice">{{ noticeMessage }}</p>
    <p v-if="state === 'unauthorized'" class="inline-error">当前浏览器没有可用登录态。先在 Go 站点登录，再回到这里刷新。</p>

    <section class="manage-list-shell">
      <div v-if="account" class="manage-list-head">
        <label class="manage-select-page">
          <input type="checkbox" :checked="allSelectedOnPage"
            @change="togglePageSelection(($event.target as HTMLInputElement).checked)" />
          <span>选择本页</span>
        </label>
        <span>{{ account.pagination.total_items }} 条</span>
      </div>

      <div v-if="state === 'loading'" class="empty-state">正在加载任务管理数据</div>
      <div v-else-if="state === 'error' && !account" class="empty-state">任务管理数据加载失败</div>

      <div v-else-if="tasks.length" class="manage-task-list">
        <article v-for="task in tasks" :key="task.id" class="manage-task-row"
          :class="{ selected: selectedIds.has(task.id), shared: task.shared_with_me }">
          <label class="manage-row-check">
            <input type="checkbox" :checked="selectedIds.has(task.id)"
              @change="toggleTask(task.id, ($event.target as HTMLInputElement).checked)" />
          </label>

          <div class="manage-task-main">
            <div class="manage-task-line">
              <span class="kind-pill" :class="taskKindClass(task)">{{ task.kind_label }}</span>
              <span class="manage-importance">{{ task.importance }} 级</span>
              <span v-if="task.shared_with_me" class="manage-origin">共享给我</span>
              <span class="status-pill" :class="task.status_class">{{ task.status_label }}</span>
            </div>
            <h2>{{ task.title }}</h2>
            <div class="manage-task-sub">
              <span v-if="task.date_line">{{ task.date_line }}</span>
              <span v-if="task.shared_line">{{ task.shared_line }}</span>
              <span v-if="task.note">{{ task.note }}</span>
            </div>
          </div>
        </article>
      </div>

      <div v-else class="empty-state">没有符合条件的任务</div>

      <footer v-if="account" class="manage-pagination">
        <div class="manage-page-controls">
          <button class="soft-button compact" type="button" :disabled="!account.pagination.has_prev"
            @click="changePage(account.pagination.prev_page)">
            上一页
          </button>
          <label class="manage-page-input">
            <span>第</span>
            <input type="number" min="1" :max="account.pagination.total_pages" :value="account.pagination.page"
              @change="changePage(Number(($event.target as HTMLInputElement).value || 1))" />
            <span>/ {{ account.pagination.total_pages }} 页</span>
          </label>
          <button class="soft-button compact" type="button" :disabled="!account.pagination.has_next"
            @click="changePage(account.pagination.next_page)">
            下一页
          </button>
        </div>
        <label class="manage-limit-select">
          <span>每页</span>
          <AccountSelect :model-value="limitValue" :options="account.filter.limit_options" compact center-menu
            @update:model-value="changeLimit" />
        </label>
      </footer>
    </section>

    <Transition name="manage-bar">
      <section v-if="hasSelection" class="manage-selection-bar" aria-label="批量操作">
        <button type="button" class="manage-selected-count" @click="clearSelection">已选 {{ selectedCount }} 条</button>
        <div class="manage-selection-actions">
          <button type="button" @click="openEdit">编辑</button>
          <button type="button" :disabled="!selectedOwnedOnly" :title="selectedBlockedCopy" @click="openShare">共享</button>
          <button type="button" class="danger" :disabled="!selectedOwnedOnly" :title="selectedBlockedCopy" @click="submitDelete">
            删除
          </button>
        </div>
      </section>
    </Transition>

    <div v-if="activeModal" class="manage-modal-layer" @click.self="closeModal">
      <section v-if="activeModal === 'filter'" class="manage-drawer manage-filter-drawer">
        <header class="manage-drawer-head">
          <div>
            <p class="eyebrow">筛选</p>
            <h2>筛选任务</h2>
          </div>
          <button type="button" class="soft-button compact" @click="closeModal">关闭</button>
        </header>

        <div class="manage-drawer-grid">
          <label class="field wide">
            <span>搜索</span>
            <input v-model="filterDraft.query" type="search" placeholder="按标题或备注搜索" autocomplete="off" />
          </label>
          <label class="field">
            <span>状态</span>
            <AccountSelect v-model="filterDraft.status" :options="account?.filter.status_options ?? []" />
          </label>
          <label class="field">
            <span>范围</span>
            <AccountSelect v-model="filterDraft.scope" :options="account?.filter.scope_options ?? []" />
          </label>
          <label class="field">
            <span>排序</span>
            <AccountSelect v-model="filterDraft.sort" :options="account?.filter.sort_options ?? []" />
          </label>
          <label class="field">
            <span>日期维度</span>
            <AccountSelect v-model="filterDraft.dateField" :options="account?.filter.date_field_options ?? []" />
          </label>
          <label v-if="hasDateFilter" class="field">
            <span>开始日期</span>
            <WheelDatePicker v-model="filterDraft.dateFrom" empty-label="开始日期" />
          </label>
          <label v-if="hasDateFilter" class="field">
            <span>结束日期</span>
            <WheelDatePicker v-model="filterDraft.dateTo" empty-label="结束日期" />
          </label>
        </div>

        <div class="manage-choice-block">
          <span>{{ selectedTypeLabel }}</span>
          <button v-for="option in account?.filter.type_options" :key="option.value" type="button" class="manage-chip"
            :class="{ active: filterDraft.types.includes(option.value) }" @click="toggleCheckFilter('types', option.value)">
            {{ option.label }}
          </button>
        </div>

        <div class="manage-choice-block">
          <span>{{ selectedImportanceLabel }}</span>
          <button v-for="option in account?.filter.importance_options" :key="option.value" type="button"
            class="manage-chip" :class="{ active: filterDraft.importances.includes(option.value) }"
            @click="toggleCheckFilter('importances', option.value)">
            {{ option.label }}
          </button>
        </div>

        <footer class="manage-modal-actions">
          <button class="soft-button" type="button" @click="resetFilters">清空</button>
          <button class="primary-button" type="button" @click="applyFilters">应用</button>
        </footer>
      </section>

      <section v-if="activeModal === 'edit'" class="manage-drawer">
        <header class="manage-drawer-head">
          <div>
            <p class="eyebrow">编辑</p>
            <h2>{{ singleSelectedTask ? "编辑任务" : "批量编辑" }}</h2>
          </div>
          <button type="button" class="soft-button compact" @click="closeModal">关闭</button>
        </header>

        <p class="manage-drawer-copy">
          {{ singleSelectedTask ? "单条任务可以修改标题、星级和时间。" : `已选 ${selectedCount} 条，批量编辑支持标题前后缀和星级。` }}
        </p>

        <div class="manage-drawer-grid">
          <label v-if="singleSelectedTask" class="field wide">
            <span>标题</span>
            <input v-model="editDraft.replaceTitle" type="text" />
          </label>
          <template v-else>
            <label class="field">
              <span>标题前缀</span>
              <input v-model="editDraft.prefix" type="text" />
            </label>
            <label class="field">
              <span>标题后缀</span>
              <input v-model="editDraft.suffix" type="text" />
            </label>
          </template>
          <label class="field">
            <span>重要等级</span>
            <div class="star-rating account-star-rating" aria-label="修改星级">
              <template v-for="value in ['5', '4', '3', '2', '1']" :key="value">
                <input :id="`manage-importance-${value}`" type="radio" :checked="editDraft.importance === value" />
                <label :for="`manage-importance-${value}`" @click.prevent="setEditImportance(value)">★</label>
              </template>
            </div>
          </label>
          <label v-if="singleSelectedTask?.schedule_mode === 'date'" class="field">
            <span>{{ scheduleLabel(singleSelectedTask) }}</span>
            <WheelDatePicker v-model="editDraft.scheduleDate" empty-label="选择日期" />
          </label>
          <label v-if="singleSelectedTask?.schedule_mode === 'datetime'" class="field">
            <span>{{ scheduleLabel(singleSelectedTask) }}</span>
            <WheelDatePicker v-model="editDraft.deadlineValue" mode="datetime" empty-label="截止时间" />
          </label>
        </div>

        <section v-if="selectedHasSharing" class="unshare-panel">
          <div>
            <p class="eyebrow">共享</p>
            <h3>取消共享</h3>
          </div>
          <p v-if="hasSharedWithMeSelection" class="manage-drawer-copy">
            共享给你的任务会从你的列表移除。
          </p>
          <div v-if="unshareUsers.length" class="share-list unshare-list">
            <label v-for="user in unshareUsers" :key="user.id" class="share-user">
              <input type="checkbox" :checked="unshareSelection.has(user.id)"
                @change="toggleUnshareUser(user, ($event.target as HTMLInputElement).checked)" />
              <span>
                <strong>{{ user.display_name }}</strong>
                <small>{{ user.email }}</small>
              </span>
            </label>
          </div>
          <button class="danger-button unshare-button" type="button" :disabled="!canSubmitUnshare || Boolean(loadingMessage)" @click="submitUnshare">
            取消共享
          </button>
        </section>

        <footer class="manage-modal-actions">
          <button class="soft-button" type="button" @click="closeModal">取消</button>
          <button class="primary-button" type="button" :disabled="Boolean(loadingMessage)" @click="submitEdit">
            {{ loadingMessage || "保存" }}
          </button>
        </footer>
      </section>

      <section v-if="activeModal === 'share'" class="manage-drawer">
        <header class="manage-drawer-head">
          <div>
            <p class="eyebrow">共享</p>
            <h2>共享任务</h2>
          </div>
          <button type="button" class="soft-button compact" @click="closeModal">关闭</button>
        </header>

        <label class="field wide share-search-field">
          <span>共享给好友</span>
          <input v-model="shareQuery" type="search" placeholder="筛选好友" autocomplete="off" />
        </label>
        <div class="share-list">
          <label v-for="user in filteredShareUsers" :key="user.id" class="share-user">
            <input type="checkbox" :checked="shareSelection.has(user.id)"
              @change="toggleShareUser(user, ($event.target as HTMLInputElement).checked)" />
            <span>
              <strong>{{ user.display_name }}</strong>
              <small>{{ user.email }}</small>
            </span>
          </label>
          <p v-if="filteredShareUsers.length === 0" class="share-empty">没有符合条件的好友</p>
        </div>

        <footer class="manage-modal-actions">
          <button class="soft-button" type="button" @click="closeModal">取消</button>
          <button class="primary-button" type="button" :disabled="shareSelection.size === 0 || Boolean(loadingMessage)"
            @click="submitShare">
            {{ loadingMessage || "共享" }}
          </button>
        </footer>
      </section>
    </div>

    <div v-if="loadingMessage" class="sync-chip active">{{ loadingMessage }}</div>
  </main>
</template>
