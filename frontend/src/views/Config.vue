<template>
  <v-container fluid class="config-page">
    <v-alert density="compact" type="info" variant="tonal" class="mb-4">
      {{ $t('configEditor.dualTip') }}
    </v-alert>

    <v-row align="stretch">
      <v-col cols="12" lg="6" class="d-flex">
        <v-card class="rounded-lg config-card d-flex flex-column" variant="outlined">
          <v-toolbar density="comfortable" color="transparent">
            <v-toolbar-title>{{ $t('configEditor.sourceTitle') }}</v-toolbar-title>
            <v-chip v-if="dirty" color="warning" size="small" variant="tonal">
              {{ $t('configEditor.unsaved') }}
            </v-chip>
            <v-chip v-else-if="pendingReload" color="info" size="small" variant="tonal">
              {{ $t('configEditor.pendingReload') }}
            </v-chip>
            <v-chip v-else color="success" size="small" variant="tonal">
              {{ $t('configEditor.saved') }}
            </v-chip>
          </v-toolbar>

          <v-divider />

          <v-card-actions class="toolbar-actions px-3 py-2">
            <v-btn size="small" variant="text" prepend-icon="mdi-refresh" :loading="loading" @click="loadConfig">
              {{ $t('configEditor.read') }}
            </v-btn>
            <v-btn size="small" variant="text" prepend-icon="mdi-content-copy" @click="copyConfig">
              {{ $t('configEditor.copy') }}
            </v-btn>
            <v-btn size="small" variant="text" prepend-icon="mdi-content-paste" @click="pasteConfig">
              {{ $t('configEditor.paste') }}
            </v-btn>
            <v-spacer />
            <v-btn size="small" variant="tonal" color="info" prepend-icon="mdi-check-decagram" :loading="checking" @click="checkConfig">
              {{ $t('configEditor.check') }}
            </v-btn>
            <v-btn size="small" variant="tonal" color="primary" prepend-icon="mdi-content-save" :loading="saving" @click="saveConfig">
              {{ $t('actions.save') }}
            </v-btn>
            <v-btn size="small" variant="tonal" color="warning" prepend-icon="mdi-reload" :loading="reloading" @click="reloadCore">
              {{ $t('configEditor.reload') }}
            </v-btn>
          </v-card-actions>

          <v-divider />

          <v-card-text class="editor-wrap flex-grow-1">
            <v-textarea
              v-model="content"
              class="config-editor"
              variant="outlined"
              hide-details
              spellcheck="false"
              no-resize
              rows="34"
              @keydown.tab.prevent="insertTab"
            />
          </v-card-text>

          <v-divider />
          <v-card-actions class="text-caption text-medium-emphasis px-4">
            <span>{{ lineCount }} {{ $t('configEditor.lines') }}</span>
            <span class="ml-4">{{ content.length }} {{ $t('configEditor.characters') }}</span>
            <v-spacer />
            <span v-if="updatedAt">{{ $t('configEditor.updatedAt') }}: {{ formatTime(updatedAt) }}</span>
          </v-card-actions>
        </v-card>
      </v-col>

      <v-col cols="12" lg="6" class="d-flex">
        <v-card class="rounded-lg config-card d-flex flex-column" variant="outlined">
          <v-toolbar density="comfortable" color="transparent">
            <v-toolbar-title>{{ $t('configEditor.runtimeTitle') }}</v-toolbar-title>
            <v-chip :color="runtimeRunning ? 'success' : 'error'" size="small" variant="tonal">
              {{ runtimeRunning ? $t('configEditor.coreRunning') : $t('configEditor.coreStopped') }}
            </v-chip>
          </v-toolbar>

          <v-divider />

          <v-card-actions class="toolbar-actions px-3 py-2">
            <v-chip color="secondary" size="small" variant="tonal" prepend-icon="mdi-lock-outline">
              {{ $t('configEditor.readOnly') }}
            </v-chip>
            <v-spacer />
            <v-btn size="small" variant="text" prepend-icon="mdi-refresh" :loading="runtimeLoading" @click="loadRuntimeConfig">
              {{ $t('configEditor.refreshRuntime') }}
            </v-btn>
            <v-btn size="small" variant="text" prepend-icon="mdi-content-copy" :disabled="!runtimeExists" @click="copyRuntimeConfig">
              {{ $t('configEditor.copy') }}
            </v-btn>
          </v-card-actions>

          <v-divider />

          <v-alert v-if="!runtimeExists && !runtimeLoading" density="compact" type="warning" variant="tonal" class="ma-3 mb-0">
            {{ $t('configEditor.runtimeMissing') }}
          </v-alert>

          <v-card-text class="editor-wrap flex-grow-1">
            <v-textarea
              :model-value="runtimeContent"
              class="config-editor runtime-editor"
              variant="outlined"
              hide-details
              spellcheck="false"
              no-resize
              readonly
              rows="34"
              :placeholder="$t('configEditor.runtimePlaceholder')"
            />
          </v-card-text>

          <v-divider />
          <v-card-actions class="text-caption text-medium-emphasis px-4">
            <span>{{ runtimeLineCount }} {{ $t('configEditor.lines') }}</span>
            <span class="ml-4">{{ runtimeContent.length }} {{ $t('configEditor.characters') }}</span>
            <v-spacer />
            <span v-if="runtimeUpdatedAt">{{ $t('configEditor.generatedAt') }}: {{ formatTime(runtimeUpdatedAt) }}</span>
          </v-card-actions>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>

