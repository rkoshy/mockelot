<template>
  <div class="flex flex-col min-h-0 h-full overflow-hidden">
    <!-- Scrollable Content Area -->
    <div class="flex-1 overflow-y-auto p-4 space-y-4 min-h-0">
      <!-- Proxy Mode Selector -->
      <div class="bg-gray-750 border border-gray-600 rounded p-4">
        <div class="flex items-center gap-2 mb-3">
          <span class="text-sm font-semibold text-white">Proxy Mode</span>
        </div>
        <div class="flex items-center gap-6">
          <label class="flex items-center gap-2 cursor-pointer">
            <input
              type="radio"
              v-model="localSettings.serverMode"
              value="http"
              class="w-4 h-4 text-blue-600 bg-gray-700 border-gray-600 focus:ring-blue-500"
              @change="handleChange"
            />
            <span class="text-sm text-gray-200">HTTP</span>
          </label>
          <label class="flex items-center gap-2 cursor-pointer">
            <input
              type="radio"
              v-model="localSettings.serverMode"
              value="socks5"
              class="w-4 h-4 text-blue-600 bg-gray-700 border-gray-600 focus:ring-blue-500"
              @change="handleChange"
            />
            <span class="text-sm text-gray-200">SOCKS5</span>
          </label>
        </div>
        <p v-if="localSettings.serverMode === 'socks5'" class="mt-2 text-xs text-yellow-400">
          SOCKS5 mode: HTTP and HTTPS listeners will not start. Only the SOCKS5 proxy will be active.
        </p>
      </div>

      <!-- HTTP Section -->
      <CollapsibleSection v-if="localSettings.serverMode === 'http'" title="HTTP Settings" :defaultOpen="true">
        <div class="space-y-4">
          <!-- HTTP Port and Redirect in row -->
          <div class="flex items-start gap-6">
            <div>
              <label class="block text-sm font-medium text-gray-300 mb-2">
                HTTP Port
              </label>
              <input
                v-model.number="localSettings.port"
                type="number"
                min="1"
                max="65535"
                class="w-32 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                @input="handleChange"
              />
            </div>

            <div class="flex items-center pt-7">
              <input
                v-model="localSettings.httpToHttpsRedirect"
                type="checkbox"
                id="http-redirect-main"
                class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500 mr-2"
                @change="handleChange"
              />
              <label for="http-redirect-main" class="text-sm text-gray-300">
                Redirect HTTP to HTTPS
              </label>
            </div>
          </div>

          <div class="flex items-center">
            <input
              v-model="localSettings.http2Enabled"
              type="checkbox"
              id="http2-enabled"
              class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500 mr-2"
              @change="handleChange"
            />
            <label for="http2-enabled" class="text-sm text-gray-300">
              Enable HTTP/2
            </label>
          </div>
          <p class="text-xs text-gray-400 ml-6">
            Enables HTTP/2 protocol support for improved performance
          </p>
        </div>
      </CollapsibleSection>

      <!-- HTTPS Section (only in HTTP mode) -->
      <CollapsibleSection v-if="localSettings.serverMode === 'http'" title="HTTPS Settings" :defaultOpen="false">
        <div class="space-y-6">
          <!-- Enable HTTPS -->
          <div class="flex items-center">
            <input
              v-model="localSettings.httpsEnabled"
              type="checkbox"
              id="https-enabled"
              class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500 mr-2"
              @change="handleChange"
            />
            <label for="https-enabled" class="text-sm font-medium text-white">
              Enable HTTPS
            </label>
          </div>

          <div v-if="localSettings.httpsEnabled" class="space-y-6">
            <!-- HTTPS Port -->
            <div>
              <label class="block text-sm font-medium text-gray-300 mb-2">
                HTTPS Port
              </label>
              <input
                v-model.number="localSettings.httpsPort"
                type="number"
                min="1"
                max="65535"
                class="w-32 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white focus:outline-none focus:border-blue-500"
                placeholder="8443"
                @input="handleChange"
              />
              <p class="mt-1 text-xs text-gray-400">
                Port for HTTPS server (default: 8443)
              </p>
            </div>
          </div>
        </div>
      </CollapsibleSection>

      <!-- Certificate Management (always visible — used by HTTPS and SOCKS5 TLS interception) -->
      <CollapsibleSection title="Certificate Management" :defaultOpen="false">
        <div class="space-y-6">
          <div class="p-3 bg-gray-700/50 rounded text-xs text-gray-300">
            The CA certificate is used by both <strong class="text-white">HTTPS</strong> (server TLS) and <strong class="text-white">SOCKS5</strong> (TLS interception of HTTPS domains). Configure it once here.
          </div>

          <!-- Certificate Mode -->
          <div>
            <h4 class="text-sm font-semibold text-white mb-3">Certificate Mode</h4>
            <ComboBox
              v-model="localSettings.certMode"
              :options="certModeOptions"
              @update:modelValue="handleChange"
            />

            <!-- Auto Mode Description -->
            <div v-if="localSettings.certMode === 'auto'" class="mt-3 p-3 bg-gray-700/50 rounded">
              <p class="text-xs text-gray-300">
                Automatically generates a CA certificate (persistent) and server certificate (regenerated on each start).
              </p>
            </div>

            <!-- CA Provided Mode -->
            <div v-if="localSettings.certMode === 'ca-provided'" class="mt-4 space-y-3">
              <div>
                <label class="block text-sm font-medium text-gray-300 mb-2">
                  CA Certificate (.pem/.crt)
                </label>
                <div class="flex gap-2">
                  <input
                    v-model="localSettings.certPaths.ca_cert_path"
                    type="text"
                    class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:border-blue-500"
                    placeholder="Select CA certificate file"
                    readonly
                  />
                  <button @click="selectCACert" class="px-3 py-2 bg-gray-600 hover:bg-gray-500 text-white text-sm rounded transition-colors">Browse</button>
                </div>
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-300 mb-2">
                  CA Private Key (.pem/.key)
                </label>
                <div class="flex gap-2">
                  <input
                    v-model="localSettings.certPaths.ca_key_path"
                    type="text"
                    class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:border-blue-500"
                    placeholder="Select CA private key file"
                    readonly
                  />
                  <button @click="selectCAKey" class="px-3 py-2 bg-gray-600 hover:bg-gray-500 text-white text-sm rounded transition-colors">Browse</button>
                </div>
              </div>
            </div>

            <!-- Cert Provided Mode -->
            <div v-if="localSettings.certMode === 'cert-provided'" class="mt-4 space-y-3">
              <div>
                <label class="block text-sm font-medium text-gray-300 mb-2">Server Certificate (.pem/.crt)</label>
                <div class="flex gap-2">
                  <input v-model="localSettings.certPaths.server_cert_path" type="text" class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:border-blue-500" placeholder="Select server certificate file" readonly />
                  <button @click="selectServerCert" class="px-3 py-2 bg-gray-600 hover:bg-gray-500 text-white text-sm rounded transition-colors">Browse</button>
                </div>
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-300 mb-2">Server Private Key (.pem/.key)</label>
                <div class="flex gap-2">
                  <input v-model="localSettings.certPaths.server_key_path" type="text" class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:border-blue-500" placeholder="Select server private key file" readonly />
                  <button @click="selectServerKey" class="px-3 py-2 bg-gray-600 hover:bg-gray-500 text-white text-sm rounded transition-colors">Browse</button>
                </div>
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-300 mb-2">Certificate Bundle (.pem) - Optional</label>
                <div class="flex gap-2">
                  <input v-model="localSettings.certPaths.server_bundle_path" type="text" class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:border-blue-500" placeholder="Select certificate bundle (optional)" readonly />
                  <button @click="selectServerBundle" class="px-3 py-2 bg-gray-600 hover:bg-gray-500 text-white text-sm rounded transition-colors">Browse</button>
                </div>
              </div>
            </div>
          </div>

          <!-- Certificate Names (CN/SAN) -->
          <div v-if="localSettings.certMode === 'auto' || localSettings.certMode === 'ca-provided'" class="border-t border-gray-700 pt-6">
            <h4 class="text-sm font-semibold text-white mb-3">Certificate Names (CN/SAN)</h4>
            <div class="p-3 bg-gray-700/50 rounded mb-3">
              <p class="text-sm font-medium text-gray-300 mb-2">Default Names:</p>
              <p class="text-xs text-gray-400 font-mono">
                {{ defaultCertNames.length > 0 ? defaultCertNames.join(', ') : 'Loading...' }}
              </p>
              <p class="text-xs text-gray-500 mt-2">
                Automatically includes: localhost, machine hostname, and interface IP to default gateway
              </p>
            </div>
            <label class="flex items-center gap-2 cursor-pointer mb-3">
              <input v-model="useCustomCertNames" type="checkbox" class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500" @change="handleChange" />
              <span class="text-sm font-medium text-white">Use Custom Names</span>
            </label>
            <div v-if="useCustomCertNames">
              <label class="block text-sm font-medium text-gray-300 mb-2">DNS Names and IP Addresses (comma-separated)</label>
              <input v-model="customCertNames" type="text" class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:border-blue-500" placeholder="e.g., example.com, 192.168.1.100, *.example.com" @input="handleChange" />
              <p class="mt-1 text-xs text-gray-400">Enter DNS names or IP addresses separated by commas. These will be used as Subject Alternative Names (SAN).</p>
            </div>
          </div>

          <!-- CA Certificate Status & Actions -->
          <div v-if="localSettings.certMode === 'auto' || localSettings.certMode === 'ca-provided'" class="border-t border-gray-700 pt-6">
            <h4 class="text-sm font-semibold text-white mb-3">CA Certificate</h4>
            <div class="p-3 bg-gray-700/50 rounded mb-4">
              <p class="text-sm text-gray-300">
                <span class="font-medium">Status:</span>
                <span v-if="isLoadingCAInfo" class="ml-2">Loading...</span>
                <span v-else-if="caInfo?.exists" class="ml-2 text-green-400">Generated</span>
                <span v-else class="ml-2 text-gray-400">Not generated</span>
              </p>
              <p v-if="caInfo?.exists && caInfo?.generated" class="text-sm text-gray-300 mt-1">
                <span class="font-medium">Generated:</span>
                <span class="ml-2">{{ formatTimestamp(caInfo.generated) }}</span>
              </p>
            </div>

            <div class="flex flex-wrap gap-2">
              <button @click="confirmRegenerateCA" class="px-3 py-2 bg-orange-600 hover:bg-orange-700 text-white text-sm rounded transition-colors flex items-center gap-2">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" /></svg>
                Regenerate CA
              </button>
              <button
                @click="downloadCA"
                :disabled="!caInfo?.exists"
                :class="['px-3 py-2 text-white text-sm rounded transition-colors flex items-center gap-2', caInfo?.exists ? 'bg-blue-600 hover:bg-blue-700' : 'bg-gray-600 cursor-not-allowed']"
              >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" /></svg>
                Download CA Cert
              </button>
            </div>

            <!-- Install buttons — vary by platform -->
            <div class="mt-3 flex flex-wrap gap-2">
              <!-- Windows: two install options (user vs system) -->
              <template v-if="runtimeOS === 'windows'">
                <button
                  @click="handleInstallCurrentUser"
                  :disabled="!caInfo?.exists"
                  :class="['px-3 py-2 text-white text-sm rounded transition-colors flex items-center gap-2', caInfo?.exists ? 'bg-green-700 hover:bg-green-600' : 'bg-gray-600 cursor-not-allowed']"
                  title="Installs to CurrentUser\Root — no UAC prompt needed"
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" /></svg>
                  Install (Current User)
                </button>
                <button
                  @click="confirmInstallElevated"
                  :disabled="!caInfo?.exists"
                  :class="['px-3 py-2 text-white text-sm rounded transition-colors flex items-center gap-2', caInfo?.exists ? 'bg-green-600 hover:bg-green-500' : 'bg-gray-600 cursor-not-allowed']"
                  title="Installs to LocalMachine\Root — triggers UAC prompt"
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" /></svg>
                  Install System-Wide (UAC)
                </button>
              </template>
              <!-- Linux / macOS: single system-wide install -->
              <template v-else>
                <button
                  @click="confirmInstallCASystem"
                  :disabled="!caInfo?.exists"
                  :class="['px-3 py-2 text-white text-sm rounded transition-colors flex items-center gap-2', caInfo?.exists ? 'bg-green-600 hover:bg-green-700' : 'bg-gray-600 cursor-not-allowed']"
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" /></svg>
                  Install CA (System-Wide)
                </button>
              </template>
              <!-- Manual install — all platforms -->
              <button
                @click="openManualInstall"
                :disabled="!caInfo?.exists"
                :class="['px-3 py-2 text-sm rounded transition-colors flex items-center gap-2', caInfo?.exists ? 'bg-gray-600 hover:bg-gray-500 text-gray-200' : 'bg-gray-700 text-gray-500 cursor-not-allowed']"
                title="View manual installation instructions and download install script"
              >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
                Install Manually
              </button>
            </div>

            <!-- Context note per platform -->
            <div class="mt-3 p-3 bg-blue-900/20 border border-blue-800 rounded text-xs text-blue-300 space-y-1">
              <template v-if="runtimeOS === 'windows'">
                <p><strong class="text-blue-200">Current User</strong> — no UAC needed, works for Chrome/Edge. Trusted for your account only.</p>
                <p><strong class="text-blue-200">System-Wide (UAC)</strong> — triggers a UAC prompt to install to Local Machine store. Trusted for all accounts.</p>
                <p class="text-blue-400">⚠ Firefox uses its own trust store — use "Install Manually" for Firefox instructions.</p>
              </template>
              <template v-else-if="runtimeOS === 'linux'">
                <p>Uses <code class="text-blue-200">pkexec</code> / <code class="text-blue-200">sudo</code> to install system-wide. Firefox requires separate manual import.</p>
              </template>
              <template v-else-if="runtimeOS === 'darwin'">
                <p>Uses <code class="text-blue-200">sudo security</code> to install to the System keychain. Firefox requires separate manual import.</p>
              </template>
            </div>
          </div>

          <!-- Error Message -->
          <div v-if="errorMessage" class="p-3 bg-red-900/20 border border-red-800 rounded">
            <p class="text-sm text-red-300">{{ errorMessage }}</p>
          </div>
        </div>
      </CollapsibleSection>

      <!-- CORS Section -->
      <CollapsibleSection title="CORS Settings" :defaultOpen="false">
        <div class="space-y-6">
          <!-- Enable CORS -->
          <div>
            <label class="flex items-center gap-2 cursor-pointer">
              <input
                v-model="localSettings.cors.enabled"
                type="checkbox"
                id="cors-enabled"
                class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500"
                @change="handleChange"
              />
              <span class="text-sm font-medium text-white">Enable Global CORS</span>
            </label>
            <p class="mt-1 text-xs text-gray-400 ml-6">
              Apply CORS headers to all responses (can be overridden per-entry or per-group)
            </p>
          </div>

          <div v-if="localSettings.cors.enabled" class="space-y-6">
            <!-- CORS Mode Tabs -->
            <div>
              <div class="flex border-b border-gray-700">
                <button
                  @click="localSettings.cors.mode = 'headers'; handleChange()"
                  :class="[
                    'px-4 py-2 text-sm font-medium border-b-2 transition-colors',
                    localSettings.cors.mode === 'headers'
                      ? 'border-blue-500 text-blue-500'
                      : 'border-transparent text-gray-400 hover:text-gray-300'
                  ]"
                >
                  Header List
                </button>
                <button
                  @click="localSettings.cors.mode = 'script'; handleChange()"
                  :class="[
                    'px-4 py-2 text-sm font-medium border-b-2 transition-colors',
                    localSettings.cors.mode === 'script'
                      ? 'border-blue-500 text-blue-500'
                      : 'border-transparent text-gray-400 hover:text-gray-300'
                  ]"
                >
                  Custom Script
                </button>
              </div>

              <!-- Mode Description -->
              <div class="mt-3 p-3 bg-gray-700/50 rounded">
                <p v-if="localSettings.cors.mode === 'headers'" class="text-xs text-gray-300">
                  Define CORS headers with JavaScript expressions evaluated per-request.
                </p>
                <p v-else class="text-xs text-gray-300">
                  Use custom JavaScript to set CORS headers with full request context.
                </p>
              </div>
            </div>

            <!-- Mode Content -->
            <div>
              <CORSHeaderList
                v-if="localSettings.cors.mode === 'headers'"
                :initial-headers="localSettings.cors.header_expressions"
                @validation-change="corsHeaderListValid = $event"
                @update:headers="handleCORSHeadersUpdate"
              />
              <CORSScript
                v-else
                :initial-script="localSettings.cors.script"
                @validation-change="corsScriptValid = $event"
                @update:script="handleCORSScriptUpdate"
              />
            </div>

            <!-- OPTIONS Response Status -->
            <div class="border-t border-gray-700 pt-6">
              <h4 class="text-sm font-semibold text-white mb-3">OPTIONS Preflight Response</h4>

              <label class="block text-sm font-medium text-gray-300 mb-2">
                Default Status Code
              </label>
              <div class="w-64">
                <StyledSelect
                  v-model="localSettings.cors.options_default_status"
                  :options="optionsStatusOptions"
                  @update:modelValue="handleChange"
                />
              </div>
              <p class="mt-2 text-xs text-gray-400">
                Status code returned for CORS preflight OPTIONS requests
              </p>
            </div>

            <!-- CORS Info -->
            <div class="p-4 bg-blue-900/20 border border-blue-800 rounded">
              <p class="text-sm font-medium text-blue-300 mb-2">CORS Precedence</p>
              <p class="text-xs text-blue-300">
                Explicit OPTIONS handler > Per-entry override > Per-group override > Global CORS
              </p>
            </div>
          </div>
        </div>
      </CollapsibleSection>

      <!-- SOCKS5 Section -->
      <CollapsibleSection title="SOCKS5 Proxy" :defaultOpen="false">
        <div class="space-y-6">
          <!-- Enable SOCKS5 -->
          <div>
            <label class="flex items-center gap-2 cursor-pointer">
              <input
                v-model="localSettings.socks5Config.enabled"
                type="checkbox"
                id="socks5-enabled"
                class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500"
                @change="handleChange"
              />
              <span class="text-sm font-medium text-white">Enable SOCKS5 Proxy</span>
            </label>
            <p class="mt-1 text-xs text-gray-400 ml-6">
              Allow browsers and apps to proxy HTTP/HTTPS requests through Mockelot
            </p>
          </div>

          <div v-if="localSettings.socks5Config.enabled" class="space-y-6">
            <!-- Port Configuration -->
            <div>
              <label class="block text-sm font-medium text-gray-300 mb-2">
                Port
              </label>
              <div class="flex gap-3 items-start">
                <div class="flex-1">
                  <input
                    v-model.number="localSettings.socks5Config.port"
                    type="number"
                    min="1"
                    max="65535"
                    :class="[
                      'w-full px-3 py-2 bg-gray-700 border rounded text-white',
                      socks5PortValid ? 'border-gray-600' : 'border-red-500'
                    ]"
                    @input="handleChange"
                  />
                  <p v-if="!socks5PortValid" class="mt-1 text-xs text-red-400">
                    Port must be between 1 and 65535
                  </p>
                </div>
                <button
                  @click="resetSOCKS5Port"
                  class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded transition-colors whitespace-nowrap"
                >
                  Reset to Default (1080)
                </button>
              </div>
            </div>

            <!-- Authentication -->
            <div>
              <label class="flex items-center gap-2 cursor-pointer">
                <input
                  v-model="localSettings.socks5Config.authentication"
                  type="checkbox"
                  id="socks5-auth"
                  class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500"
                  @change="handleChange"
                />
                <span class="text-sm font-medium text-white">Require Authentication</span>
              </label>
              <p class="mt-1 text-xs text-gray-400 ml-6">
                Require username/password for SOCKS5 connections
              </p>

              <!-- Credentials (shown only if auth enabled) -->
              <div v-if="localSettings.socks5Config.authentication" class="mt-3 ml-6 space-y-3">
                <div>
                  <label class="block text-sm font-medium text-gray-300 mb-2">
                    Username
                  </label>
                  <input
                    v-model="localSettings.socks5Config.username"
                    type="text"
                    placeholder="Enter username"
                    :class="[
                      'w-full px-3 py-2 bg-gray-700 border rounded text-white',
                      socks5AuthValid || !localSettings.socks5Config.authentication ? 'border-gray-600' : 'border-red-500'
                    ]"
                    @input="handleChange"
                  />
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-300 mb-2">
                    Password
                  </label>
                  <input
                    v-model="localSettings.socks5Config.password"
                    type="password"
                    placeholder="Enter password"
                    :class="[
                      'w-full px-3 py-2 bg-gray-700 border rounded text-white',
                      socks5AuthValid || !localSettings.socks5Config.authentication ? 'border-gray-600' : 'border-red-500'
                    ]"
                    @input="handleChange"
                  />
                </div>
                <p v-if="!socks5AuthValid" class="text-xs text-red-400">
                  Username and password are required when authentication is enabled
                </p>
              </div>
            </div>

            <!-- Domain Takeover List (Intercepted Domains) -->
            <div class="border-t border-gray-700 pt-6">
              <h4 class="text-sm font-semibold text-white mb-3">Intercepted Domains</h4>
              <p class="text-xs text-gray-400 mb-4">
                Configure which domains should be intercepted when using SOCKS5 proxy
              </p>

              <div class="overflow-x-auto">
                <table class="w-full text-sm">
                  <thead class="text-xs uppercase text-gray-400 bg-gray-700/50">
                    <tr>
                      <th class="px-3 py-2 text-left">Domain Pattern (regex)</th>
                      <th class="px-3 py-2 text-center w-32">Overlay Mode</th>
                      <th class="px-3 py-2 text-center w-24">Enabled</th>
                      <th class="px-3 py-2 text-center w-24">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="domain in domains"
                      :key="domain.id"
                      class="border-t border-gray-700 hover:bg-gray-700/30"
                    >
                      <td class="px-3 py-2">
                        <input
                          v-model="domain.pattern"
                          type="text"
                          placeholder="e.g., api\.example\.com"
                          class="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-sm"
                          @input="handleChange"
                        />
                      </td>
                      <td class="px-3 py-2 text-center">
                        <input
                          v-model="domain.overlayMode"
                          type="checkbox"
                          class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500"
                          @change="handleChange"
                        />
                      </td>
                      <td class="px-3 py-2 text-center">
                        <input
                          v-model="domain.enabled"
                          type="checkbox"
                          class="w-4 h-4 rounded bg-gray-700 border-gray-600 text-blue-600 focus:ring-blue-500"
                          @change="handleChange"
                        />
                      </td>
                      <td class="px-3 py-2 text-center">
                        <button
                          @click="removeDomain(domain.id)"
                          class="px-2 py-1 bg-red-600 hover:bg-red-700 text-white rounded text-xs transition-colors"
                        >
                          Delete
                        </button>
                      </td>
                    </tr>
                    <tr v-if="domains.length === 0" class="border-t border-gray-700">
                      <td colspan="4" class="px-3 py-4 text-center text-gray-500 text-sm">
                        No domains configured. Click "Add Domain" to get started.
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>

              <button
                @click="addDomain"
                class="mt-3 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded transition-colors text-sm"
              >
                Add Domain
              </button>
              <p class="mt-2 text-xs text-gray-400">
                New domains default to overlay mode ON (pass through to real server if no endpoint matches)
              </p>
            </div>

            <!-- Hosts File Helper -->
            <div class="border-t border-gray-700 pt-6">
              <h4 class="text-sm font-semibold text-white mb-3">Hosts File Helper</h4>
              <p class="text-xs text-gray-400 mb-3">
                For apps that don't support SOCKS5, add these entries to your hosts file:
              </p>

              <textarea
                readonly
                :value="hostsFileEntries"
                rows="5"
                class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm font-mono"
                placeholder="No enabled domains configured"
              />

              <button
                @click="copyHostsEntries"
                :disabled="hostsFileEntries === ''"
                :class="[
                  'mt-2 px-4 py-2 rounded transition-colors text-sm',
                  hostsFileEntries !== ''
                    ? 'bg-gray-700 hover:bg-gray-600 text-gray-300'
                    : 'bg-gray-800 text-gray-600 cursor-not-allowed'
                ]"
              >
                Copy to Clipboard
              </button>

              <div class="mt-4 p-3 bg-gray-700/50 rounded text-xs text-gray-300 space-y-1">
                <p><strong class="text-white">Windows:</strong> C:\Windows\System32\drivers\etc\hosts</p>
                <p><strong class="text-white">Linux/macOS:</strong> /etc/hosts</p>
                <p class="text-gray-400 mt-2">Note: Editing hosts file requires administrator/root privileges</p>
              </div>
            </div>

            <!-- Browser Configuration Instructions -->
            <div class="border-t border-gray-700 pt-6">
              <h4 class="text-sm font-semibold text-white mb-3">Browser Setup</h4>
              <p class="text-xs text-gray-400 mb-3">
                Configure your browser's SOCKS5 proxy:
              </p>

              <div class="p-3 bg-gray-700/50 rounded">
                <code class="text-sm text-blue-300">Host: localhost, Port: {{ localSettings.socks5Config.port }}</code>
              </div>

              <details class="mt-3">
                <summary class="cursor-pointer text-sm text-blue-400 hover:text-blue-300">
                  Browser-specific instructions
                </summary>
                <div class="mt-3 space-y-3 text-xs text-gray-300">
                  <div>
                    <strong class="text-white">Firefox:</strong>
                    <p class="ml-4 mt-1">Settings > Network Settings > Manual proxy configuration</p>
                    <p class="ml-4">Set "SOCKS Host" to "localhost" and Port to "{{ localSettings.socks5Config.port }}"</p>
                    <p class="ml-4">Select "SOCKS v5" and enable "Proxy DNS when using SOCKS v5"</p>
                  </div>
                  <div>
                    <strong class="text-white">Chrome/Edge:</strong>
                    <p class="ml-4 mt-1">Use a browser extension like "Proxy SwitchyOmega" or</p>
                    <p class="ml-4">Configure system proxy settings (OS-level)</p>
                  </div>
                  <div>
                    <strong class="text-white">cURL:</strong>
                    <p class="ml-4 mt-1">
                      <code class="text-blue-300">curl --socks5 localhost:{{ localSettings.socks5Config.port }} https://api.example.com</code>
                    </p>
                  </div>
                </div>
              </details>
            </div>

            <!-- How it Works Info -->
            <div class="p-4 bg-blue-900/20 border border-blue-800 rounded">
              <p class="text-sm font-medium text-blue-300 mb-2">How it Works</p>
              <ul class="text-xs text-blue-300 space-y-1 list-disc list-inside">
                <li>Browser connects to Mockelot via SOCKS5</li>
                <li>Requests to intercepted domains are routed through your endpoints</li>
                <li>Overlay mode passes unmatched requests to real servers</li>
                <li>Non-intercepted domains pass through transparently</li>
              </ul>
            </div>
          </div>
        </div>
      </CollapsibleSection>

      <!-- WebSocket Section -->
      <CollapsibleSection title="WebSocket" :defaultOpen="false">
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-300 mb-1">
              Capture size per message
            </label>
            <p class="text-xs text-gray-500 mb-2">
              Maximum bytes stored per WebSocket frame for inspection and export.
              Large frames are truncated; binary frames are stored as hex.
            </p>
            <div class="flex items-center gap-3">
              <input
                v-model.number="localSettings.wsCaptureBytes"
                type="number"
                min="64"
                max="1048576"
                step="1024"
                class="w-32 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                @input="handleChange"
              />
              <span class="text-xs text-gray-400">bytes (default: 1024)</span>
              <div class="flex gap-1">
                <button
                  v-for="preset in [512, 1024, 4096, 16384, 65536]"
                  :key="preset"
                  @click="localSettings.wsCaptureBytes = preset; handleChange()"
                  :class="[
                    'px-2 py-0.5 rounded text-xs transition-colors',
                    localSettings.wsCaptureBytes === preset
                      ? 'bg-blue-600 text-white'
                      : 'bg-gray-700 hover:bg-gray-600 text-gray-300'
                  ]"
                >
                  {{ preset >= 1024 ? (preset / 1024) + 'K' : preset + 'B' }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </CollapsibleSection>

      <!-- DNS Section -->
      <CollapsibleSection title="DNS Resolution" :defaultOpen="false">
        <DNSSection
          :config="localSettings.dnsOverrides"
          :providers="dnsProviders"
          @update:config="updateDNSConfig"
          @change="handleChange"
        />
      </CollapsibleSection>
    </div>

    <!-- Locked Footer with APPLY/RESET Buttons -->
    <div class="flex-shrink-0 border-t border-gray-700 p-4 bg-gray-800">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span v-if="hasChanges" class="text-xs text-yellow-400">Unapplied changes</span>
        </div>
        <div class="flex gap-2">
          <button
            :disabled="!hasChanges"
            :class="[
              'px-4 py-2 rounded font-medium transition-colors',
              hasChanges
                ? 'bg-gray-600 hover:bg-gray-500 text-white cursor-pointer'
                : 'bg-gray-700 text-gray-500 cursor-not-allowed'
            ]"
            @click="handleReset"
          >
            RESET
          </button>
          <button
            :disabled="!hasChanges"
            :class="[
              'px-4 py-2 rounded font-medium transition-colors',
              hasChanges
                ? 'bg-blue-600 hover:bg-blue-700 text-white cursor-pointer'
                : 'bg-gray-600 text-gray-400 cursor-not-allowed'
            ]"
            @click="handleSave"
          >
            APPLY
          </button>
        </div>
      </div>
    </div>

    <!-- Regenerate CA Confirmation Dialog -->
    <ConfirmDialog
      :show="showRegenerateConfirm"
      title="Regenerate CA Certificate?"
      message="This will invalidate all existing client trust. HTTPS will restart. Continue?"
      primary-text="Regenerate"
      cancel-text="Cancel"
      @primary="handleRegenerateCA"
      @cancel="cancelRegenerateCA"
    />

    <!-- Install CA System Confirmation Dialog (Linux/macOS) -->
    <ConfirmDialog
      :show="showInstallConfirm"
      title="Install CA Certificate (System-Wide)?"
      message="This will install the Mockelot CA certificate at the system level. This requires administrator/root privileges and will prompt for your password. All applications on this system will trust certificates signed by this CA. Continue?"
      primary-text="Install"
      cancel-text="Cancel"
      @primary="handleInstallCASystem"
      @cancel="cancelInstallCASystem"
    />

    <!-- Install CA System Elevated (Windows UAC) Confirmation Dialog -->
    <ConfirmDialog
      :show="showInstallElevatedConfirm"
      title="Install CA Certificate (System-Wide)?"
      message="A UAC prompt will appear asking for administrator permission. This installs the Mockelot CA to the Local Machine certificate store, trusted by all accounts on this computer. Continue?"
      primary-text="Install (UAC)"
      cancel-text="Cancel"
      @primary="handleInstallElevated"
      @cancel="cancelInstallElevated"
    />

    <!-- Manual Install Modal -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="showManualInstallModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div class="bg-gray-800 rounded-lg shadow-2xl border border-gray-700 flex flex-col max-h-[85vh] w-full max-w-2xl mx-4">
            <!-- Header -->
            <div class="flex items-center justify-between px-6 py-4 border-b border-gray-700 flex-shrink-0">
              <div>
                <h3 class="text-base font-semibold text-white">Manual CA Certificate Installation</h3>
                <p class="text-xs text-gray-400 mt-0.5">
                  CA cert location: <code class="text-cyan-300 font-mono">{{ manualInstallScript.certPath }}</code>
                </p>
              </div>
              <button @click="showManualInstallModal = false" class="p-1 text-gray-400 hover:text-white rounded hover:bg-gray-700 transition-colors">
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
              </button>
            </div>

            <!-- Body -->
            <div class="flex-1 overflow-y-auto px-6 py-4 space-y-5">

              <!-- Windows instructions -->
              <template v-if="runtimeOS === 'windows'">
                <div class="space-y-3 text-sm">
                  <h4 class="font-semibold text-white">Option A — PowerShell (Recommended)</h4>
                  <p class="text-gray-300">Run the script below in PowerShell. It will prompt you to choose current-user or system-wide install.</p>
                  <ol class="list-decimal list-inside space-y-1 text-gray-300 text-xs ml-2">
                    <li>Copy the script below and save as <code class="text-cyan-300">install-mockelot-ca.ps1</code></li>
                    <li>Right-click the file → <strong>Run with PowerShell</strong></li>
                    <li>Follow the on-screen prompts</li>
                  </ol>

                  <h4 class="font-semibold text-white mt-4">Option B — Quick one-liner (Current User, no UAC)</h4>
                  <div class="bg-gray-900 rounded p-3 font-mono text-xs text-cyan-300 break-all select-all">
                    Import-Certificate -FilePath '{{ manualInstallScript.certPath }}' -CertStoreLocation Cert:\CurrentUser\Root
                  </div>

                  <h4 class="font-semibold text-white mt-4">Option C — Firefox</h4>
                  <p class="text-gray-300 text-xs">Firefox has its own trust store and ignores the Windows cert store.</p>
                  <ol class="list-decimal list-inside space-y-1 text-gray-300 text-xs ml-2">
                    <li>Open Firefox → Settings → Privacy &amp; Security</li>
                    <li>Scroll to <strong>Certificates</strong> → click <strong>View Certificates</strong></li>
                    <li>Go to <strong>Authorities</strong> tab → click <strong>Import</strong></li>
                    <li>Select: <code class="text-cyan-300 font-mono">{{ manualInstallScript.certPath }}</code></li>
                    <li>Check <strong>"Trust this CA to identify websites"</strong> → OK</li>
                  </ol>
                </div>
              </template>

              <!-- Linux instructions -->
              <template v-else-if="runtimeOS === 'linux'">
                <div class="space-y-3 text-sm">
                  <h4 class="font-semibold text-white">Option A — Shell Script (Recommended)</h4>
                  <p class="text-gray-300">Run the script below in a terminal. Requires sudo.</p>

                  <h4 class="font-semibold text-white mt-4">Option B — Manual commands</h4>
                  <div class="bg-gray-900 rounded p-3 font-mono text-xs text-cyan-300 space-y-1">
                    <div>sudo cp '{{ manualInstallScript.certPath }}' /usr/local/share/ca-certificates/mockelot-ca.crt</div>
                    <div>sudo update-ca-certificates</div>
                  </div>

                  <h4 class="font-semibold text-white mt-4">Firefox</h4>
                  <ol class="list-decimal list-inside space-y-1 text-gray-300 text-xs ml-2">
                    <li>Open Firefox → Settings → Privacy &amp; Security → Certificates → View Certificates</li>
                    <li>Authorities tab → Import → select: <code class="text-cyan-300 font-mono">{{ manualInstallScript.certPath }}</code></li>
                    <li>Check <strong>"Trust this CA to identify websites"</strong> → OK</li>
                  </ol>
                </div>
              </template>

              <!-- macOS instructions -->
              <template v-else-if="runtimeOS === 'darwin'">
                <div class="space-y-3 text-sm">
                  <h4 class="font-semibold text-white">Option A — Shell Script (Recommended)</h4>
                  <p class="text-gray-300">Run the script below in Terminal. Requires sudo.</p>

                  <h4 class="font-semibold text-white mt-4">Option B — Keychain Access GUI</h4>
                  <ol class="list-decimal list-inside space-y-1 text-gray-300 text-xs ml-2">
                    <li>Open <strong>Keychain Access</strong> (Spotlight → Keychain Access)</li>
                    <li>Select <strong>System</strong> keychain on the left</li>
                    <li>File → Import Items → select: <code class="text-cyan-300 font-mono">{{ manualInstallScript.certPath }}</code></li>
                    <li>Double-click the imported cert → Expand <strong>Trust</strong></li>
                    <li>Set <strong>"When using this certificate"</strong> → <strong>Always Trust</strong></li>
                  </ol>

                  <h4 class="font-semibold text-white mt-4">Firefox</h4>
                  <p class="text-gray-300 text-xs">Same as other platforms — import via Firefox's own certificate manager (see Linux instructions above).</p>
                </div>
              </template>

              <!-- Script display -->
              <div v-if="manualInstallScript.script" class="border-t border-gray-700 pt-4">
                <div class="flex items-center justify-between mb-2">
                  <h4 class="text-sm font-semibold text-white">
                    Install Script
                    <span class="ml-2 text-xs font-normal text-gray-400">({{ manualInstallScript.filename }})</span>
                  </h4>
                  <button
                    @click="copyScript"
                    class="px-3 py-1 bg-gray-600 hover:bg-gray-500 text-gray-200 text-xs rounded transition-colors flex items-center gap-1"
                  >
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" /></svg>
                    Copy Script
                  </button>
                </div>
                <pre class="bg-gray-900 rounded p-3 text-xs text-gray-300 overflow-x-auto whitespace-pre-wrap max-h-64 font-mono select-all">{{ manualInstallScript.script }}</pre>
              </div>
            </div>

            <!-- Footer -->
            <div class="px-6 py-3 border-t border-gray-700 flex justify-end flex-shrink-0">
              <button @click="showManualInstallModal = false" class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white text-sm rounded transition-colors">Close</button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.modal-enter-active, .modal-leave-active { transition: opacity 0.2s ease; }
