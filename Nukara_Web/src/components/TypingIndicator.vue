<template>
  <div class="typing">
    <span class="dot"></span>
    <span class="dot"></span>
    <span class="dot"></span>
  </div>
</template>

<style scoped>
.typing {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 10px 16px;
  background: #f0f0f0;
  border-radius: 16px;
  margin: 8px 0;
}
.dot {
  width: 8px; height: 8px;
  background: #999;
  border-radius: 50%;
  animation: bounce 1.4s infinite ease-in-out both;
}
.dot:nth-child(1) { animation-delay: -0.32s; }
.dot:nth-child(2) { animation-delay: -0.16s; }
@keyframes bounce {
  0%, 80%, 100% { transform: scale(0.6); opacity: 0.4; }
  40% { transform: scale(1); opacity: 1; }
}
</style>
