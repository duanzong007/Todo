<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { APIError, fetchDashboardPage, openDashboardEvents, submitFormAction } from "../api/client";
import type { CompletedTaskCard, DashboardPageData, TaskCard } from "../types";
import WheelDatePicker from "./WheelDatePicker.vue";

type ComposerTab = "todo" | "schedule" | "ddl" | "sms";
type ScheduleMode = "single" | "batch";

const page = ref<DashboardPageData | null>(null);
const loading = ref(true);
const errorMessage = ref("");
const pendingCount = ref(0);
const moreOpen = ref(false);
const composerOpen = ref(false);
const composerTab = ref<ComposerTab>("todo");
const composerModal = ref<ComposerTab | "">("");
const scheduleMode = ref<ScheduleMode>("single");
const postponeOpen = ref("");
const postponeDrafts = reactive<Record<string, string>>({});
const editTaskID = ref("");
const editTitle = ref("");
const editImportance = ref("2");
const editOriginalTitle = ref("");
const editOriginalImportance = ref("2");
const dateJumpValue = ref("");
const icsInput = ref<HTMLInputElement | null>(null);

const forms = reactive({
  todoTitle: "",
  todoImportance: "2",
  scheduleTitle: "",
  scheduleImportance: "2",
  scheduleDate: "",
  batchStart: "",
  batchEnd: "",
  batchWeekdays: [] as string[],
  ddlTitle: "",
  ddlImportance: "2",
  ddlValue: "",
  smsInput: "",
});

let eventStream: EventSource | null = null;
let syncTimer = 0;
let popStateHandler: (() => void) | null = null;
let suppressRealtimeUntil = 0;
let editPointerDownHandler: ((event: PointerEvent) => void) | null = null;
let postponePointerDownHandler: ((event: PointerEvent) => void) | null = null;

const focusTasks = computed(() => page.value?.focus_tasks ?? []);
const completedTasks = computed(() => page.value?.completed_tasks ?? []);
const focusDate = computed(() => page.value?.focus_date_iso ?? "");
const todayDate = computed(() => page.value?.today_date_iso ?? "");
const tomorrowDate = computed(() => page.value?.tomorrow_date_iso ?? "");
const dayAfterDate = computed(() => page.value?.day_after_date_iso ?? "");

async function load(search = window.location.search) {
  loading.value = !page.value;
  errorMessage.value = "";
  try {
    page.value = await fetchDashboardPage(search);
    dateJumpValue.value = page.value.focus_date_iso;
    if (!forms.scheduleDate) forms.scheduleDate = page.value.today_date_iso;
    if (!forms.batchStart) forms.batchStart = page.value.today_date_iso;
    if (!forms.batchEnd) forms.batchEnd = page.value.today_date_iso;
    if (!forms.ddlValue) forms.ddlValue = `${page.value.today_date_iso}T08:00`;
  } catch (error) {
    if (error instanceof APIError && error.status === 401) {
      errorMessage.value = "当前浏览器没有可用登录态。先在 Go 站点登录，再回到这里刷新。";
    } else {
      errorMessage.value = error instanceof Error ? error.message : "首页数据加载失败";
    }
  } finally {
    loading.value = false;
  }
}

function searchForDate(date: string) {
  if (!date || date === todayDate.value) {
    return "";
  }
  return `?date=${encodeURIComponent(date)}`;
}

function navigateDate(date: string) {
  const search = searchForDate(date);
  window.history.pushState(null, "", `/${search}`);
  void load(window.location.search);
}

function navigatePath(path = "/") {
  window.history.pushState(null, "", path || "/");
  void load(window.location.search);
}

function refresh() {
  void load(window.location.search);
}

function scheduleReload() {
  if (Date.now() < suppressRealtimeUntil) {
    return;
  }
  window.clearTimeout(syncTimer);
  syncTimer = window.setTimeout(() => {
    void load(window.location.search);
  }, 180);
}

function connectEvents() {
  if (eventStream) {
    eventStream.close();
  }
  try {
    eventStream = openDashboardEvents(scheduleReload);
  } catch (_error) {
    // Realtime is opportunistic; manual refresh still works.
  }
}

