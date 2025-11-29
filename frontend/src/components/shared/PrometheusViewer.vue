<script lang="ts" setup>
import { computed, ref } from 'vue'
import { parsePrometheusMetrics, type MetricGroup, type MetricLine } from '../../utils/prometheus-formatter'

const props = defineProps<{
  content: string
}>()

const allMetricGroups = computed(() => parsePrometheusMetrics(props.content))

// Filter text
const filterText = ref('')

// Filtered metric groups based on search text
const metricGroups = computed(() => {
  const filter = filterText.value.toLowerCase().trim()
  if (!filter) {
    return allMetricGroups.value
  }

  return allMetricGroups.value
    .map(group => {
      // Check if group name or help matches
      const groupMatches =
        group.name.toLowerCase().includes(filter) ||
        (group.help && group.help.toLowerCase().includes(filter))

      if (groupMatches) {
        // If group matches, include all metrics
        return group
      }

      // Filter metrics by label values
      const filteredMetrics = group.metrics.filter(metric => {
        if (!metric.labels) return false
        return Object.entries(metric.labels).some(([key, value]) =>
          key.toLowerCase().includes(filter) ||
          value.toLowerCase().includes(filter)
        )
      })

      if (filteredMetrics.length > 0) {
        return { ...group, metrics: filteredMetrics }
      }

      return null
    })
    .filter((group): group is MetricGroup => group !== null)
})

// Track expanded state for each group
const expandedGroups = ref<Set<string>>(new Set())

// Initialize all groups as expanded
computed(() => {
  const names = metricGroups.value.map(g => g.name)
  names.forEach(name => expandedGroups.value.add(name))
})

function toggleGroup(name: string) {
  if (expandedGroups.value.has(name)) {
    expandedGroups.value.delete(name)
  } else {
    expandedGroups.value.add(name)
  }
}

function isExpanded(name: string): boolean {
  // Default to expanded if not in set
  return !expandedGroups.value.has(name) || expandedGroups.value.has(name)
}

// Format value for display
function formatValue(value: string | undefined): string {
  if (!value) return '-'

  const num = parseFloat(value)
  if (isNaN(num)) return value // NaN, +Inf, -Inf

  // Format large numbers with commas
  if (Number.isInteger(num) && Math.abs(num) >= 1000) {
    return num.toLocaleString()
  }

  // Format decimals nicely
  if (!Number.isInteger(num)) {
    return num.toLocaleString(undefined, { maximumFractionDigits: 6 })
  }

  return value
}

// Get type badge color
function getTypeBadgeClass(type: string | undefined): string {
  switch (type) {
    case 'counter':
      return 'bg-blue-600'
    case 'gauge':
      return 'bg-green-600'
    case 'histogram':
      return 'bg-purple-600'
    case 'summary':
      return 'bg-orange-600'
    case 'untyped':
      return 'bg-gray-600'
    default:
      return 'bg-gray-600'
  }
}

// Get label pill color based on key
function getLabelColor(key: string): string {
  // Common label types get distinct colors
  const colors: Record<string, string> = {
    // Infrastructure
    host: 'bg-cyan-700',
    hostname: 'bg-cyan-700',
    instance: 'bg-cyan-700',
    node: 'bg-cyan-700',
    server: 'bg-cyan-700',

    // Environment
    env: 'bg-amber-700',
    environment: 'bg-amber-700',
    stage: 'bg-amber-700',

    // Service
    service: 'bg-indigo-700',
    job: 'bg-indigo-700',
    app: 'bg-indigo-700',
    application: 'bg-indigo-700',

    // HTTP
    method: 'bg-blue-700',
    code: 'bg-emerald-700',
    status: 'bg-emerald-700',
    status_code: 'bg-emerald-700',
    path: 'bg-violet-700',
    endpoint: 'bg-violet-700',
    handler: 'bg-violet-700',

    // State
    state: 'bg-rose-700',
    type: 'bg-pink-700',
    mode: 'bg-pink-700',

    // Kubernetes
    namespace: 'bg-teal-700',
    pod: 'bg-teal-700',
    container: 'bg-teal-700',

    // Database
    db: 'bg-orange-700',
    database: 'bg-orange-700',
    table: 'bg-orange-700',
    query: 'bg-orange-700',
  }

  return colors[key.toLowerCase()] || 'bg-gray-600'
}

// Expand all groups
function expandAll() {
  metricGroups.value.forEach(g => expandedGroups.value.add(g.name))
}

// Collapse all groups
function collapseAll() {
  expandedGroups.value.clear()
}
</script>

