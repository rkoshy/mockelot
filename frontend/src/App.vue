<script lang="ts" setup>
import { onMounted, ref, provide } from 'vue'
import { useServerStore } from './stores/server'
import HeaderBar from './components/layout/HeaderBar.vue'
import ServerConfigPanel from './components/server/ServerConfigPanel.vue'
import LoadEndpointsDialog from './components/dialogs/LoadEndpointsDialog.vue'

const serverStore = useServerStore()
const showLoadDialog = ref(false)

// Provide method to child components to show the load dialog
provide('showLoadDialog', () => {
  showLoadDialog.value = true
})

onMounted(() => {
  serverStore.initEventListeners()
  serverStore.refreshStatus()

  // Auto-show load dialog on startup
  showLoadDialog.value = true
})

async function handleLoadDialogLoaded() {
  showLoadDialog.value = false
  // Refresh server store after config is loaded
  await serverStore.refreshStatus()
  await serverStore.refreshItems()
}

function handleLoadDialogClose() {
  showLoadDialog.value = false
}
</script>

<template>
  <div class="h-screen flex flex-col bg-gray-900 text-gray-100">
    <!-- Header -->
    <HeaderBar />

    <!-- Main Content Area -->
    <div class="flex-1 overflow-hidden">
      <ServerConfigPanel />
    </div>

    <!-- Load Endpoints Dialog - Auto-shown on startup -->
    <LoadEndpointsDialog
      :show="showLoadDialog"
      @close="handleLoadDialogClose"
      @loaded="handleLoadDialogLoaded"
    />
  </div>
</template>