.modal-enter-from, .modal-leave-to { opacity: 0; }
</style>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useServerStore } from '../../../stores/server'
import CollapsibleSection from '../../shared/CollapsibleSection.vue'
import ComboBox from '../../shared/ComboBox.vue'
import StyledSelect from '../../shared/StyledSelect.vue'
import ConfirmDialog from '../../dialogs/ConfirmDialog.vue'
import CORSHeaderList from '../../dialogs/CORSHeaderList.vue'
import CORSScript from '../../dialogs/CORSScript.vue'
import DNSSection from './DNSSection.vue'
import { UpdateServerSettings, GetCACertInfo, RegenerateCA, DownloadCACert, InstallCACertSystem, InstallCACertCurrentUser, InstallCACertSystemElevated, GetRuntimeOS, GetCACertInstallScript, SelectCertFile, GetDefaultCertNames, GetDNSProviders, GetDNSOverrides, UpdateDNSOverrides } from '../../../../wailsjs/go/main/App'
import { models } from '../../../../wailsjs/go/models'
import { v4 as uuidv4 } from 'uuid'

const serverStore = useServerStore()

// CA Certificate Info
const caInfo = ref<models.CACertInfo | null>(null)
const isLoadingCAInfo = ref(false)

// Certificate Names (CN/SAN)
const useCustomCertNames = ref(false)
const customCertNames = ref('')
const defaultCertNames = ref<string[]>([])