<template>
  <div class="prometheus-viewer">
    <!-- Toolbar -->
    <div class="flex items-center gap-3 mb-3 pb-2 border-b border-gray-700">
      <!-- Filter Input -->
      <div class="flex-1 relative">
        <input
          v-model="filterText"
          type="text"
          placeholder="Filter metrics..."
          class="w-full px-3 py-1.5 pl-8 bg-gray-800 border border-gray-600 rounded text-xs text-white
                 focus:outline-none focus:border-blue-500 placeholder-gray-500"
        />
        <svg
          class="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-500"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <button
          v-if="filterText"
          @click="filterText = ''"
          class="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-white transition-colors"
        >
          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <!-- Count -->
      <div class="text-xs text-gray-400 flex-shrink-0">
        <span v-if="filterText">{{ metricGroups.length }}/{{ allMetricGroups.length }}</span>
        <span v-else>{{ metricGroups.length }} metric{{ metricGroups.length !== 1 ? 's' : '' }}</span>
      </div>

      <!-- Expand/Collapse Buttons -->
      <div class="flex items-center gap-1 flex-shrink-0">
        <button
          @click="expandAll"
          class="px-2 py-0.5 text-xs text-gray-400 hover:text-white transition-colors"
          title="Expand All"
        >
          Expand
        </button>
        <span class="text-gray-600">|</span>
        <button
          @click="collapseAll"
          class="px-2 py-0.5 text-xs text-gray-400 hover:text-white transition-colors"
          title="Collapse All"
        >
          Collapse
        </button>
      </div>
    </div>

    <!-- Metric Groups -->
    <div class="space-y-3">
      <div
        v-for="group in metricGroups"
        :key="group.name"
        class="bg-gray-800/50 rounded-lg border border-gray-700 overflow-hidden"
      >
        <!-- Group Header -->
        <div
          @click="toggleGroup(group.name)"
          class="flex items-center gap-3 px-3 py-2 cursor-pointer hover:bg-gray-700/50 transition-colors"
        >
          <!-- Expand/Collapse Arrow -->
          <svg
            class="w-4 h-4 text-gray-400 transition-transform flex-shrink-0"
            :class="{ '-rotate-90': !expandedGroups.has(group.name) }"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>

          <!-- Metric Name -->
          <span class="text-sm font-mono text-blue-400 font-medium flex-1">
            {{ group.name }}
          </span>

          <!-- Type Badge -->
          <span
            v-if="group.type"
            :class="['px-2 py-0.5 rounded text-[10px] font-bold text-white uppercase', getTypeBadgeClass(group.type)]"
          >
            {{ group.type }}
          </span>

          <!-- Count -->
          <span class="text-xs text-gray-500">
            {{ group.metrics.length }}
          </span>
        </div>

        <!-- Help Text -->
        <div
          v-if="group.help && expandedGroups.has(group.name)"
          class="px-3 py-1.5 bg-gray-900/50 text-xs text-gray-400 italic border-t border-gray-700/50"
        >
          {{ group.help }}
        </div>

        <!-- Metrics Table -->
        <div v-if="expandedGroups.has(group.name)" class="border-t border-gray-700">
          <table class="w-full text-xs">
            <tbody>
              <tr
                v-for="(metric, idx) in group.metrics"
                :key="idx"
                class="border-b border-gray-700/50 last:border-0 hover:bg-gray-700/30 transition-colors"
              >
                <!-- Labels Column -->
                <td class="px-3 py-2 align-top">
                  <div class="flex flex-wrap gap-1">
                    <template v-if="metric.labels && Object.keys(metric.labels).length > 0">
                      <span
                        v-for="(value, key) in metric.labels"
                        :key="key"
                        :class="['inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium text-white', getLabelColor(String(key))]"
                        :title="`${key}=&quot;${value}&quot;`"
                      >
                        <span class="opacity-70">{{ key }}:</span>
                        <span class="font-mono">{{ value }}</span>
                      </span>
                    </template>
                    <span v-else class="text-gray-500 italic">(no labels)</span>
                  </div>
                </td>

                <!-- Value Column -->
                <td class="px-3 py-2 text-right align-top w-32">
                  <span class="font-mono text-green-400 font-medium">
                    {{ formatValue(metric.value) }}
                  </span>
                </td>

                <!-- Timestamp Column (if present) -->
                <td v-if="metric.timestamp" class="px-3 py-2 text-right align-top w-28 text-gray-500">
                  {{ metric.timestamp }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div v-if="metricGroups.length === 0" class="text-center py-8 text-gray-500">
      <p>No metrics found</p>
    </div>
  </div>
</template>

<style scoped>
.prometheus-viewer {
  font-size: 13px;
}
</style>
