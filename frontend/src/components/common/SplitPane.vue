<script lang="ts" setup>
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'

interface Props {
  orientation?: 'horizontal' | 'vertical' // horizontal = left/right, vertical = top/bottom
  showSecondary?: boolean // Show/hide secondary pane
  defaultSize?: number // Default size of secondary pane in pixels
  minSize?: number // Minimum size in pixels
  maxSize?: number // Maximum size in pixels (0 = no limit)
}

const props = withDefaults(defineProps<Props>(), {
  orientation: 'horizontal',
  showSecondary: false,
  defaultSize: 400,
  minSize: 200,
  maxSize: 0
})

const emit = defineEmits<{
  resize: [size: number]
}>()

const containerRef = ref<HTMLElement | null>(null)
const isDragging = ref(false)
const secondarySize = ref(props.defaultSize)

// Computed classes for layout direction
const containerClass = computed(() => {
  return props.orientation === 'horizontal' ? 'flex flex-row' : 'flex flex-col'
})

const dividerClass = computed(() => {
  const baseClass = 'flex-shrink-0 transition-colors bg-gray-700 hover:bg-blue-500 cursor-col-resize'
  if (props.orientation === 'horizontal') {
    return `${baseClass} w-1 cursor-col-resize`
  } else {
    return `${baseClass} h-1 cursor-row-resize`
  }
})

const activeDividerClass = computed(() => {
  return isDragging.value ? 'bg-blue-500' : ''
})

// Computed styles for panes
const primaryStyle = computed(() => {
  if (!props.showSecondary) {
    return props.orientation === 'horizontal'
      ? { width: '100%' }
      : { height: '100%' }
  }

  if (props.orientation === 'horizontal') {
    return {
      width: `calc(100% - ${secondarySize.value}px - 4px)` // 4px for divider
    }
  } else {
    return {
      height: `calc(100% - ${secondarySize.value}px - 4px)` // 4px for divider
    }
  }
})

const secondaryStyle = computed(() => {
  if (props.orientation === 'horizontal') {
    return {
      width: `${secondarySize.value}px`,
      display: props.showSecondary ? 'block' : 'none'
    }
  } else {
    return {
      height: `${secondarySize.value}px`,
      display: props.showSecondary ? 'block' : 'none'
    }
  }
})

// Mouse drag handlers
function startDragging(event: MouseEvent | TouchEvent) {
  event.preventDefault()
  isDragging.value = true

  const container = containerRef.value
  if (!container) return

  const containerRect = container.getBoundingClientRect()

  const onMove = (e: MouseEvent | TouchEvent) => {
    if (!isDragging.value || !containerRect) return

    let newSize: number
    if (props.orientation === 'horizontal') {
      const clientX = 'touches' in e ? e.touches[0].clientX : e.clientX
      newSize = containerRect.right - clientX
    } else {
      const clientY = 'touches' in e ? e.touches[0].clientY : e.clientY
      newSize = containerRect.bottom - clientY
    }

    // Apply min/max constraints
    const containerSize = props.orientation === 'horizontal'
      ? containerRect.width
      : containerRect.height

    const effectiveMaxSize = props.maxSize > 0
      ? props.maxSize
      : containerSize * 0.8 // Default max 80% of container

    newSize = Math.max(props.minSize, Math.min(effectiveMaxSize, newSize))

    secondarySize.value = newSize
    emit('resize', newSize)
  }

  const onEnd = () => {
    isDragging.value = false
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onEnd)
    document.removeEventListener('touchmove', onMove)
    document.removeEventListener('touchend', onEnd)
  }

  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onEnd)
  document.addEventListener('touchmove', onMove)
  document.addEventListener('touchend', onEnd)
}

// Watch for showSecondary changes to animate open/close
watch(() => props.showSecondary, (show) => {
  if (!show) {
    secondarySize.value = 0
  } else {
    secondarySize.value = props.defaultSize
  }
})

// Watch for orientation changes
watch(() => props.orientation, () => {
  if (props.showSecondary) {
    secondarySize.value = props.defaultSize
  }
})
</script>

<template>
  <div ref="containerRef" :class="containerClass" class="w-full h-full overflow-hidden">
    <!-- Primary pane (main content) -->
    <div :style="primaryStyle" class="overflow-hidden flex-shrink-0">
      <slot name="primary" />
    </div>

    <!-- Resizable divider (only visible when secondary is shown) -->
    <div
      v-if="showSecondary"
      :class="[dividerClass, activeDividerClass]"
      @mousedown="startDragging"
      @touchstart="startDragging"
      role="separator"
      :aria-orientation="orientation"
      tabindex="0"
    />

    <!-- Secondary pane (logs, etc.) -->
    <div :style="secondaryStyle" class="overflow-hidden flex-shrink-0">
      <slot name="secondary" />
    </div>
  </div>
</template>

<style scoped>
/* Prevent text selection during drag */
.cursor-col-resize:active,
.cursor-row-resize:active {
  user-select: none;
}
</style>