// UI State
const showRegenerateConfirm = ref(false)
const showInstallConfirm = ref(false)
const showInstallElevatedConfirm = ref(false)
const showManualInstallModal = ref(false)
const errorMessage = ref('')

// Runtime OS and manual install script
const runtimeOS = ref('')
const manualInstallScript = ref<Record<string, string>>({})

async function loadRuntimeOS() {
  try {
    runtimeOS.value = await GetRuntimeOS()
  } catch (e) {
    console.error('Failed to get runtime OS:', e)
  }
}

// Install for current user only (no elevation)
async function handleInstallCurrentUser() {
  try {
    await InstallCACertCurrentUser()
    errorMessage.value = ''
  } catch (error) {
    errorMessage.value = `Failed to install certificate for current user: ${error}`
  }
}

// Install system-wide with elevation
function confirmInstallElevated() {
  showInstallElevatedConfirm.value = true
}

async function handleInstallElevated() {
  showInstallElevatedConfirm.value = false
  try {
    await InstallCACertSystemElevated()
    errorMessage.value = ''
  } catch (error) {
    errorMessage.value = `Failed to install certificate system-wide: ${error}`
  }
}

function cancelInstallElevated() {
  showInstallElevatedConfirm.value = false
}

// Manual install modal
async function openManualInstall() {
  try {
    manualInstallScript.value = await GetCACertInstallScript()
    showManualInstallModal.value = true
  } catch (error) {
    errorMessage.value = `Failed to generate install script: ${error}`
  }
}

