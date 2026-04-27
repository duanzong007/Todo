<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import type { AccountFilterOption } from "../types";

const props = withDefaults(
  defineProps<{
    modelValue: string;
    options: AccountFilterOption[];
    centerMenu?: boolean;
    compact?: boolean;
  }>(),
  {
    centerMenu: false,
    compact: false,
  },
);

const emit = defineEmits<{
  "update:modelValue": [value: string];
  change: [value: string];
}>();

const root = ref<HTMLElement | null>(null);
const isOpen = ref(false);

const selectedLabel = computed(() => {
  return props.options.find((option) => option.value === props.modelValue)?.label ?? props.options[0]?.label ?? "";
});

function selectOption(value: string) {
  if (value !== props.modelValue) {
    emit("update:modelValue", value);
    emit("change", value);
  }
  isOpen.value = false;
}

function onDocumentPointerDown(event: PointerEvent) {
  if (!root.value || !(event.target instanceof Node) || root.value.contains(event.target)) {
    return;
  }
  isOpen.value = false;
}

function onDocumentKeyDown(event: KeyboardEvent) {
  if (event.key === "Escape") {
    isOpen.value = false;
  }
}

onMounted(() => {
  document.addEventListener("pointerdown", onDocumentPointerDown);
  document.addEventListener("keydown", onDocumentKeyDown);
});

onBeforeUnmount(() => {
  document.removeEventListener("pointerdown", onDocumentPointerDown);
  document.removeEventListener("keydown", onDocumentKeyDown);
});
</script>

<template>
  <div
    ref="root"
    class="account-select"
    :class="{ 'is-open': isOpen, 'account-select-center-menu': centerMenu, 'is-compact': compact }"
  >
    <button type="button" class="account-select-trigger" @click="isOpen = !isOpen">
      <span class="account-select-label">{{ selectedLabel }}</span>
      <span class="account-select-caret" aria-hidden="true"></span>
    </button>
    <div v-if="isOpen" class="account-select-menu">
      <button
        v-for="option in options"
        :key="option.value"
        type="button"
        class="account-select-option"
        :class="{ 'is-selected': option.value === modelValue }"
        :aria-pressed="option.value === modelValue"
        @click="selectOption(option.value)"
      >
        {{ option.label }}
      </button>
    </div>
  </div>
</template>
