<script lang="ts" setup>
import { ref, computed } from 'vue'
import { models } from '../../types/models'
import ResponseRuleCard from './ResponseRuleCard.vue'

const props = defineProps<{
  group: models.ResponseGroup
  index: number
}>()

const emit = defineEmits<{
  (e: 'update', group: models.ResponseGroup): void
  (e: 'delete'): void
  (e: 'dragstart', event: DragEvent): void
  (e: 'dragover', event: DragEvent): void
  (e: 'drop', event: DragEvent): void
}>()

// Local state
const isEditing = ref(false)
const editName = ref('')
const cardRef = ref<HTMLElement | null>(null)
const isDragging = ref(false)

// Whether group is expanded (defaults to true)
const isExpanded = computed({
  get: () => props.group.expanded !== false,
  set: (value: boolean) => {
    const updated = new models.ResponseGroup({ ...props.group, expanded: value })
    emit('update', updated)
  }
})

// Whether group is enabled (defaults to true)
const isEnabled = computed({
  get: () => props.group.enabled !== false,
  set: (value: boolean) => {
    const updated = new models.ResponseGroup({ ...props.group, enabled: value })
    emit('update', updated)
  }
})

// Expanded response index within group
const expandedResponseIndex = ref<number | null>(null)

// Toggle response expansion
function toggleResponse(index: number) {
  expandedResponseIndex.value = expandedResponseIndex.value === index ? null : index
}

// Update a response within the group
function updateResponse(index: number, response: models.MethodResponse) {
  const responses = [...(props.group.responses || [])]
  responses[index] = response
  const updated = new models.ResponseGroup({ ...props.group, responses })
  emit('update', updated)
}

// Delete a response from the group
function deleteResponse(index: number) {
  const responses = [...(props.group.responses || [])]
  responses.splice(index, 1)
  const updated = new models.ResponseGroup({ ...props.group, responses })
  emit('update', updated)
}

// Add new response to group
function addResponse() {
  const responses = [...(props.group.responses || [])]
  const newResponse = new models.MethodResponse({
    id: crypto.randomUUID(),
    path_pattern: '/*',
    methods: ['GET'],
    status_code: 200,
    status_text: 'OK'
  })
  responses.push(newResponse)
  const updated = new models.ResponseGroup({ ...props.group, responses })
  emit('update', updated)
  expandedResponseIndex.value = responses.length - 1
}

// Start editing group name
function startEditing() {
  editName.value = props.group.name
  isEditing.value = true
}

// Save group name
function saveName() {
  if (editName.value.trim()) {
    const updated = new models.ResponseGroup({ ...props.group, name: editName.value.trim() })
    emit('update', updated)
  }
  isEditing.value = false
}

// Cancel editing
function cancelEdit() {
  isEditing.value = false
}

// Drag and drop for responses within group
let draggedResponseIndex: number | null = null

function onResponseDragStart(index: number, e: DragEvent) {
  draggedResponseIndex = index
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', `response:${index}`)
  }
}

function onResponseDragOver(index: number, e: DragEvent) {
  e.preventDefault()
  if (e.dataTransfer) {
    e.dataTransfer.dropEffect = 'move'
  }
}

function onResponseDrop(index: number, e: DragEvent) {
  e.preventDefault()
  if (draggedResponseIndex !== null && draggedResponseIndex !== index) {
    const responses = [...(props.group.responses || [])]
    const [removed] = responses.splice(draggedResponseIndex, 1)
    responses.splice(index, 0, removed)
    const updated = new models.ResponseGroup({ ...props.group, responses })
    emit('update', updated)

    // Update expanded index if needed
    if (expandedResponseIndex.value === draggedResponseIndex) {
      expandedResponseIndex.value = index
    } else if (expandedResponseIndex.value !== null) {
      if (draggedResponseIndex < expandedResponseIndex.value && index >= expandedResponseIndex.value) {
        expandedResponseIndex.value--
      } else if (draggedResponseIndex > expandedResponseIndex.value && index <= expandedResponseIndex.value) {
        expandedResponseIndex.value++
      }
    }
  }
  draggedResponseIndex = null
}

// Group drag handlers
function onGroupDragStart(e: DragEvent) {
  isDragging.value = true
  if (e.dataTransfer && cardRef.value) {
    const dragImage = cardRef.value.cloneNode(true) as HTMLElement
    dragImage.style.position = 'absolute'
    dragImage.style.top = '-9999px'
    dragImage.style.left = '-9999px'
    dragImage.style.width = cardRef.value.offsetWidth + 'px'
    dragImage.style.opacity = '0.9'
    document.body.appendChild(dragImage)
    e.dataTransfer.setDragImage(dragImage, 20, 20)
    setTimeout(() => document.body.removeChild(dragImage), 0)
  }
  emit('dragstart', e)
}

