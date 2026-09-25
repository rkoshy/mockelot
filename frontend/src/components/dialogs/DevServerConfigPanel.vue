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

// ---- Local state mirrors ----
const projectDir        = ref(props.config.project_dir || '')
const command           = ref(props.config.command || '')
const autoInstall       = ref(props.config.auto_install || false)
const startOnBoot       = ref(props.config.start_on_boot || false)
const nodeVersionManager = ref(props.config.node_version_manager || '')
const nodeVersion       = ref(props.config.node_version || '')
const preRunScript      = ref(props.config.pre_run_script || '')
const cleanupScript     = ref(props.config.cleanup_script || '')
const envVars           = ref<models.EnvironmentVar[]>(props.config.env_vars || [])
const proxyConfig       = ref<models.ProxyConfig>(
  props.config.proxy_config ?? new models.ProxyConfig({
    inbound_headers: [], outbound_headers: [], status_passthrough: true,
    status_translation: [], health_check_enabled: false,
    health_check_interval: 30, timeout_seconds: 30, backend_url: '',
  })
)

// ---- Project scan state ----
interface ProjectInfo {
  name: string
  scripts: Record<string, string>
  suggested_scripts: string[]
  detected_framework: string
  package_manager: string
  has_node_modules: boolean
  nvm_available: boolean
  fnm_available: boolean
  volta_available: boolean
  nvmrc_version: string
  node_version_file: string
  volta_node_version: string
  suggested_manager: string
  suggested_version: string
  error?: string
}
const projectInfo   = ref<ProjectInfo | null>(null)
const scanning      = ref(false)
const scanError     = ref('')
const selectingDir  = ref(false)

// ---- Computed helpers ----
const scriptOptions = computed(() => projectInfo.value ? Object.keys(projectInfo.value.scripts) : [])

const frameworkLabel = computed(() => {
  const fw = projectInfo.value?.detected_framework
  if (!fw) return ''
  const labels: Record<string, string> = {
    angular: 'Angular', 'vue-vite': 'Vue + Vite', vue: 'Vue',
    'react-cra': 'React (CRA)', 'react-vite': 'React + Vite',
    next: 'Next.js', nuxt: 'Nuxt', sveltekit: 'SvelteKit',
    'solid-vite': 'Solid + Vite', gatsby: 'Gatsby', remix: 'Remix',
    astro: 'Astro', vite: 'Vite',
  }
  return labels[fw] ?? fw
})

const pmLabel = computed(() => projectInfo.value?.package_manager ?? 'npm')

const commandPlaceholder = computed(() => {
  const pm = projectInfo.value?.package_manager ?? 'npm'
  const fw = projectInfo.value?.detected_framework ?? ''
  if (fw === 'angular' || fw === 'react-cra') return `${pm} start`
  return `${pm} run dev`
})

// Version hint shown below the version input
const versionHint = computed(() => {
  if (!projectInfo.value) return ''
  const pi = projectInfo.value
  if (nodeVersionManager.value === 'nvm') {
    if (pi.nvmrc_version) return `.nvmrc: ${pi.nvmrc_version}`
    return ''
  }
  if (nodeVersionManager.value === 'fnm') {
    if (pi.nvmrc_version) return `.nvmrc: ${pi.nvmrc_version}`
    if (pi.node_version_file) return `.node-version: ${pi.node_version_file}`
    return ''
  }
  if (pi.volta_node_version) return `volta pin: ${pi.volta_node_version}`
  return ''
})

// Detection hints shown in the detection info box
const detectionBadges = computed(() => {
  if (!projectInfo.value) return []
  const pi = projectInfo.value
  const badges: { text: string; cls: string }[] = []
  if (pi.nvm_available)   badges.push({ text: 'nvm', cls: 'text-green-400 border-green-700 bg-green-900/30' })
  if (pi.fnm_available)   badges.push({ text: 'fnm', cls: 'text-blue-400 border-blue-700 bg-blue-900/30' })
  if (pi.volta_available) badges.push({ text: 'volta', cls: 'text-purple-400 border-purple-700 bg-purple-900/30' })
  return badges
})