<script lang="ts" setup>
import HttpUtils from '@/plugins/httputil'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

const content = ref('')
const baseline = ref('')
const updatedAt = ref(0)
const loading = ref(false)
const checking = ref(false)
const saving = ref(false)
const reloading = ref(false)
const pendingReload = ref(false)

const runtimeContent = ref('')
const runtimeUpdatedAt = ref(0)
const runtimeExists = ref(false)
const runtimeRunning = ref(false)
const runtimeLoading = ref(false)

const dirty = computed(() => content.value !== baseline.value)
const lineCount = computed(() => content.value.length === 0 ? 0 : content.value.split(/\r?\n/).length)
const runtimeLineCount = computed(() => runtimeContent.value.length === 0 ? 0 : runtimeContent.value.split(/\r?\n/).length)

const loadConfig = async () => {
  if (dirty.value && !window.confirm('Discard unsaved changes and reload config.json?')) return
  loading.value = true
  const msg = await HttpUtils.get('api/configText')
  loading.value = false
  if (msg.success) {
    content.value = msg.obj.content ?? ''
    baseline.value = content.value
    updatedAt.value = msg.obj.updatedAt ?? 0
    pendingReload.value = Boolean(msg.obj.pendingReload)
  }
}

const loadRuntimeConfig = async () => {
  runtimeLoading.value = true
  const msg = await HttpUtils.get('api/runtimeConfigText')
  runtimeLoading.value = false
  if (msg.success) {
    runtimeContent.value = msg.obj.content ?? ''
    runtimeUpdatedAt.value = msg.obj.updatedAt ?? 0
    runtimeExists.value = Boolean(msg.obj.exists)
    runtimeRunning.value = Boolean(msg.obj.running)
  }
}

const checkConfig = async () => {
  checking.value = true
  await HttpUtils.postJSON('api/checkConfigText', { content: content.value })
  checking.value = false
}

const saveConfig = async () => {
  saving.value = true
  const msg = await HttpUtils.postJSON('api/saveConfigText', { content: content.value })
  saving.value = false
  if (msg.success) {
    baseline.value = content.value
    updatedAt.value = Math.floor(Date.now() / 1000)
    pendingReload.value = true
  }
}

const reloadCore = async () => {
  if (dirty.value && !window.confirm('The editor contains unsaved changes. Reload sing-box anyway?')) return
  reloading.value = true
  const msg = await HttpUtils.post('api/restartSb', {})
  reloading.value = false
  if (msg.success) {
    pendingReload.value = false
    await Promise.all([loadConfig(), loadRuntimeConfig()])
  }
}

const copyConfig = async () => {
  await navigator.clipboard.writeText(content.value)
}

const copyRuntimeConfig = async () => {
  await navigator.clipboard.writeText(runtimeContent.value)
}

const pasteConfig = async () => {
  const text = await navigator.clipboard.readText()
  content.value = text
}

const insertTab = (event: KeyboardEvent) => {
  const target = event.target as HTMLTextAreaElement
  if (!target) return
  const start = target.selectionStart
  const end = target.selectionEnd
  content.value = content.value.slice(0, start) + '  ' + content.value.slice(end)
  requestAnimationFrame(() => {
    target.selectionStart = target.selectionEnd = start + 2
  })
}

const formatTime = (timestamp: number) => new Date(timestamp * 1000).toLocaleString()

const beforeUnload = (event: BeforeUnloadEvent) => {
  if (!dirty.value) return
  event.preventDefault()
  event.returnValue = ''
}

onMounted(() => {
  Promise.all([loadConfig(), loadRuntimeConfig()])
  window.addEventListener('beforeunload', beforeUnload)
})

onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))
</script>

<style scoped>
.config-page {
  max-width: 1920px;
}
.config-card {
  width: 100%;
  min-width: 0;
}
.toolbar-actions {
  flex-wrap: wrap;
  gap: 4px;
}
.editor-wrap {
  padding: 12px;
}
:deep(.config-editor textarea) {
  min-height: 68vh;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  font-size: 13px;
  line-height: 1.55;
  direction: ltr;
  tab-size: 2;
  white-space: pre;
  overflow: auto !important;
}
:deep(.runtime-editor textarea) {
  background: rgba(var(--v-theme-on-surface), 0.025);
}
</style>
