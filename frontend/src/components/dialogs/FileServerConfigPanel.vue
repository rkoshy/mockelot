<script lang="ts" setup>
import { ref, watch } from 'vue'
import { models } from '../../../wailsjs/go/models'
import ProxyConfigPanel from './ProxyConfigPanel.vue'

const props = defineProps<{
  config: models.FileServerConfig
}>()

const emit = defineEmits<{
  'update:config': [config: models.FileServerConfig]
}>()

const basePath        = ref(props.config.base_path || '')
const enableSSI       = ref(props.config.enable_ssi || false)
const spaFallback     = ref(props.config.spa_fallback || false)
const spaFallbackFile = ref(props.config.spa_fallback_file || '')
const proxyConfig     = ref<models.ProxyConfig>(
  props.config.proxy_config ?? new models.ProxyConfig({
    inbound_headers: [],
    outbound_headers: [],
    status_passthrough: true,
    status_translation: [],
    health_check_enabled: false,
    health_check_interval: 30,
    timeout_seconds: 0,
    backend_url: '',
  })
)

function emitUpdate() {
  emit('update:config', new models.FileServerConfig({
    base_path:         basePath.value,
    enable_ssi:        enableSSI.value,
    spa_fallback:      spaFallback.value,
    spa_fallback_file: spaFallbackFile.value,
    proxy_config:      proxyConfig.value,
  }))
}

function handleProxyConfigUpdate(cfg: models.ProxyConfig) {
  proxyConfig.value = cfg
  emitUpdate()
}

watch(() => props.config, (cfg) => {
  basePath.value        = cfg.base_path || ''
  enableSSI.value       = cfg.enable_ssi || false
  spaFallback.value     = cfg.spa_fallback || false
  spaFallbackFile.value = cfg.spa_fallback_file || ''
  proxyConfig.value     = cfg.proxy_config ?? proxyConfig.value
}, { deep: true })
</script>

<template>
  <div class="space-y-6">

    <!-- Base Path -->
    <div>
      <label class="block text-sm font-medium text-gray-300 mb-2">
        Base Directory <span class="text-red-400">*</span>
      </label>
      <input
        v-model="basePath"
        @blur="emitUpdate"
        type="text"
        placeholder="/home/user/myapp/dist"
        class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white
               placeholder-gray-400 focus:outline-none focus:border-blue-500 font-mono text-sm"
      />
      <p class="mt-1 text-xs text-gray-400">
        Filesystem directory to serve files from. Supports <code class="text-gray-300">~</code> for home directory.
        The translated request path is appended to this base path to locate files on disk.
      </p>
    </div>

    <!-- SPA Fallback -->
    <div class="space-y-3">
      <div class="flex items-start gap-3">
        <input
          v-model="spaFallback"
          @change="emitUpdate"
          type="checkbox"
          id="spa-fallback"
          class="mt-1 w-4 h-4 bg-gray-700 border-gray-600 rounded text-yellow-500 focus:ring-yellow-500"
        />
        <div class="flex-1">
          <label for="spa-fallback" class="block text-sm font-medium text-gray-300">
            SPA Fallback (serve index.html for unknown paths)
          </label>
          <p class="text-xs text-gray-400 mt-1">
            Equivalent to nginx's <code class="text-gray-300">try_files $uri $uri/ /index.html</code>.
            Required for Angular, Vue, React, and other single-page apps that use client-side routing —
            any path that doesn't exist on disk is served the fallback file instead of a 404.
          </p>
        </div>
      </div>

      <!-- Fallback filename (only shown when SPA fallback enabled) -->
      <div v-if="spaFallback" class="ml-7">
        <label class="block text-xs font-medium text-gray-400 mb-1">Fallback file</label>
        <input
          v-model="spaFallbackFile"
          @blur="emitUpdate"
          type="text"
          placeholder="index.html"
          class="w-48 px-3 py-1.5 bg-gray-700 border border-gray-600 rounded text-white
                 placeholder-gray-500 focus:outline-none focus:border-yellow-500 font-mono text-sm"
        />
        <p class="mt-1 text-xs text-gray-500">Leave blank to use <code>index.html</code></p>
      </div>
    </div>

    <!-- SSI Toggle -->
    <div class="flex items-start gap-3">
      <input
        v-model="enableSSI"
        @change="emitUpdate"
        type="checkbox"
        id="enable-ssi"
        class="mt-1 w-4 h-4 bg-gray-700 border-gray-600 rounded text-yellow-500 focus:ring-yellow-500"
      />
      <div>
        <label for="enable-ssi" class="block text-sm font-medium text-gray-300">
          Enable SSI (Server Side Includes)
        </label>
        <p class="text-xs text-gray-400 mt-1">
          Process <code class="text-gray-300">&lt;!--#include virtual="..."--&gt;</code> directives
          in <code class="text-gray-300">.shtml</code> and <code class="text-gray-300">.html</code> files.
          Virtual include paths are resolved as internal sub-requests through the full endpoint
          matching pipeline.
        </p>
      </div>
    </div>

    <!-- Divider -->
    <div class="border-t border-gray-700" />

    <!-- Header / Status manipulation -->
    <ProxyConfigPanel
      :config="proxyConfig"
      :is-file-server-endpoint="true"
      @update:config="handleProxyConfigUpdate"
    />

  </div>
</template>
