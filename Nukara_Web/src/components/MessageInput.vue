<script setup>
import { ref } from 'vue'

const emit = defineEmits(['send'])
const text = ref('')

function handleSend() {
  if (!text.value.trim()) return
  emit('send', text.value.trim())
  text.value = ''
}
</script>

<template>
  <div class="input-bar">
    <input
      v-model="text"
      type="text"
      placeholder="说点什么..."
      @keydown.enter="handleSend"
    />
    <button :disabled="!text.trim()" @click="handleSend">发送</button>
  </div>
</template>

<style scoped>
.input-bar {
  display: flex;
  gap: 8px;
  padding: 10px 16px;
  border-top: 0.5px solid #e5e5e5;
  background: #fff;
  padding-bottom: calc(10px + env(safe-area-inset-bottom, 0));
}
.input-bar input {
  flex: 1;
  padding: 10px 16px;
  border: 1px solid #e5e5e5;
  border-radius: 20px;
  font-size: 15px;
  outline: none;
}
.input-bar input:focus { border-color: #007aff; }
.input-bar button {
  padding: 10px 20px;
  background: #007aff;
  color: #fff;
  border: none;
  border-radius: 20px;
  font-size: 15px;
}
.input-bar button:disabled { background: #ccc; }
</style>