const versionFileDetected = computed(() => {
  if (!projectInfo.value) return ''
  const pi = projectInfo.value
  if (pi.nvmrc_version)    return `.nvmrc (${pi.nvmrc_version})`
  if (pi.node_version_file) return `.node-version (${pi.node_version_file})`
  if (pi.volta_node_version) return `package.json volta (${pi.volta_node_version})`
  return ''
})

// Available manager options for radio group
const managerOptions = computed(() => {
  const opts = [{ value: '', label: 'System default' }]
  if (projectInfo.value?.nvm_available)   opts.push({ value: 'nvm', label: 'nvm' })
  if (projectInfo.value?.fnm_available)   opts.push({ value: 'fnm', label: 'fnm' })
  // Always show nvm/fnm even if not detected so user can still configure
  if (!projectInfo.value?.nvm_available)  opts.push({ value: 'nvm', label: 'nvm (not detected)' })
  if (!projectInfo.value?.fnm_available)  opts.push({ value: 'fnm', label: 'fnm (not detected)' })
  return opts
})

// Pre-run script placeholder that shows what nvm/fnm would inject automatically
const preRunPlaceholder = computed(() => {
  if (nodeVersionManager.value === 'nvm') {
    return '# Additional setup runs here after nvm is loaded\n# export MY_VAR=value'
  }
  if (nodeVersionManager.value === 'fnm') {
    return '# Additional setup runs here after fnm is loaded\n# export MY_VAR=value'
  }
  return '# Runs before the command in the same shell session\n# export MY_VAR=value\n# source .env'
})

// ---- Actions ----
watch(projectDir, async (dir) => {
  if (!dir) { projectInfo.value = null; return }
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
      // Auto-select suggested manager if not already set
      if (!nodeVersionManager.value && info.suggested_manager) {
        nodeVersionManager.value = info.suggested_manager
        if (!nodeVersion.value && info.suggested_version) {
          nodeVersion.value = info.suggested_version
        }
      }
      // Auto-fill command if blank
      if (!command.value && info.suggested_scripts.length > 0) {
        command.value = `${info.package_manager} run ${info.suggested_scripts[0]}`
      }
      emitUpdate()
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
    if (dir) { projectDir.value = dir; emitUpdate() }
  } finally {
    selectingDir.value = false
  }
}

function selectScript(scriptName: string) {
  command.value = `${pmLabel.value} run ${scriptName}`
  emitUpdate()
}

function emitUpdate() {
  emit('update:config', new models.DevServerConfig({
    project_dir:          projectDir.value,
    command:              command.value,
    auto_install:         autoInstall.value,
    start_on_boot:        startOnBoot.value,
    node_version_manager: nodeVersionManager.value,
    node_version:         nodeVersion.value,
    pre_run_script:       preRunScript.value,
    cleanup_script:       cleanupScript.value,
    env_vars:             envVars.value,
    proxy_config:         proxyConfig.value,
    port:                 props.config.port ?? 0,
    process_id:           props.config.process_id ?? 0,
  }))
}

function handleProxyConfigUpdate(cfg: models.ProxyConfig) { proxyConfig.value = cfg; emitUpdate() }
function handleEnvVarsUpdate(vars: models.EnvironmentVar[]) { envVars.value = vars; emitUpdate() }

// Sync when parent passes new config (e.g. endpoint switches)
watch(() => props.config, (cfg) => {
  projectDir.value        = cfg.project_dir || ''
  command.value           = cfg.command || ''
  autoInstall.value       = cfg.auto_install || false
  startOnBoot.value       = cfg.start_on_boot || false
  nodeVersionManager.value = cfg.node_version_manager || ''
  nodeVersion.value       = cfg.node_version || ''
  preRunScript.value      = cfg.pre_run_script || ''
  cleanupScript.value     = cfg.cleanup_script || ''
  envVars.value           = cfg.env_vars || []
  if (cfg.proxy_config) proxyConfig.value = cfg.proxy_config
}, { deep: true })