function onGroupDragEnd() {
  isDragging.value = false
}

function onGroupDragOver(e: DragEvent) {
  emit('dragover', e)
}

function onGroupDrop(e: DragEvent) {
  emit('drop', e)
}
</script>

<template>
  <div
    ref="cardRef"
    class="rounded-lg border overflow-hidden transition-all"
    :class="[
      isEnabled ? 'bg-gray-850 border-blue-700' : 'bg-gray-900 border-gray-800 opacity-60',
      { 'opacity-50': isDragging }
    ]"
    @dragover.prevent="onGroupDragOver"
    @drop="onGroupDrop"
  >
    <!-- Group Header -->
    <div
      class="px-3 py-2 bg-blue-900/30 cursor-grab active:cursor-grabbing hover:bg-blue-900/40 transition-colors select-none"
      draggable="true"
      @dragstart="onGroupDragStart"
      @dragend="onGroupDragEnd"
    >
      <div class="flex items-center gap-2">
        <!-- Priority Number -->
        <span class="text-xs text-gray-500 font-mono w-4 flex-shrink-0">{{ index + 1 }}</span>

        <!-- Drag Handle Icon -->
        <div class="text-gray-500 flex-shrink-0">
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path d="M7 2a2 2 0 1 0 .001 4.001A2 2 0 0 0 7 2zm0 6a2 2 0 1 0 .001 4.001A2 2 0 0 0 7 8zm0 6a2 2 0 1 0 .001 4.001A2 2 0 0 0 7 14zm6-8a2 2 0 1 0-.001-4.001A2 2 0 0 0 13 6zm0 2a2 2 0 1 0 .001 4.001A2 2 0 0 0 13 8zm0 6a2 2 0 1 0 .001 4.001A2 2 0 0 0 13 14z"/>
          </svg>
        </div>

        <!-- Folder Icon -->
        <svg class="w-4 h-4 text-blue-400 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
          <path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
        </svg>

        <!-- Group Name (editable) -->
        <template v-if="isEditing">
          <input
            v-model="editName"
            type="text"
            class="flex-1 px-2 py-0.5 bg-gray-900 border border-blue-500 rounded text-sm text-white focus:outline-none"
            @keyup.enter="saveName"
            @keyup.escape="cancelEdit"
            @blur="saveName"
            @click.stop
            autofocus
          />
        </template>
        <template v-else>
          <span
            class="text-sm text-blue-300 font-medium truncate flex-1 cursor-pointer hover:text-blue-200"
            @click.stop="startEditing"
            title="Click to edit name"
          >
            {{ group.name }}
          </span>
        </template>

        <!-- Response count -->
        <span class="text-xs text-gray-500 flex-shrink-0">
          {{ group.responses?.length || 0 }} rules
        </span>

        <!-- Enable/Disable Toggle -->
        <button
          @click.stop="isEnabled = !isEnabled"
          class="flex-shrink-0 p-0.5 rounded transition-colors"
          :class="isEnabled ? 'text-green-500 hover:text-green-400' : 'text-gray-600 hover:text-gray-500'"
          :title="isEnabled ? 'Enabled - click to disable all' : 'Disabled - click to enable all'"
        >
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path v-if="isEnabled" fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
            <path v-else fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
          </svg>
        </button>

        <!-- Expand/Collapse Arrow -->
        <button
          @click.stop="isExpanded = !isExpanded"
          class="flex-shrink-0 p-0.5 text-gray-400 hover:text-white transition-colors"
        >
          <svg
            class="w-4 h-4 transition-transform"
            :class="{ 'rotate-180': isExpanded }"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>

        <!-- Delete Group Button -->
        <button
          @click.stop="emit('delete')"
          class="flex-shrink-0 p-0.5 text-gray-500 hover:text-red-400 transition-colors"
          title="Delete group"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Group Content (responses) -->
    <div v-if="isExpanded" class="p-2 space-y-2 bg-gray-900/50">
      <!-- Responses list -->
      <ResponseRuleCard
        v-for="(response, idx) in group.responses"
        :key="response.id || idx"
        :response="response"
        :is-expanded="expandedResponseIndex === idx"
        :index="idx"
        @toggle="toggleResponse(idx)"
        @update="updateResponse(idx, $event)"
        @delete="deleteResponse(idx)"
        @dragstart="onResponseDragStart(idx, $event)"
        @dragover="onResponseDragOver(idx, $event)"
        @drop="onResponseDrop(idx, $event)"
      />

      <!-- Add Response Button -->
      <button
        @click="addResponse"
        class="w-full py-2 border-2 border-dashed border-gray-700 hover:border-blue-500 rounded-lg
               text-gray-500 hover:text-blue-400 text-sm transition-colors flex items-center justify-center gap-2"
      >
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        Add Response to Group
      </button>
    </div>
  </div>
</template>
