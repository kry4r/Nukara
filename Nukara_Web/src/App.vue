<script setup>
import { useRoute } from 'vue-router'
import NavBar from './components/NavBar.vue'

const route = useRoute()
const showNav = () => !route.path.startsWith('/auth') && !route.path.startsWith('/chat/')
</script>

<template>
  <div id="app-root">
    <router-view />
    <NavBar v-if="showNav()" />
  </div>
</template>

<style scoped>
#app-root {
  height: 100vh;
  display: flex;
  flex-direction: column;
  width: min(100%, 520px);
  margin: 0 auto;
  background: #fff;
  border-radius: 20px;
  overflow: hidden;
  box-shadow: 0 20px 50px rgba(80, 98, 62, 0.2);
}

@media (max-width: 640px) {
  #app-root {
    width: 100%;
    border-radius: 0;
    box-shadow: none;
  }
}
</style>
