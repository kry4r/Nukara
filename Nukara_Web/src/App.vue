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
  max-width: 480px;
  margin: 0 auto;
  background: #fff;
}
</style>
