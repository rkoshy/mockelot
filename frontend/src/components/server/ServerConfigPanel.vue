<script lang="ts" setup>
import { ref } from 'vue'
import { useServerStore } from '../../stores/server'
import ResponseRuleCard from './ResponseRuleCard.vue'
import ResponseGroupCard from './ResponseGroupCard.vue'
import { models } from '../../types/models'

const serverStore = useServerStore()

// Drag and drop state
const draggedIndex = ref<number | null>(null)
const dragOverIndex = ref<number | null>(null)

function onDragStart(index: number, event: DragEvent) {
  draggedIndex.value = index
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', String(index))
  }
}

function onDragOver(index: number, event: DragEvent) {
  event.preventDefault()
  dragOverIndex.value = index
}

function onDrop(index: number, event: DragEvent) {
  event.preventDefault()

  if (draggedIndex.value === null || draggedIndex.value === index) {
    draggedIndex.value = null
    dragOverIndex.value = null
    return
  }

  serverStore.reorderItems(draggedIndex.value, index)

  draggedIndex.value = null
  dragOverIndex.value = null
}

function onDragEnd() {
  draggedIndex.value = null
  dragOverIndex.value = null
}

function getItemId(item: models.ResponseItem): string {
  return item.type === 'response' ? item.response?.id || '' : item.group?.id || ''
}

async function handleResponseUpdate(index: number, response: models.MethodResponse) {
  const item = new models.ResponseItem({
    type: 'response',
    response: response
  })
  await serverStore.updateItem(index, item)
}

async function handleGroupUpdate(index: number, group: models.ResponseGroup) {
  const item = new models.ResponseItem({
    type: 'group',
    group: group
  })
  await serverStore.updateItem(index, item)
}

async function handleDelete(index: number) {
  await serverStore.removeItem(index)
}
</script>

<template>
  <div class="h-full flex flex-col">
    <!-- Header -->
    <div class="flex items-center justify-between p-3 border-b border-gray-700 flex-shrink-0">
      <h2 class="text-lg font-semibold text-white">Response Rules</h2>
      <div class="flex gap-2">
        <button
          @click="serverStore.addNewGroup"
          class="px-3 py-1 bg-blue-800 hover:bg-blue-700 rounded text-sm text-white font-medium flex items-center gap-1"
          title="Add a new group to organize responses"
        >
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
          </svg>
          Add Group
        </button>
        <button
          @click="serverStore.addNewResponse"
          class="px-3 py-1 bg-blue-600 hover:bg-blue-700 rounded text-sm text-white font-medium flex items-center gap-1"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          Add Rule
        </button>
      </div>
    </div>

    <!-- Info Banner -->
    <div class="px-3 py-2 bg-gray-800/50 border-b border-gray-700 flex-shrink-0">
      <p class="text-xs text-gray-400">
        Rules are checked in order. First matching rule wins. Drag to reorder. Use groups to organize related rules.
      </p>
    </div>

    <!-- Rules List -->
    <div class="flex-1 overflow-y-auto p-3 space-y-2" @dragend="onDragEnd">
      <!-- Empty State -->
      <div v-if="serverStore.items.length === 0" class="flex items-center justify-center h-32">
        <div class="text-center text-gray-500">
          <p class="text-sm">No response rules configured</p>
          <p class="text-xs mt-1">Click "Add Rule" or "Add Group" to get started</p>
        </div>
      </div>

      <!-- Items (Responses and Groups) -->
      <div
        v-for="(item, index) in serverStore.items"
        :key="getItemId(item)"
        :class="[
          'transition-all',
          dragOverIndex === index && draggedIndex !== index ? 'border-t-2 border-blue-500 pt-2' : ''
        ]"
      >
        <!-- Response Card -->
        <ResponseRuleCard
          v-if="item.type === 'response' && item.response"
          :response="item.response"
          :is-expanded="serverStore.expandedItemId === item.response.id"
          :index="index"
          @toggle="serverStore.toggleExpanded(item.response?.id || '')"
          @update="handleResponseUpdate(index, $event)"
          @delete="handleDelete(index)"
          @dragstart="onDragStart(index, $event)"
          @dragover="onDragOver(index, $event)"
          @drop="onDrop(index, $event)"
        />

        <!-- Group Card -->
        <ResponseGroupCard
          v-else-if="item.type === 'group' && item.group"
          :group="item.group"
          :index="index"
          @update="handleGroupUpdate(index, $event)"
          @delete="handleDelete(index)"
          @dragstart="onDragStart(index, $event)"
          @dragover="onDragOver(index, $event)"
          @drop="onDrop(index, $event)"
        />
      </div>
    </div>
  </div>
</template>