async function copyScript() {
  try {
    await navigator.clipboard.writeText(manualInstallScript.value.script || '')
  } catch (e) {
    console.error('Clipboard copy failed:', e)
  }
}

// CORS validation state
const corsHeaderListValid = ref(true)
const corsScriptValid = ref(true)

// Local state (editable copy of config)
const localSettings = ref({
  serverMode: 'http' as 'http' | 'socks5',
  port: 8080,
  http2Enabled: false,
  httpsEnabled: false,
  httpsPort: 8443,
  httpToHttpsRedirect: false,
  certMode: 'auto',
  certPaths: new models.CertPaths({
    ca_cert_path: '',
    ca_key_path: '',
    server_cert_path: '',
    server_key_path: '',
    server_bundle_path: '',
  }),
  certNames: [] as string[],
  cors: {
    enabled: false,
    mode: 'headers',
    header_expressions: [] as Array<{ name: string; expression: string }>,
    script: '',
    options_default_status: 200,
  },
  socks5Config: {
    enabled: false,
    port: 1080,
    authentication: false,
    username: '',
    password: '',
  },
  wsCaptureBytes: 1024,
  dnsOverrides: {
    enabled: false,
    overrides: [] as Array<{
      id: string
      pattern: string
      type: 'static' | 'cname'
      target: string
      priority: number
      enabled: boolean
    }>,
    upstreamServers: [] as string[],
    useSystemDNS: true,
  },
})

