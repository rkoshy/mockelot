<script lang="ts" setup>
import { ref, watch, computed } from 'vue'
import { models } from '../../../wailsjs/go/models'
import { ScanProjectDir, SelectProjectDir } from '../../../wailsjs/go/main/App'
import EnvironmentVarList from './EnvironmentVarList.vue'
import ProxyConfigPanel from './ProxyConfigPanel.vue'

const props = defineProps<{
  config: models.DevServerConfig
}>()

const emit = defineEmits<{
  'update:config': [config: models.DevServerConfig]
}>()

// Local state
const projectDir = ref(props.config.project_dir || '')
const command = ref(props.config.command || '')
const autoInstall = ref(props.config.auto_install || false)
const startOnBoot = ref(props.config.start_on_boot || false)
const envVars = ref<models.EnvironmentVar[]>(props.config.env_vars || [])
const proxyConfig = ref<models.ProxyConfig>(
  props.config.proxy_config ?? new models.ProxyConfig({
    inbound_headers: [],
    outbound_headers: [],
    status_passthrough: true,
    status_translation: [],
    health_check_enabled: false,
    health_check_interval: 30,
    timeout_seconds: 30,
    backend_url: '',
  })
)

// Project scan state
interface ProjectInfo {
  name: string
  scripts: Record<string, string>
  suggested_scripts: string[]
  detected_framework: string
  package_manager: string
  has_node_modules: boolean
  error?: string
}
const projectInfo = ref<ProjectInfo | null>(null)
const scanning = ref(false)
const scanError = ref('')
const selectingDir = ref(false)

// Script dropdown options derived from scanned project
const scriptOptions = computed(() => {
  if (!projectInfo.value) return []
  return Object.keys(projectInfo.value.scripts)
})

// Framework badge label
const frameworkLabel = computed(() => {
  const fw = projectInfo.value?.detected_framework
  if (!fw) return ''
  const labels: Record<string, string> = {
    angular:     'Angular',
    'vue-vite':  'Vue + Vite',
    vue:         'Vue',
    'react-cra': 'React (CRA)',
    'react-vite':'React + Vite',
    next:        'Next.js',
    nuxt:        'Nuxt',
    sveltekit:   'SvelteKit',
    'solid-vite':'Solid + Vite',
    gatsby:      'Gatsby',
    remix:       'Remix',
    astro:       'Astro',
    vite:        'Vite',
  }
  return labels[fw] ?? fw
})

// Package manager label
const pmLabel = computed(() => projectInfo.value?.package_manager ?? 'npm')

// Command placeholder based on detected setup
const commandPlaceholder = computed(() => {
  const pm = projectInfo.value?.package_manager ?? 'npm'
  const fw = projectInfo.value?.detected_framework ?? ''
  if (fw === 'angular') return `${pm} run start`
  if (fw === 'react-cra') return `${pm} start`
  return `${pm} run dev`
})

// Watch projectDir and auto-scan when it changes
watch(projectDir, async (dir) => {
  if (!dir) {
    projectInfo.value = null
    return
  }
  await scanDir(dir)
})

async function scanDir(dir: string) {
  scanning.value = true
  scanError.value = ''
  projectInfo.value = null
  try {
    const info = await ScanProjectDir(dir) as ProjectInfo
    if (info.error) {
      scanError.value = info.error
    } else {
      projectInfo.value = info
      // Auto-fill command from suggested scripts if blank
      if (!command.value && info.suggested_scripts.length > 0) {
        const pm = info.package_manager
        const script = info.suggested_scripts[0]
        command.value = `${pm} run ${script}`
        emitUpdate()
      }
    }
  } catch (err) {
    scanError.value = String(err)
  } finally {
    scanning.value = false
  }
}

async function browseDirs() {
  selectingDir.value = true
  try {
    const dir = await SelectProjectDir()
    if (dir) {
      projectDir.value = dir
      emitUpdate()
    }
  } finally {
    selectingDir.value = false
  }
}

