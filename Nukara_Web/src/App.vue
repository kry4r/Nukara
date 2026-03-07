<script setup>
import { useRoute } from 'vue-router'
import NavBar from './components/NavBar.vue'

const route = useRoute()
const showNav = () => !route.path.startsWith('/auth') && !route.path.startsWith('/chat/')
</script>

<template>
  <div id="stage">
    <div class="phone-shell">
      <div id="app-root">
        <router-view />
        <NavBar v-if="showNav()" />
      </div>
    </div>
  </div>
</template>