// Domain takeover state (part of SOCKS5)
const domains = ref<Array<{
  id: string
  pattern: string
  overlayMode: boolean
  enabled: boolean
}>>([])

// DNS state
const dnsProviders = ref<Record<string, models.DNSProvider>>({})

// Saved state (last saved or loaded from config)
const savedSettings = ref(JSON.parse(JSON.stringify(localSettings.value)))
const savedDomains = ref(JSON.parse(JSON.stringify(domains.value)))

// Has changes (dirty state)
const hasChanges = computed(() => {
  return JSON.stringify(localSettings.value) !== JSON.stringify(savedSettings.value) ||
         JSON.stringify(domains.value) !== JSON.stringify(savedDomains.value)
})

// SOCKS5 validation
const socks5PortValid = computed(() => {
  return localSettings.value.socks5Config.port >= 1 && localSettings.value.socks5Config.port <= 65535
})

const socks5AuthValid = computed(() => {
  if (!localSettings.value.socks5Config.authentication) return true
  return localSettings.value.socks5Config.username.trim() !== '' && localSettings.value.socks5Config.password.trim() !== ''
})

// Options for dropdowns
const certModeOptions = [
  { value: 'auto', label: 'Auto-generate (default)' },
  { value: 'ca-provided', label: 'Provide CA Cert + Key' },
  { value: 'cert-provided', label: 'Provide Server Cert + Key + Bundle' },
]

