<script lang="ts" setup>
import { ref, onMounted, onUnmounted } from 'vue'

const props = defineProps<{
  initialLeftWidth?: number  // in pixels
  minLeftWidth?: number
  minRightWidth?: number
}>()

const containerRef = ref<HTMLElement | null>(null)
const leftWidthPx = ref(props.initialLeftWidth || 500) // pixels
const isDragging = ref(false)

const minLeft = props.minLeftWidth || 200
const minRight = props.minRightWidth || 200

function startDrag(e: MouseEvent) {
  isDragging.value = true
  e.preventDefault()
}

function onDrag(e: MouseEvent) {
  if (!isDragging.value || !containerRef.value) return

  const container = containerRef.value
  const rect = container.getBoundingClientRect()
  const containerWidth = rect.width

  let newLeftWidth = e.clientX - rect.left

  // Apply min constraints
  if (newLeftWidth < minLeft) newLeftWidth = minLeft
  if (containerWidth - newLeftWidth < minRight) newLeftWidth = containerWidth - minRight

  leftWidthPx.value = newLeftWidth
}

function stopDrag() {
  isDragging.value = false
}

onMounted(() => {
  document.addEventListener('mousemove', onDrag)
  document.addEventListener('mouseup', stopDrag)
})

onUnmounted(() => {
  document.removeEventListener('mousemove', onDrag)
  document.removeEventListener('mouseup', stopDrag)
})
</script>

<template>
  <div
    ref="containerRef"
    class="flex h-full overflow-hidden"
    :class="{ 'select-none': isDragging }"
  >
    <!-- Left Panel -->
    <div
      class="h-full overflow-hidden flex-shrink-0"
      :style="{ width: `${leftWidthPx}px` }"
    >
      <slot name="left" />
    </div>

    <!-- Resize Handle -->
    <div
      @mousedown="startDrag"
      class="w-1 h-full bg-gray-700 hover:bg-blue-500 cursor-col-resize flex-shrink-0 transition-colors"
      :class="{ 'bg-blue-500': isDragging }"
    />

    <!-- Right Panel -->
    <div class="h-full overflow-hidden flex-1 min-w-0">
      <slot name="right" />
    </div>
  </div>
</template>