function selectScript(scriptName: string) {
  const pm = projectInfo.value?.package_manager ?? 'npm'
  command.value = `${pm} run ${scriptName}`
  emitUpdate()
}

function emitUpdate() {
  emit('update:config', new models.DevServerConfig({
    project_dir:  projectDir.value,
    command:      command.value,
    auto_install: autoInstall.value,
    start_on_boot: startOnBoot.value,
    env_vars:     envVars.value,
    proxy_config: proxyConfig.value,
    // runtime state not included
    port: props.config.port ?? 0,
    process_id: props.config.process_id ?? 0,
  }))
}

function handleProxyConfigUpdate(cfg: models.ProxyConfig) {
  proxyConfig.value = cfg
  emitUpdate()
}

function handleEnvVarsUpdate(vars: models.EnvironmentVar[]) {
  envVars.value = vars
  emitUpdate()
}

// Sync when parent passes new config
watch(() => props.config, (cfg) => {
  projectDir.value = cfg.project_dir || ''
  command.value = cfg.command || ''
  autoInstall.value = cfg.auto_install || false
  startOnBoot.value = cfg.start_on_boot || false
  envVars.value = cfg.env_vars || []
  if (cfg.proxy_config) proxyConfig.value = cfg.proxy_config
}, { deep: true })

// Scan on mount if dir already set
if (projectDir.value) {
  scanDir(projectDir.value)
}
</script>