// OPTIONS status options
const optionsStatusOptions = computed(() => [
  {
    value: 200,
    label: '200 OK (Default)',
    description: 'Standard success response'
  },
  {
    value: 204,
    label: '204 No Content',
    description: 'Success with no body'
  }
])

// Hosts file helper
const hostsFileEntries = computed(() => {
  return domains.value
    .filter(d => d.enabled && d.pattern.trim() !== '')
    .map(d => `127.0.0.1 ${d.pattern}`)
    .join('\n')
})


// Load DNS providers
async function loadDNSProviders() {
  try {
    const providers = await GetDNSProviders()
    dnsProviders.value = providers || {}
  } catch (error) {
    console.error('Failed to load DNS providers:', error)
  }
}

// Load CA info
async function loadCAInfo() {
  isLoadingCAInfo.value = true
  try {
    caInfo.value = await GetCACertInfo()
  } catch (error) {
    console.error('Failed to load CA info:', error)
  } finally {
    isLoadingCAInfo.value = false
  }
}

// Load default cert names
async function loadDefaultCertNames() {
  try {
    defaultCertNames.value = await GetDefaultCertNames()
  } catch (error) {
    console.error('Failed to load default cert names:', error)
  }
}

// Regenerate CA
function confirmRegenerateCA() {
  showRegenerateConfirm.value = true
}

