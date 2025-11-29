<script lang="ts" setup>
import { onMounted } from 'vue'
import { useServerStore } from './stores/server'
import HeaderBar from './components/layout/HeaderBar.vue'
import ServerConfigPanel from './components/server/ServerConfigPanel.vue'
import TrafficLogPanel from './components/traffic/TrafficLogPanel.vue'
import InspectorPanel from './components/inspector/InspectorPanel.vue'
import ResizablePanels from './components/shared/ResizablePanels.vue'

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
    <div class="flex-1 flex overflow-hidden">
      <!-- Left Panel: Server Config (fixed width, wider now) -->
      <div class="w-[420px] flex-shrink-0 border-r border-gray-700 overflow-y-auto">
        <ServerConfigPanel />
      </div>

      <!-- Right Area: Traffic Log + Inspector (resizable) -->
      <div class="flex-1 min-w-0 overflow-hidden">
        <ResizablePanels :initial-left-width="550" :min-left-width="300" :min-right-width="280">
          <template #left>
            <TrafficLogPanel />
          </template>
          <template #right>
            <InspectorPanel />
          </template>
        </ResizablePanels>
      </div>
    </div>
  </div>
</template>