// Scan on mount if dir already set
if (projectDir.value) scanDir(projectDir.value)
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
      <div v-if="scanning" class="mt-2 flex items-center gap-2 text-xs text-gray-400">
        <svg class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
        Scanning...
      </div>
      <p v-if="scanError" class="mt-2 text-xs text-red-400">{{ scanError }}</p>
    </div>

    <!-- Detected project info -->
    <div v-if="projectInfo && !projectInfo.error" class="p-3 bg-gray-700 rounded border border-gray-600 space-y-2">
      <div class="flex items-center gap-3 flex-wrap">
        <span class="text-sm font-medium text-white">{{ projectInfo.name }}</span>
        <span v-if="frameworkLabel" class="px-2 py-0.5 bg-orange-900/50 text-orange-300 text-xs rounded border border-orange-700">
          {{ frameworkLabel }}
        </span>
        <span class="px-2 py-0.5 bg-gray-600 text-gray-300 text-xs rounded">{{ pmLabel }}</span>
        <span :class="projectInfo.has_node_modules ? 'text-green-400' : 'text-yellow-400'" class="text-xs">
          {{ projectInfo.has_node_modules ? '✓ node_modules present' : '⚠ node_modules missing' }}
        </span>
      </div>
      <!-- Version managers detected -->
      <div v-if="detectionBadges.length > 0 || versionFileDetected" class="flex items-center gap-2 flex-wrap">
        <span class="text-xs text-gray-400">Detected:</span>
        <span
          v-for="b in detectionBadges" :key="b.text"
          :class="['px-1.5 py-0.5 text-xs rounded border font-mono', b.cls]"
        >{{ b.text }}</span>
        <span v-if="versionFileDetected" class="text-xs text-gray-300 font-mono">{{ versionFileDetected }}</span>
      </div>
      <!-- Quick select script -->
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
      <p class="mt-1 text-xs text-gray-400">
        Use <code class="text-orange-300">$PORT</code> to inject the port (e.g. <code class="text-gray-300">ng serve --port $PORT</code>).
        If omitted, <code class="text-gray-300">--port &lt;port&gt;</code> is appended automatically.
      </p>
      <p v-if="scriptOptions.length > 0" class="mt-0.5 text-xs text-gray-500">
        Available scripts: {{ scriptOptions.join(', ') }}
      </p>
    </div>

    <!-- ── Node Version ─────────────────────────────────────────────────── -->
    <div class="space-y-3">
      <div class="flex items-center gap-2">
        <label class="text-sm font-medium text-gray-300">Node Version</label>
        <span class="text-xs text-gray-500">— set the right Node.js version for this project</span>
      </div>

      <!-- Manager radio buttons -->
      <div class="flex flex-wrap gap-4">
        <label
          v-for="opt in managerOptions"
          :key="opt.value"
          class="flex items-center gap-2 cursor-pointer"
        >
          <input
            type="radio"
            :value="opt.value"
            v-model="nodeVersionManager"
            @change="emitUpdate"
            class="w-3.5 h-3.5 text-orange-500 focus:ring-orange-500 bg-gray-700 border-gray-600"
          />
          <span
            :class="[
              'text-sm',
              opt.value === '' ? 'text-gray-300' :
              (opt.value === 'nvm' && projectInfo?.nvm_available) ||
              (opt.value === 'fnm' && projectInfo?.fnm_available)
                ? 'text-orange-300'
                : 'text-gray-500',
            ]"
          >{{ opt.label }}</span>
        </label>
      </div>

      <!-- Version input (shown when nvm or fnm selected) -->
      <div v-if="nodeVersionManager === 'nvm' || nodeVersionManager === 'fnm'" class="space-y-1">
        <input
          v-model="nodeVersion"
          @blur="emitUpdate"
          type="text"
          placeholder="e.g. 18, 20.11.1, lts/hydrogen"
          class="w-48 px-3 py-1.5 bg-gray-700 border border-gray-600 rounded text-white
                 placeholder-gray-400 focus:outline-none focus:border-orange-500 font-mono text-sm"
        />
        <p class="text-xs text-gray-400">
          <template v-if="versionHint">
            <span class="text-orange-300">{{ versionHint }}</span> —
          </template>
          Leave blank to use project <code class="text-gray-300">.nvmrc</code> or <code class="text-gray-300">.node-version</code>.
        </p>

        <!-- nvm explanation -->
        <div v-if="nodeVersionManager === 'nvm'" class="p-2.5 bg-gray-700/50 rounded border border-gray-600 text-xs text-gray-300 space-y-1">
          <p class="font-medium text-orange-300">How nvm runs here:</p>
          <p>Mockelot sources <code class="text-gray-200">$NVM_DIR/nvm.sh</code> explicitly before your command — no need to have it loaded in your shell profile.</p>
          <p v-if="nodeVersion">Generated: <code class="text-gray-200 bg-gray-800 px-1 rounded">nvm use {{ nodeVersion }} || nvm install {{ nodeVersion }}</code></p>
          <p v-else>Generated: <code class="text-gray-200 bg-gray-800 px-1 rounded">nvm use</code> (reads .nvmrc)</p>
        </div>

        <!-- fnm explanation -->
        <div v-if="nodeVersionManager === 'fnm'" class="p-2.5 bg-gray-700/50 rounded border border-gray-600 text-xs text-gray-300 space-y-1">
          <p class="font-medium text-blue-300">How fnm runs here:</p>
          <p>Mockelot runs <code class="text-gray-200">eval "$(fnm env)"</code> then switches to the specified version before your command.</p>
          <p v-if="nodeVersion">Generated: <code class="text-gray-200 bg-gray-800 px-1 rounded">fnm use {{ nodeVersion }} || fnm install {{ nodeVersion }}</code></p>
          <p v-else>Generated: <code class="text-gray-200 bg-gray-800 px-1 rounded">fnm use</code> (reads .nvmrc / .node-version)</p>
        </div>
      </div>
    </div>

    <!-- ── Pre-run Script ────────────────────────────────────────────────── -->
    <div>
      <label class="block text-sm font-medium text-gray-300 mb-1">Pre-run Script</label>
      <p class="text-xs text-gray-400 mb-2">
        Shell commands that run <strong class="text-gray-300">before</strong> the dev server command in the same shell session.
        <span v-if="nodeVersionManager">nvm/fnm setup is injected automatically before this.</span>
      </p>
      <textarea
        v-model="preRunScript"
        @blur="emitUpdate"
        rows="4"
        :placeholder="preRunPlaceholder"
        class="w-full px-3 py-2 bg-gray-900 border border-gray-600 rounded text-green-300
               placeholder-gray-600 focus:outline-none focus:border-orange-500 font-mono text-xs
               resize-y leading-relaxed"
      />
    </div>

    <!-- ── Cleanup Script ────────────────────────────────────────────────── -->
    <div>
      <label class="block text-sm font-medium text-gray-300 mb-1">Cleanup Script</label>
      <p class="text-xs text-gray-400 mb-2">
        Shell commands that run <strong class="text-gray-300">after</strong> the dev server stops, as a separate invocation.
        Useful for cleaning temp files or resetting state.
      </p>
      <textarea
        v-model="cleanupScript"
        @blur="emitUpdate"
        rows="3"
        placeholder="# Runs after the process stops (separate shell invocation)&#10;# rm -f .dev.pid"
        class="w-full px-3 py-2 bg-gray-900 border border-gray-600 rounded text-green-300
               placeholder-gray-600 focus:outline-none focus:border-orange-500 font-mono text-xs
               resize-y leading-relaxed"
      />
    </div>

    <!-- ── Checkboxes ────────────────────────────────────────────────────── -->
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
            Respects the Node version setting above.
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
        </div>
      </div>
    </div>

    <!-- ── Environment Variables ─────────────────────────────────────────── -->
    <div>
      <label class="block text-sm font-medium text-gray-300 mb-2">Environment Variables</label>
      <p class="text-xs text-gray-400 mb-3">
        Additional variables passed to the process.
        <code class="text-gray-300">PORT</code> and <code class="text-gray-300">BROWSER=none</code> are always set.
      </p>
      <EnvironmentVarList :model-value="envVars" @update:model-value="handleEnvVarsUpdate" />
    </div>

    <!-- ── Response Manipulation ─────────────────────────────────────────── -->
    <div class="border-t border-gray-700 pt-4">
      <p class="text-sm font-medium text-gray-300 mb-3">Response Manipulation</p>
      <p class="text-xs text-gray-400 mb-4">Optionally modify headers or status codes on responses.</p>
      <ProxyConfigPanel
        :config="proxyConfig"
        :is-file-server-endpoint="true"
        @update:config="handleProxyConfigUpdate"
      />
    </div>

  </div>
</template>