async function handleRegenerateCA() {
  showRegenerateConfirm.value = false
  try {
    await RegenerateCA()
    await loadCAInfo()
    errorMessage.value = ''
  } catch (error) {
    errorMessage.value = `Failed to regenerate CA: ${error}`
  }
}

function cancelRegenerateCA() {
  showRegenerateConfirm.value = false
}

// Download CA
async function downloadCA() {
  try {
    const path = await DownloadCACert()
    if (path) {
      console.log('CA certificate saved to:', path)
    }
    errorMessage.value = ''
  } catch (error) {
    errorMessage.value = `Failed to download CA: ${error}`
  }
}

// Install CA System
function confirmInstallCASystem() {
  showInstallConfirm.value = true
}

async function handleInstallCASystem() {
  showInstallConfirm.value = false
  try {
    await InstallCACertSystem()
    errorMessage.value = ''
    console.log('CA certificate installed successfully')
  } catch (error) {
    errorMessage.value = `Failed to install CA certificate: ${error}`
  }
}

function cancelInstallCASystem() {
  showInstallConfirm.value = false
}

// File selection
async function selectCACert() {
  try {
    const path = await SelectCertFile('Select CA Certificate')
    if (path) {
      localSettings.value.certPaths.ca_cert_path = path
      handleChange()
    }
  } catch (error) {
    errorMessage.value = `Failed to select file: ${error}`
  }
}

async function selectCAKey() {
  try {
    const path = await SelectCertFile('Select CA Private Key')
    if (path) {
      localSettings.value.certPaths.ca_key_path = path
      handleChange()
    }
  } catch (error) {
    errorMessage.value = `Failed to select file: ${error}`
  }
}

async function selectServerCert() {
  try {
    const path = await SelectCertFile('Select Server Certificate')
    if (path) {
      localSettings.value.certPaths.server_cert_path = path
      handleChange()
    }
  } catch (error) {
    errorMessage.value = `Failed to select file: ${error}`
  }
}

async function selectServerKey() {
  try {
    const path = await SelectCertFile('Select Server Private Key')
    if (path) {
      localSettings.value.certPaths.server_key_path = path
      handleChange()
    }
  } catch (error) {
    errorMessage.value = `Failed to select file: ${error}`
  }
}

async function selectServerBundle() {
  try {
    const path = await SelectCertFile('Select Certificate Bundle')
    if (path) {
      localSettings.value.certPaths.server_bundle_path = path
      handleChange()
    }
  } catch (error) {
    errorMessage.value = `Failed to select file: ${error}`
  }
}

// Format timestamp
function formatTimestamp(timestamp: string): string {
  if (!timestamp) return 'Not generated'
  const date = new Date(timestamp)
  return date.toLocaleString()
}

// CORS handlers
function handleCORSHeadersUpdate(headers: Array<{name: string, expression: string}>) {
  localSettings.value.cors.header_expressions = headers
  handleChange()
}

function handleCORSScriptUpdate(script: string) {
  localSettings.value.cors.script = script
  handleChange()
}

// DNS config update
function updateDNSConfig(newConfig: typeof localSettings.value.dnsOverrides) {
  localSettings.value.dnsOverrides = newConfig
  handleChange()
}

// SOCKS5 domain management
function addDomain() {
  const newId = 'domain-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9)
  domains.value.push({
    id: newId,
    pattern: '',
    overlayMode: true,
    enabled: true
  })
}

function removeDomain(id: string) {
  const index = domains.value.findIndex(d => d.id === id)
  if (index !== -1) {
    domains.value.splice(index, 1)
  }
}

function resetSOCKS5Port() {
  localSettings.value.socks5Config.port = 1080
  handleChange()
}

async function copyHostsEntries() {
  try {
    await navigator.clipboard.writeText(hostsFileEntries.value)
    console.log('Hosts file entries copied to clipboard')
  } catch (error) {
    console.error('Failed to copy to clipboard:', error)
  }
}

