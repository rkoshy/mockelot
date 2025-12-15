<script lang="ts" setup>
import { onMounted } from 'vue'
import { useServerStore } from './stores/server'
import HeaderBar from './components/layout/HeaderBar.vue'
import ServerConfigPanel from './components/server/ServerConfigPanel.vue'

const serverStore = useServerStore()

onMounted(() => {
  serverStore.initEventListeners()
  serverStore.refreshStatus()
})
</script>

<template>
  <div class="h-screen flex flex-col bg-gray-900 text-gray-100">
    <!-- Header -->
    <HeaderBar />

    <!-- Main Content Area -->
    <div class="flex-1 overflow-hidden">
      <ServerConfigPanel />
    </div>
  </div>
</template>
