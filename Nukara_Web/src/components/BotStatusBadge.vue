<script setup>
defineProps({
  emoji: { type: String, default: '' },
  text: { type: String, default: '' },
})
</script>

<template>
  <span v-if="emoji || text" class="status-badge">
    {{ emoji }} {{ text }}
  </span>
</template>

<style scoped>
.status-badge {
  font-size: 12px;
  color: #666;
  background: #f0f0f0;
  padding: 2px 8px;
  border-radius: 10px;
}
</style>
