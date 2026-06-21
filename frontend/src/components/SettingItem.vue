<script setup lang="ts">
defineProps<{
  category: string;
  title: string;
  description: string;
}>();
</script>

<template>
  <section class="setting-item">
    <div class="setting-item-copy">
      <span class="eyebrow">{{ category }}</span>
      <strong>{{ title }}</strong>
      <small>{{ description }}</small>
      <slot name="message"></slot>
    </div>
    <div class="setting-item-action">
      <slot></slot>
    </div>
  </section>
</template>