<template>
  <div class="space-y-6">

    <!-- Project Directory -->
    <div>
      <label class="block text-sm font-medium text-gray-300 mb-2">
        Project Directory <span class="text-red-400">*</span>
      </label>
      <div class="flex gap-2">
        <input
          v-model="projectDir"
          @blur="emitUpdate"
          type="text"
          placeholder="/home/user/my-app"
          class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white
                 placeholder-gray-400 focus:outline-none focus:border-orange-500 font-mono text-sm"
        />
        <button
          @click="browseDirs"
          :disabled="selectingDir"
          class="px-3 py-2 bg-gray-600 hover:bg-gray-500 disabled:opacity-50 text-white rounded text-sm transition-colors whitespace-nowrap"
        >
          {{ selectingDir ? 'Opening...' : 'Browse...' }}
        </button>
      </div>
      <p class="mt-1 text-xs text-gray-400">
        Directory containing <code class="text-gray-300">package.json</code>. Use Browse or paste the path directly.
      </p>

      <!-- Scan status -->
      <div v-if="scanning" class="mt-2 flex items-center gap-2 text-xs text-gray-400">
        <svg class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
        Scanning...
      </div>
      <p v-if="scanError" class="mt-2 text-xs text-red-400">{{ scanError }}</p>
    </div>

    <!-- Detected info banner -->
    <div v-if="projectInfo && !projectInfo.error" class="p-3 bg-gray-700 rounded border border-gray-600 space-y-2">
      <div class="flex items-center gap-3 flex-wrap">
        <span class="text-sm font-medium text-white">{{ projectInfo.name }}</span>
        <span v-if="frameworkLabel" class="px-2 py-0.5 bg-orange-900/50 text-orange-300 text-xs rounded border border-orange-700">
          {{ frameworkLabel }}
        </span>
        <span class="px-2 py-0.5 bg-gray-600 text-gray-300 text-xs rounded">{{ pmLabel }}</span>
        <span
          :class="projectInfo.has_node_modules ? 'text-green-400' : 'text-yellow-400'"
          class="text-xs"
        >
          {{ projectInfo.has_node_modules ? '✓ node_modules present' : '⚠ node_modules missing' }}
        </span>
      </div>

      <!-- Suggested scripts -->
      <div v-if="projectInfo.suggested_scripts.length > 0">
        <p class="text-xs text-gray-400 mb-1">Quick select script:</p>
        <div class="flex flex-wrap gap-1">
          <button
            v-for="s in projectInfo.suggested_scripts"
            :key="s"
            @click="selectScript(s)"
            class="px-2 py-0.5 bg-gray-600 hover:bg-orange-800 text-gray-200 text-xs rounded transition-colors font-mono"
          >
            {{ s }}
          </button>
        </div>
      </div>
    </div>

    <!-- Command -->
    <div>
      <label class="block text-sm font-medium text-gray-300 mb-2">
        Command <span class="text-red-400">*</span>
      </label>
      <input
        v-model="command"
        @blur="emitUpdate"
        type="text"
        :placeholder="commandPlaceholder || 'npm run dev'"
        class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white
               placeholder-gray-400 focus:outline-none focus:border-orange-500 font-mono text-sm"
      />
      <div class="mt-1.5 space-y-1">
        <p class="text-xs text-gray-400">
          Use <code class="text-orange-300">$PORT</code> to inject the assigned port (e.g. <code class="text-gray-300">ng serve --port $PORT</code>).
          If <code class="text-orange-300">$PORT</code> is omitted, <code class="text-gray-300">--port &lt;port&gt;</code> is appended automatically.
        </p>
        <p v-if="scriptOptions.length > 0" class="text-xs text-gray-500">
          Available scripts: {{ scriptOptions.join(', ') }}
        </p>
      </div>
    </div>

    <!-- Checkboxes -->
    <div class="space-y-3">
      <div class="flex items-start gap-3">
        <input
          v-model="autoInstall"
          @change="emitUpdate"
          type="checkbox"
          id="ds-auto-install"
          class="mt-0.5 w-4 h-4 bg-gray-700 border-gray-600 rounded text-orange-500 focus:ring-orange-500"
        />
        <div>
          <label for="ds-auto-install" class="block text-sm font-medium text-gray-300 cursor-pointer">
            Auto-install dependencies
          </label>
          <p class="text-xs text-gray-400 mt-0.5">
            Run <code class="text-gray-300">{{ pmLabel }} install</code> before starting if
            <code class="text-gray-300">node_modules</code> is missing.
          </p>
        </div>
      </div>

      <div class="flex items-start gap-3">
        <input
          v-model="startOnBoot"
          @change="emitUpdate"
          type="checkbox"
          id="ds-start-on-boot"
          class="mt-0.5 w-4 h-4 bg-gray-700 border-gray-600 rounded text-orange-500 focus:ring-orange-500"
        />
        <div>
          <label for="ds-start-on-boot" class="block text-sm font-medium text-gray-300 cursor-pointer">
            Start automatically when Mockelot server starts
          </label>
          <p class="text-xs text-gray-400 mt-0.5">
            Launches this dev server whenever you start the Mockelot proxy server.
          </p>
        </div>
      </div>
    </div>

    <!-- Environment Variables -->
    <div>
      <label class="block text-sm font-medium text-gray-300 mb-2">Environment Variables</label>
      <p class="text-xs text-gray-400 mb-3">
        Additional environment variables passed to the process.
        <code class="text-gray-300">PORT</code> and <code class="text-gray-300">BROWSER=none</code>
        are always set automatically.
      </p>
      <EnvironmentVarList
        :model-value="envVars"
        @update:model-value="handleEnvVarsUpdate"
      />
    </div>

    <!-- Divider -->
    <div class="border-t border-gray-700" />

    <!-- Header/status manipulation -->
    <div>
      <p class="text-sm font-medium text-gray-300 mb-3">Response Manipulation</p>
      <p class="text-xs text-gray-400 mb-4">
        Optionally modify headers or status codes on responses before they reach the browser.
      </p>
      <ProxyConfigPanel
        :config="proxyConfig"
        :is-file-server-endpoint="true"
        @update:config="handleProxyConfigUpdate"
      />
    </div>

  </div>
</template>
