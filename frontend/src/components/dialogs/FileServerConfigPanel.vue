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

const basePath = ref(props.config.base_path || '')
const enableSSI = ref(props.config.enable_ssi || false)
const proxyConfig = ref<models.ProxyConfig>(
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
    base_path: basePath.value,
    enable_ssi: enableSSI.value,
    proxy_config: proxyConfig.value,
  }))
}

function handleProxyConfigUpdate(cfg: models.ProxyConfig) {
  proxyConfig.value = cfg
  emitUpdate()
}

// Keep local state in sync if parent passes a new config (e.g. dialog re-open)
watch(() => props.config, (cfg) => {
  basePath.value = cfg.base_path || ''
  enableSSI.value = cfg.enable_ssi || false
  proxyConfig.value = cfg.proxy_config ?? proxyConfig.value
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
        placeholder="/home/user/myapp/src"
        class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white
               placeholder-gray-400 focus:outline-none focus:border-blue-500 font-mono text-sm"
      />
      <p class="mt-1 text-xs text-gray-400">
        Filesystem directory to serve files from. Supports <code class="text-gray-300">~</code> for home directory.
        The translated request path is appended to this base path to locate files on disk.
      </p>
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
          matching pipeline — the same way a browser request would be handled.
        </p>
      </div>
    </div>

    <!-- Info box -->
    <div class="p-4 bg-yellow-900/20 border border-yellow-800 rounded">
      <p class="text-sm font-medium text-yellow-300 mb-2">About File Server Endpoints</p>
      <div class="space-y-2 text-xs text-yellow-200">
        <p>
          File server endpoints serve files directly from a local directory using the same
          path prefix, translation, and domain filter machinery as proxy endpoints.
        </p>
        <p>
          The translated request path (after prefix stripping / regex replacement) is joined
          onto the base directory to locate the file on disk.
        </p>
        <p>
          SSI <code class="text-yellow-100">virtual=</code> includes are resolved by issuing an
          internal sub-request — so existing endpoint translation rules automatically handle
          path rewriting for included fragments.
        </p>
      </div>
    </div>

    <!-- Divider -->
    <div class="border-t border-gray-700" />

    <!-- Header / Status manipulation (reuses ProxyConfigPanel, Backend+Health tabs hidden) -->
    <ProxyConfigPanel
      :config="proxyConfig"
      :is-file-server-endpoint="true"
      @update:config="handleProxyConfigUpdate"
    />

  </div>
</template>
