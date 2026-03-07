<script setup>
import { onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import NavBar from './components/NavBar.vue'
import { useAuthStore } from './stores/auth'
import { useRealtimeStore } from './stores/realtime'

const route = useRoute()
const auth = useAuthStore()
const realtime = useRealtimeStore()
const showNav = () => !route.path.startsWith('/auth') && !route.path.startsWith('/chat/')

onMounted(() => {
  auth.restoreSession()
})

watch(() => auth.token, (token) => {
  realtime.connectWithToken(token)
}, { immediate: true })
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