async function mutate(path: string, formData?: FormData, options: { reload?: boolean; suppressRealtime?: boolean } = {}) {
  pendingCount.value += 1;
  if (options.suppressRealtime) {
    suppressRealtimeUntil = Date.now() + 1200;
  }
  try {
    await submitFormAction(path, formData);
    if (options.reload !== false) {
      await load(window.location.search);
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "操作失败";
    await load(window.location.search);
  } finally {
    pendingCount.value = Math.max(0, pendingCount.value - 1);
  }
}

function taskFormData(task: TaskCard | CompletedTaskCard) {
  const formData = new FormData();
  formData.set("return_date", focusDate.value || task.return_date || "");
  return formData;
}

function completeTask(task: TaskCard) {
  optimisticallyComplete(task);
  void mutate(`/tasks/${task.id}/complete`, taskFormData(task), { reload: false, suppressRealtime: true });
}

function restoreTask(task: CompletedTaskCard) {
  optimisticallyRestore(task);
  void mutate(`/tasks/${task.id}/restore`, taskFormData(task), { reload: false, suppressRealtime: true });
}

function optimisticallyComplete(task: TaskCard) {
  if (!page.value) return;
  page.value.focus_tasks = page.value.focus_tasks.filter((item) => item.id !== task.id);
  if (!page.value.completed_tasks.some((item) => item.id === task.id)) {
    page.value.completed_tasks = [
      {
        id: task.id,
        title: task.title,
        kind_label: task.kind_label,
        kind_class: task.kind_class,
        importance: task.importance,
        finished_line: "刚刚完成",
        status_line: task.status_line,
        note: task.note,
        can_postpone: task.can_postpone,
        postpone_mode: task.postpone_mode,
        postpone_value: task.postpone_value,
        postpone_min_value: task.postpone_min_value,
        return_date: task.return_date,
      },
      ...page.value.completed_tasks,
    ];
  }
}

function optimisticallyRestore(task: CompletedTaskCard) {
  if (!page.value) return;
  page.value.completed_tasks = page.value.completed_tasks.filter((item) => item.id !== task.id);
  if (!page.value.focus_tasks.some((item) => item.id === task.id)) {
    page.value.focus_tasks = [
      {
        id: task.id,
        title: task.title,
        kind_label: task.kind_label,
        kind_class: task.kind_class,
        importance: task.importance,
        status_line: task.status_line,
        compact_status: "",
        mobile_compact: false,
        note: task.note,
        can_complete: true,
        can_postpone: task.can_postpone,
        postpone_mode: task.postpone_mode,
        postpone_value: task.postpone_value,
        postpone_min_value: task.postpone_min_value,
        return_date: task.return_date,
      },
      ...page.value.focus_tasks,
    ];
  }
}

function postponeTask(task: TaskCard | CompletedTaskCard, value: string) {
  if (!value) {
    return;
  }
  const formData = taskFormData(task);
  formData.set("target_value", value);
  postponeOpen.value = "";
  const remainsInView = optimisticallyPostpone(task, value);
  void mutate(`/tasks/${task.id}/postpone`, formData, { reload: remainsInView, suppressRealtime: !remainsInView });
}

function optimisticallyPostpone(task: TaskCard | CompletedTaskCard, value: string) {
  if (!page.value) return true;
  const targetDate = value.split("T")[0];
  if (targetDate && targetDate !== focusDate.value) {
    page.value.focus_tasks = page.value.focus_tasks.filter((item) => item.id !== task.id);
    return false;
  }
  const nextTasks = page.value.focus_tasks.map((item) =>
    item.id === task.id
      ? {
          ...item,
          postpone_value: value,
        }
      : item,
  );
  page.value.focus_tasks = nextTasks;
  return true;
}

function setPostponeOpen(task: TaskCard | CompletedTaskCard, event: Event) {
  const details = event.target instanceof HTMLDetailsElement ? event.target : null;
  if (!details?.open) {
    delete postponeDrafts[task.id];
    postponeOpen.value = "";
    return;
  }
  postponeOpen.value = task.id;
  postponeDrafts[task.id] = task.postpone_value;
}

function closePostponeWithoutCommit() {
  if (!postponeOpen.value) return;
  delete postponeDrafts[postponeOpen.value];
  postponeOpen.value = "";
}

function onPostponePointerDown(event: PointerEvent) {
  if (!postponeOpen.value) return;
  if (event.target instanceof Element && event.target.closest(".inline-postpone-vue")) return;
  closePostponeWithoutCommit();
}

function startEdit(task: TaskCard | CompletedTaskCard) {
  editTaskID.value = task.id;
  editTitle.value = task.title;
  editImportance.value = String(task.importance || 2);
  editOriginalTitle.value = task.title;
  editOriginalImportance.value = String(task.importance || 2);
}

async function commitEdit(task: TaskCard | CompletedTaskCard) {
  const title = editTitle.value.trim();
  if (!title) {
    cancelEdit();
    return;
  }
  const importance = editImportance.value || editOriginalImportance.value;
  if (title === editOriginalTitle.value && importance === editOriginalImportance.value) {
    cancelEdit();
    return;
  }
  const formData = new FormData();
  formData.set("title", title);
  formData.set("importance", importance);
  cancelEdit();
  await mutate(`/tasks/${task.id}/rename`, formData);
}

function setEditImportance(value: string) {
  editImportance.value = value;
}

function cancelEdit() {
  editTaskID.value = "";
  editTitle.value = "";
  editImportance.value = "2";
  editOriginalTitle.value = "";
  editOriginalImportance.value = "2";
}

function onEditPointerDown(event: PointerEvent) {
  if (!editTaskID.value || !page.value) return;
  if (event.target instanceof Element && event.target.closest(".inline-title-editor")) return;
  const activeTask = [...page.value.focus_tasks, ...page.value.completed_tasks].find((task) => task.id === editTaskID.value);
  if (activeTask) {
    void commitEdit(activeTask);
  } else {
    cancelEdit();
  }
}

function kindClass(task: TaskCard | CompletedTaskCard) {
  return {
    todo: task.kind_class === "todo",
    schedule: task.kind_class === "schedule",
    ddl: task.kind_class === "ddl",
  };
}

function quickSetDate(target: "schedule" | "batchStart" | "batchEnd" | "ddl", date: string) {
  if (target === "ddl") {
    const time = forms.ddlValue.includes("T") ? forms.ddlValue.split("T")[1] : "08:00";
    forms.ddlValue = `${date}T${time}`;
    return;
  }
  if (target === "schedule") {
    forms.scheduleDate = date;
    return;
  }
  forms[target] = date;
}

function baseManualForm(type: string, title: string, importance: string) {
  const formData = new FormData();
  formData.set("return_date", focusDate.value);
  formData.set("task_type", type);
  formData.set("title", title.trim());
  formData.set("importance", importance || "2");
  return formData;
}

async function submitTodo() {
  if (!forms.todoTitle.trim()) return;
  const formData = baseManualForm("todo", forms.todoTitle, forms.todoImportance);
  await mutate("/tasks/manual", formData);
  forms.todoTitle = "";
}

async function submitSchedule() {
  if (!forms.scheduleTitle.trim()) return;
  const formData = baseManualForm("schedule", forms.scheduleTitle, forms.scheduleImportance);
  formData.set("schedule_mode", scheduleMode.value);
  if (scheduleMode.value === "batch") {
    formData.set("batch_start_value", forms.batchStart);
    formData.set("batch_end_value", forms.batchEnd);
    forms.batchWeekdays.forEach((weekday) => formData.append("batch_weekdays", weekday));
  } else {
    formData.set("scheduled_value", forms.scheduleDate);
  }
  await mutate("/tasks/manual", formData);
  forms.scheduleTitle = "";
}

async function submitDDL() {
  if (!forms.ddlTitle.trim()) return;
  const formData = baseManualForm("ddl", forms.ddlTitle, forms.ddlImportance);
  formData.set("deadline_value", forms.ddlValue);
  await mutate("/tasks/manual", formData);
  forms.ddlTitle = "";
}

async function submitSMS() {
  if (!forms.smsInput.trim()) return;
  const formData = new FormData();
  formData.set("return_date", focusDate.value);
  formData.set("sms_input", forms.smsInput.trim());
  await mutate("/tasks/parse-sms", formData);
  forms.smsInput = "";
}

function openSMS() {
  const hasNative = Boolean((window as unknown as { Capacitor?: { Plugins?: { SmsBridge?: unknown } } }).Capacitor?.Plugins?.SmsBridge);
  if (hasNative) {
    window.location.assign(`/sms/native?return=${encodeURIComponent(`${window.location.pathname}${window.location.search}`)}`);
    return;
  }
  openComposerModal("sms");
}

function openComposerModal(tab: ComposerTab) {
  composerTab.value = tab;
  composerModal.value = tab;
  composerOpen.value = false;
}

function closeComposerModal() {
  composerModal.value = "";
}

function openICS() {
  icsInput.value?.click();
}

async function importICS(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  const formData = new FormData();
  formData.set("return_date", focusDate.value);
  formData.set("ics_file", file);
  await mutate("/imports/ics", formData);
  input.value = "";
}

onMounted(() => {
  void load();
  connectEvents();
  popStateHandler = () => void load(window.location.search);
  editPointerDownHandler = onEditPointerDown;
  postponePointerDownHandler = onPostponePointerDown;
  document.addEventListener("pointerdown", postponePointerDownHandler);
  document.addEventListener("pointerdown", editPointerDownHandler);
  window.addEventListener("popstate", popStateHandler);
});

onBeforeUnmount(() => {
  window.clearTimeout(syncTimer);
  if (editPointerDownHandler) {
    document.removeEventListener("pointerdown", editPointerDownHandler);
  }
  if (postponePointerDownHandler) {
    document.removeEventListener("pointerdown", postponePointerDownHandler);
  }
  if (popStateHandler) {
    window.removeEventListener("popstate", popStateHandler);
  }
  if (eventStream) eventStream.close();
});
</script>

<template>
  <main class="focus-page-vue">
    <section v-if="page" class="focus-hero-vue">
      <a href="/me" class="focus-user-link">{{ page.current_user.display_name }}</a>
      <button type="button" class="focus-title-reload" @click="refresh">
        <Transition name="title-swap" mode="out-in">
          <h1 :key="page.focus_title">{{ page.focus_title }}</h1>
        </Transition>
      </button>
    </section>

    <p v-if="errorMessage" class="inline-error">{{ errorMessage }}</p>

    <section class="focus-panel-vue">
      <div class="focus-panel-head-vue">
        <Transition name="weekday-swap" mode="out-in">
        <p :key="`${focusDate}-weekday`" class="section-kicker focus-weekday">
          {{ page?.focus_weekday_label }}
          <template v-for="mark in page?.focus_day_marks ?? []" :key="mark"> · {{ mark }}</template>
        </p>
        </Transition>
        <span class="focus-counter" :class="{ 'is-pending': pendingCount > 0 }">
          <Transition name="count-swap" mode="out-in">
            <span :key="focusTasks.length">{{ focusTasks.length }}</span>
          </Transition>
        </span>
      </div>

      <Transition name="focus-empty-fade" mode="out-in">
        <div v-if="loading" key="loading" class="focus-empty">正在加载</div>

        <TransitionGroup v-else-if="focusTasks.length" :key="`tasks-${focusDate}`" name="task-flow" tag="div" class="focus-list-vue">
        <article
          v-for="task in focusTasks"
          :key="task.id"
          class="focus-card-vue"
          :class="{ 'is-mobile-ddl-ready': task.kind_class === 'ddl', 'is-mobile-ddl-compact': task.mobile_compact }"
          :data-kind-class="task.kind_class"
        >
          <span class="task-importance-badge">{{ task.importance }}</span>
          <div class="focus-card-main-vue">
            <div class="task-kind-stack">
              <span class="task-kind" :class="kindClass(task)" @dblclick="startEdit(task)">{{ task.kind_label }}</span>
              <span v-if="task.compact_status" class="task-status-mobile">{{ task.compact_status }}</span>
            </div>
            <div class="task-body-vue">
              <template v-if="editTaskID === task.id">
                <div class="inline-title-editor">
                  <input v-model="editTitle" type="text" @keydown.enter.prevent="commitEdit(task)" @keydown.esc.prevent="cancelEdit" />
                  <div class="star-rating inline-edit-stars" @pointerdown.stop @click.stop>
                    <template v-for="value in ['5', '4', '3', '2', '1']" :key="value">
                      <input :id="`edit-${task.id}-${value}`" type="radio" :checked="editImportance === value" />
                      <label :for="`edit-${task.id}-${value}`" @pointerdown.prevent @click.prevent="setEditImportance(value)">★</label>
                    </template>
                  </div>
                </div>
              </template>
              <h3 v-else>{{ task.title }}</h3>
              <p v-if="task.status_line" class="status">{{ task.status_line }}</p>
              <p v-if="task.note" class="note">{{ task.note }}</p>
            </div>
          </div>

          <div class="focus-actions-vue">
            <details v-if="task.can_postpone" class="inline-postpone-vue" :open="postponeOpen === task.id" @toggle="setPostponeOpen(task, $event)">
              <summary>延期</summary>
              <div class="postpone-form-panel-vue">
                <WheelDatePicker
                  :model-value="postponeDrafts[task.id] || task.postpone_value"
                  :mode="task.postpone_mode"
                  @update:model-value="postponeDrafts[task.id] = $event"
                />
                <button type="button" class="secondary" @click="postponeTask(task, postponeDrafts[task.id] || task.postpone_value)">确定</button>
              </div>
            </details>
            <button v-if="task.can_complete" type="button" class="complete-toggle" @click="completeTask(task)">
              <span class="visually-hidden">确认完成</span>
            </button>
          </div>
        </article>
        </TransitionGroup>

        <div v-else :key="`empty-${focusDate}`" class="focus-empty">
        <div v-if="page?.empty_quote" class="empty-quote-block">
          <p class="empty-quote">{{ page.empty_quote.text }}</p>
          <p v-if="page.empty_quote.has_meta" class="empty-quote-meta">{{ page.empty_quote.meta_line }}</p>
        </div>
        </div>
      </Transition>
    </section>

    <section class="focus-drawers-vue">
      <section class="utility-drawer-vue" :class="{ open: moreOpen }">
        <button type="button" class="utility-drawer-summary" @click="moreOpen = !moreOpen">更多</button>
        <Transition name="drawer-expand">
        <div v-if="moreOpen" class="drawer-body-vue">
          <div class="quick-links">
            <button type="button" class="date-chip" @click="navigatePath(page?.yesterday_path)">昨天</button>
            <button type="button" class="date-chip" @click="navigatePath(page?.today_path)">今天</button>
            <button type="button" class="date-chip" @click="navigatePath(page?.tomorrow_path)">明天</button>
            <button type="button" class="date-chip" @click="navigatePath(page?.day_after_tomorrow_path)">后天</button>
          </div>
          <div class="mini-date-form">
            <WheelDatePicker v-model="dateJumpValue" />
            <button type="button" class="secondary" @click="navigateDate(dateJumpValue)">确定</button>
          </div>

          <div class="archive-section-vue" :class="{ 'is-empty': completedTasks.length === 0 }">
            <div class="archive-head-vue">
              <p>已完成</p>
              <span>
                <Transition name="count-swap" mode="out-in">
                  <span :key="completedTasks.length">{{ completedTasks.length }}</span>
                </Transition>
              </span>
            </div>
            <TransitionGroup name="task-flow" tag="div" class="archive-list-vue">
              <article v-for="task in completedTasks" :key="task.id" class="archive-card-vue">
                <div class="archive-card-main-vue">
                  <span class="task-kind" :class="kindClass(task)" @dblclick="startEdit(task)">{{ task.kind_label }}</span>
                  <div class="task-body-vue">
                    <h3>{{ task.title }}</h3>
                    <p class="status">{{ task.finished_line }}</p>
                    <p v-if="task.note" class="note">{{ task.note }}</p>
                  </div>
                  <button type="button" class="secondary archive-restore" @click="restoreTask(task)">撤销</button>
                </div>
              </article>
            </TransitionGroup>
          </div>
        </div>
        </Transition>
      </section>
    </section>

    <div class="composer-fab-vue" :class="{ open: composerOpen }">
      <button type="button" class="composer-fab-button" @click.stop="composerOpen = !composerOpen">{{ composerOpen ? "×" : "+" }}</button>
      <Transition name="composer-pop">
      <div v-if="composerOpen" class="composer-panel-vue">
        <div class="composer-choice-grid">
          <button type="button" class="composer-choice-button" @click="openComposerModal('todo')">Todo</button>
          <button type="button" class="composer-choice-button" @click="openComposerModal('schedule')">日程</button>
          <button type="button" class="composer-choice-button" @click="openComposerModal('ddl')">DDL</button>
          <button type="button" class="composer-choice-button" @click="openSMS">短信</button>
          <button type="button" class="composer-choice-button composer-choice-ics" @click="openICS">ICS</button>
          <input ref="icsInput" type="file" accept=".ics,text/calendar" hidden @change="importICS" />
        </div>
      </div>
      </Transition>
    </div>

    <Transition name="composer-modal">
    <div v-if="composerModal" class="composer-modal-shell" role="dialog" aria-modal="true">
      <div class="composer-modal-backdrop"></div>
      <section class="composer-modal-card">
        <header class="composer-modal-head">
          <div>
            <p class="eyebrow">新建</p>
            <h2>{{ composerModal === 'todo' ? 'Todo' : composerModal === 'schedule' ? '日程' : composerModal === 'ddl' ? 'DDL' : '短信' }}</h2>
          </div>
          <button type="button" class="composer-modal-close" @click="closeComposerModal">关闭</button>
        </header>

        <form v-if="composerModal === 'todo'" class="composer-form-vue" @submit.prevent="submitTodo">
          <label>标题<input v-model="forms.todoTitle" type="text" placeholder="例如：买电池" required /></label>
          <label>重要等级</label>
          <div class="star-rating composer-stars">
            <template v-for="value in ['5', '4', '3', '2', '1']" :key="value"><input :id="`todo-${value}`" type="radio" :checked="forms.todoImportance === value" /><label :for="`todo-${value}`" @click.prevent="forms.todoImportance = value">★</label></template>
          </div>
          <button type="submit">添加 Todo</button>
        </form>

        <form v-if="composerModal === 'schedule'" class="composer-form-vue" @submit.prevent="submitSchedule">
          <label>标题<input v-model="forms.scheduleTitle" type="text" placeholder="例如：上课" required /></label>
          <div class="schedule-mode-tabs"><button type="button" :class="{ active: scheduleMode === 'single' }" @click="scheduleMode = 'single'">单次</button><button type="button" :class="{ active: scheduleMode === 'batch' }" @click="scheduleMode = 'batch'">批量</button></div>
          <div v-if="scheduleMode === 'single'" class="composer-shortcuts"><button type="button" @click="quickSetDate('schedule', todayDate)">今天</button><button type="button" @click="quickSetDate('schedule', tomorrowDate)">明天</button><button type="button" @click="quickSetDate('schedule', dayAfterDate)">后天</button></div>
          <WheelDatePicker v-if="scheduleMode === 'single'" v-model="forms.scheduleDate" />
          <div v-else class="batch-box">
            <label>起始日期<WheelDatePicker v-model="forms.batchStart" /></label>
            <label>截止日期<WheelDatePicker v-model="forms.batchEnd" /></label>
            <div class="weekday-picker-vue">
              <label v-for="[value, label] in [['mon','周一'],['tue','周二'],['wed','周三'],['thu','周四'],['fri','周五'],['sat','周六'],['sun','周日']]" :key="value" :class="{ active: forms.batchWeekdays.includes(value) }"><input v-model="forms.batchWeekdays" type="checkbox" :value="value" />{{ label }}</label>
            </div>
          </div>
          <label>重要等级</label><div class="star-rating composer-stars"><template v-for="value in ['5','4','3','2','1']" :key="value"><input :id="`schedule-${value}`" type="radio" :checked="forms.scheduleImportance === value" /><label :for="`schedule-${value}`" @click.prevent="forms.scheduleImportance = value">★</label></template></div>
          <button type="submit">添加日程</button>
        </form>

        <form v-if="composerModal === 'ddl'" class="composer-form-vue" @submit.prevent="submitDDL">
          <label>标题<input v-model="forms.ddlTitle" type="text" placeholder="例如：交作业" required /></label>
          <div class="composer-shortcuts"><button type="button" @click="quickSetDate('ddl', todayDate)">今天</button><button type="button" @click="quickSetDate('ddl', tomorrowDate)">明天</button><button type="button" @click="quickSetDate('ddl', dayAfterDate)">后天</button></div>
          <WheelDatePicker v-model="forms.ddlValue" mode="datetime" />
          <label>重要等级</label><div class="star-rating composer-stars"><template v-for="value in ['5','4','3','2','1']" :key="value"><input :id="`ddl-${value}`" type="radio" :checked="forms.ddlImportance === value" /><label :for="`ddl-${value}`" @click.prevent="forms.ddlImportance = value">★</label></template></div>
          <button type="submit">添加 DDL</button>
        </form>

        <form v-if="composerModal === 'sms'" class="composer-form-vue" @submit.prevent="submitSMS">
          <label>短信内容<textarea v-model="forms.smsInput" placeholder="直接粘贴取件短信；一次贴很多条也可以。" required></textarea></label>
          <button type="submit">解析短信</button>
        </form>
      </section>
    </div>
    </Transition>
  </main>
</template>