// Initialize from store config
onMounted(async () => {
  loadCAInfo()
  loadDefaultCertNames()
  loadRuntimeOS()
  await loadDNSProviders()
  // Always fetch fresh config on mount so domain takeover list (which can be
  // updated externally via AddDomainToSOCKS5Takeover) is never stale.
  const freshConfig = await serverStore.refreshConfig()
  loadFromConfig(freshConfig ?? serverStore.config)
})



// Watch store config changes
watch(() => serverStore.config, (newConfig) => {
  if (newConfig) {
    loadFromConfig(newConfig)
  }
})

function loadFromConfig(config: models.AppConfig) {
  localSettings.value = {
    serverMode: (config.server_mode as 'http' | 'socks5') || 'http',
    port: config.port || 8080,
    http2Enabled: config.http2_enabled || false,
    httpsEnabled: config.https_enabled || false,
    httpsPort: config.https_port || 8443,
    httpToHttpsRedirect: config.http_to_https_redirect || false,
    certMode: config.cert_mode || 'auto',
    certPaths: config.cert_paths || new models.CertPaths({
      ca_cert_path: '',
      ca_key_path: '',
      server_cert_path: '',
      server_key_path: '',
      server_bundle_path: '',
    }),
    certNames: config.cert_names || [],
    cors: {
      enabled: config.cors?.enabled || false,
      mode: config.cors?.mode || 'headers',
      header_expressions: config.cors?.header_expressions || [],
      script: config.cors?.script || '',
      options_default_status: config.cors?.options_default_status || 200,
    },
    socks5Config: {
      enabled: config.socks5_config?.enabled || false,
      port: config.socks5_config?.port || 1080,
      authentication: config.socks5_config?.authentication || false,
      username: config.socks5_config?.username || '',
      password: config.socks5_config?.password || '',
    },
    wsCaptureBytes: config.ws_capture_bytes || 1024,
    dnsOverrides: {
      enabled: config.dns_overrides?.enabled || false,
      overrides: config.dns_overrides?.overrides?.map((o: any) => ({
        id: o.id || uuidv4(),
        pattern: o.pattern || '',
        type: (o.type || 'static') as 'static' | 'cname',
        target: o.target || '',
        priority: o.priority ?? 0,
        enabled: o.enabled ?? true
      })) || [],
      upstreamServers: config.dns_overrides?.upstream_servers || [],
      useSystemDNS: config.dns_overrides?.use_system_dns ?? true,
    },
  }

  // Load DNS provider selection
  if (config.dns_overrides?.use_system_dns) {
    selectedDNSProvider.value = 'system'
    customDNSServers.value = ''
  } else if (config.dns_overrides?.upstream_servers && config.dns_overrides.upstream_servers.length > 0) {
    // Check if these match a known provider
    const serversStr = config.dns_overrides.upstream_servers.join(',')
    let foundProvider = false

    for (const [key, provider] of Object.entries(dnsProviders.value)) {
      if (provider.servers.join(',') === serversStr) {
        selectedDNSProvider.value = key
        foundProvider = true
        break
      }
    }

    if (!foundProvider) {
      selectedDNSProvider.value = 'custom'
      customDNSServers.value = config.dns_overrides.upstream_servers.join('\n')
    }
  }

  // Load domain takeover
  if (config.domain_takeover && config.domain_takeover.domains) {
    domains.value = config.domain_takeover.domains.map((d: any) => ({
      id: d.id,
      pattern: d.pattern,
      overlayMode: d.overlay_mode,
      enabled: d.enabled
    }))
  } else {
    domains.value = []
  }

  // Load certificate names if available
  if (config.cert_names && config.cert_names.length > 0) {
    useCustomCertNames.value = true
    customCertNames.value = config.cert_names.join(', ')
  } else {
    useCustomCertNames.value = false
    customCertNames.value = ''
  }

  // Save as clean state
  savedSettings.value = JSON.parse(JSON.stringify(localSettings.value))
  savedDomains.value = JSON.parse(JSON.stringify(domains.value))
}

function handleChange() {
  // Changes are tracked automatically via computed hasChanges
}

function handleReset() {
  localSettings.value = JSON.parse(JSON.stringify(savedSettings.value))
  domains.value = JSON.parse(JSON.stringify(savedDomains.value))
}

async function handleSave() {
  if (!hasChanges.value) return

  try {
    // Build cert_names array
    const certNames = useCustomCertNames.value && customCertNames.value
      ? customCertNames.value.split(',').map(s => s.trim()).filter(s => s !== '')
      : []

    // Build ServerSettings object
    const settings: any = {
      server_mode: localSettings.value.serverMode,
      port: localSettings.value.port,
      http2_enabled: localSettings.value.http2Enabled,
      https_enabled: localSettings.value.httpsEnabled,
      https_port: localSettings.value.httpsPort,
      http_to_https_redirect: localSettings.value.httpToHttpsRedirect,
      cert_mode: localSettings.value.certMode,
      cert_paths: localSettings.value.certPaths,
      cert_names: certNames,
      cors: {
        enabled: localSettings.value.cors.enabled,
        mode: localSettings.value.cors.mode,
        header_expressions: localSettings.value.cors.header_expressions,
        script: localSettings.value.cors.script,
        options_default_status: localSettings.value.cors.options_default_status,
      },
      socks5_config: {
        enabled: localSettings.value.socks5Config.enabled,
        port: localSettings.value.socks5Config.port,
        authentication: localSettings.value.socks5Config.authentication,
        username: localSettings.value.socks5Config.username,
        password: localSettings.value.socks5Config.password,
      },
      ws_capture_bytes: localSettings.value.wsCaptureBytes,
      domain_takeover: new models.DomainTakeoverConfig({
        domains: domains.value.map(d => new models.DomainConfig({
          id: d.id,
          pattern: d.pattern,
          overlay_mode: d.overlayMode,
          enabled: d.enabled,
        })),
      }),
    }

    // Call backend to update settings
    await UpdateServerSettings(settings)

    // Save DNS configuration
    const dnsConfig = new models.DNSOverrideConfig({
      enabled: true, // DNS is always enabled now
      overrides: localSettings.value.dnsOverrides.overrides.map(o => new models.DNSOverride({
        id: o.id,
        pattern: o.pattern,
        type: o.type,
        target: o.target,
        priority: o.priority,
        enabled: o.enabled
      })),
      upstream_servers: localSettings.value.dnsOverrides.upstreamServers,
      use_system_dns: localSettings.value.dnsOverrides.useSystemDNS
    })

    await UpdateDNSOverrides(dnsConfig)

    // Mark dirty in store
    serverStore.markDirty()

    // Update saved state
    savedSettings.value = JSON.parse(JSON.stringify(localSettings.value))
    savedDomains.value = JSON.parse(JSON.stringify(domains.value))

    // Refresh config
    await serverStore.refreshConfig()
  } catch (error) {
    console.error('Error updating server settings:', error)
    errorMessage.value = `Failed to save settings: ${error}`
  }
}
</script>
