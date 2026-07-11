<template>
  <v-container fluid class="users-page">
    <v-card class="rounded-lg" variant="outlined">
      <v-toolbar density="comfortable" color="transparent">
        <v-toolbar-title>{{ $t('pages.clients') }}</v-toolbar-title>
        <v-chip size="small" variant="tonal" color="primary">
          {{ clients.length }}
        </v-chip>
        <v-spacer />
        <v-text-field
          v-model="search"
          density="compact"
          variant="outlined"
          prepend-inner-icon="mdi-magnify"
          :label="$t('filter')"
          hide-details
          clearable
          max-width="300"
        />
        <v-btn class="ml-2" variant="text" prepend-icon="mdi-refresh" :loading="loading" @click="loadClients">
          {{ $t('configEditor.read') }}
        </v-btn>
      </v-toolbar>

      <v-divider />

      <v-alert density="compact" type="info" variant="tonal" class="ma-3 mb-0">
        {{ $t('client.sourceTip') }}
      </v-alert>
      <v-alert density="compact" type="warning" variant="tonal" class="mx-3 mt-2 mb-0">
        {{ $t('client.limitTip') }}
      </v-alert>

      <v-data-table
        :headers="headers"
        :items="clients"
        :search="search"
        :loading="loading"
        item-value="id"
        density="comfortable"
      >
        <template #item.name="{ item }">
          <div class="font-weight-medium">{{ item.name }}</div>
          <div class="text-caption text-medium-emphasis">{{ inboundText(item) }}</div>
        </template>

        <template #item.status="{ item }">
          <v-chip :color="statusColor(item)" size="small" variant="tonal">
            <v-icon start size="small" :icon="statusIcon(item)" />
            {{ statusText(item) }}
          </v-chip>
          <div v-if="item.blockedAt" class="text-caption text-medium-emphasis mt-1">
            {{ new Date(item.blockedAt * 1000).toLocaleString() }}
          </div>
        </template>

        <template #item.online="{ item }">
          <v-chip :color="isOnline(item.name) ? 'success' : 'default'" size="small" variant="tonal">
            {{ isOnline(item.name) ? $t('online') : $t('client.offline') }}
          </v-chip>
        </template>

        <template #item.up="{ item }">
          <span class="text-orange"><v-icon size="small" icon="mdi-upload" /> {{ size(item.up) }}</span>
        </template>

        <template #item.down="{ item }">
          <span class="text-success"><v-icon size="small" icon="mdi-download" /> {{ size(item.down) }}</span>
        </template>

        <template #item.total="{ item }">
          {{ size(item.up + item.down) }}
        </template>

        <template #item.adjusted="{ item }">
          <strong>{{ size(adjusted(item)) }}</strong>
          <div class="text-caption text-medium-emphasis">× {{ normalizedMultiplier(item.multiplier) }}</div>
        </template>

        <template #item.quota="{ item }">
          <template v-if="item.volume > 0">
            <div class="d-flex justify-space-between text-caption mb-1">
              <span>{{ size(adjusted(item)) }}</span>
              <span>{{ size(item.volume) }}</span>
            </div>
            <v-progress-linear
              :model-value="quotaPercent(item)"
              :color="quotaColor(item)"
              height="7"
              rounded
            />
            <div class="text-caption text-medium-emphasis mt-1">
              {{ $t('client.remaining') }} {{ size(Math.max(0, item.volume - adjusted(item))) }}
            </div>
          </template>
          <span v-else class="text-medium-emphasis">{{ $t('unlimited') }}</span>
        </template>

        <template #item.expiry="{ item }">
          <v-chip :color="expiryColor(item.expiry)" size="small" variant="tonal">
            {{ expiryText(item.expiry) }}
          </v-chip>
        </template>

        <template #item.resetDay="{ item }">
          {{ item.resetDay > 0 ? $t('client.monthlyDay', { day: item.resetDay }) : $t('none') }}
        </template>

        <template #item.actions="{ item }">
          <v-btn icon="mdi-pencil" size="small" variant="text" @click="openEditor(item)">
            <v-tooltip activator="parent" location="top">{{ $t('actions.edit') }}</v-tooltip>
          </v-btn>
          <v-btn icon="mdi-restore" size="small" variant="text" color="warning" @click="resetTraffic(item)">
            <v-tooltip activator="parent" location="top">{{ $t('client.resetTraffic') }}</v-tooltip>
          </v-btn>
        </template>
      </v-data-table>
    </v-card>

    <v-dialog v-model="editor.visible" max-width="720">
      <v-card class="rounded-lg">
        <v-card-title class="d-flex align-center">
          {{ editor.data.name }}
          <v-spacer />
          <v-chip :color="statusColor(editor.data)" size="small" variant="tonal">
            {{ statusText(editor.data) }}
          </v-chip>
        </v-card-title>
        <v-divider />
        <v-card-text>
          <v-alert density="compact" type="info" variant="tonal" class="mb-4">
            {{ $t('client.runtimeTip') }}
          </v-alert>
          <v-row>
            <v-col cols="12">
              <v-switch
                v-model="editor.data.enable"
                color="success"
                :label="$t('client.manualEnable')"
                hide-details
                inset
              />
            </v-col>
            <v-col cols="12" sm="6">
              <v-text-field
                v-model.number="editor.volumeGB"
                type="number"
                min="0"
                step="0.1"
                :label="$t('client.trafficLimitGB')"
                suffix="GB"
                :hint="$t('client.zeroUnlimited')"
                persistent-hint
              />
            </v-col>
            <v-col cols="12" sm="6">
              <v-text-field
                v-model.number="editor.data.multiplier"
                type="number"
                min="0.01"
                max="1000"
                step="0.01"
                :label="$t('client.multiplier')"
                prefix="×"
                :hint="$t('client.multiplierHint')"
                persistent-hint
              />
            </v-col>
            <v-col cols="12" sm="6">
              <v-select
                v-model.number="editor.data.resetDay"
                :items="resetDays"
                item-title="title"
                item-value="value"
                :label="$t('client.resetDay')"
                hide-details
              />
            </v-col>
            <v-col cols="12" sm="6">
              <v-text-field
                v-model="editor.expiryLocal"
                type="datetime-local"
                :label="$t('date.expiry')"
                clearable
                hide-details
              />
            </v-col>
            <v-col cols="12">
              <v-textarea v-model="editor.data.desc" :label="$t('client.desc')" rows="2" hide-details />
            </v-col>
          </v-row>
          <v-row class="mt-3">
            <v-col cols="6" sm="3">
              <div class="text-caption text-medium-emphasis">{{ $t('stats.upload') }}</div>
              <div>{{ size(editor.data.up) }}</div>
            </v-col>
            <v-col cols="6" sm="3">
              <div class="text-caption text-medium-emphasis">{{ $t('stats.download') }}</div>
              <div>{{ size(editor.data.down) }}</div>
            </v-col>
            <v-col cols="6" sm="3">
              <div class="text-caption text-medium-emphasis">{{ $t('client.adjustedTraffic') }}</div>
              <div>{{ size(adjusted(editor.data)) }}</div>
            </v-col>
            <v-col cols="6" sm="3">
              <div class="text-caption text-medium-emphasis">{{ $t('client.remaining') }}</div>
              <div>{{ editor.data.volume > 0 ? size(Math.max(0, editor.data.volume - adjusted(editor.data))) : $t('unlimited') }}</div>
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider />
        <v-card-actions>
          <v-btn color="warning" variant="text" prepend-icon="mdi-restore" @click="resetTraffic(editor.data)">
            {{ $t('client.resetTraffic') }}
          </v-btn>
          <v-spacer />
          <v-btn variant="text" @click="editor.visible = false">{{ $t('actions.close') }}</v-btn>
          <v-btn color="primary" variant="tonal" :loading="editor.saving" @click="saveMetadata">
            {{ $t('actions.save') }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-container>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import HttpUtils from '@/plugins/httputil'
import { HumanReadable } from '@/plugins/utils'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { i18n } from '@/locales'

const GIB = 1024 * 1024 * 1024

type ManagedClient = {
  id: number
  enable: boolean
  name: string
  inbounds: string[] | string
  up: number
  down: number
  volume: number
  multiplier: number
  adjusted: number
  remaining: number
  expiry: number
  resetDay: number
  lastResetAt: number
  desc: string
  blocked: boolean
  blockedReason: string
  blockedAt: number
  status: string
}

const clients = ref<ManagedClient[]>([])
const loading = ref(false)
const search = ref('')
let timer: ReturnType<typeof setInterval> | undefined

const headers = computed(() => [
  { title: i18n.global.t('client.name'), key: 'name' },
  { title: i18n.global.t('client.status'), key: 'status', sortable: false },
  { title: i18n.global.t('online'), key: 'online', sortable: false },
  { title: i18n.global.t('stats.upload'), key: 'up' },
  { title: i18n.global.t('stats.download'), key: 'down' },
  { title: i18n.global.t('client.totalTraffic'), key: 'total', value: (item: ManagedClient) => item.up + item.down },
  { title: i18n.global.t('client.adjustedTraffic'), key: 'adjusted' },
  { title: i18n.global.t('client.trafficQuota'), key: 'quota', sortable: false, width: 190 },
  { title: i18n.global.t('date.expiry'), key: 'expiry' },
  { title: i18n.global.t('client.resetDay'), key: 'resetDay' },
  { title: i18n.global.t('actions.action'), key: 'actions', sortable: false },
])

const resetDays = computed(() => [
  { title: i18n.global.t('none'), value: 0 },
  ...Array.from({ length: 31 }, (_, index) => ({
    title: i18n.global.t('client.monthlyDay', { day: index + 1 }),
    value: index + 1,
  })),
])

const editor = ref({
  visible: false,
  saving: false,
  expiryLocal: '',
  volumeGB: 0,
  data: {} as ManagedClient,
})

const loadClients = async () => {
  loading.value = true
  const msg = await HttpUtils.get('api/clients')
  loading.value = false
  if (msg.success) clients.value = msg.obj ?? []
}

const openEditor = (item: ManagedClient) => {
  editor.value.data = JSON.parse(JSON.stringify(item))
  editor.value.expiryLocal = toLocalInput(item.expiry)
  editor.value.volumeGB = item.volume > 0 ? Number((item.volume / GIB).toFixed(3)) : 0
  editor.value.visible = true
}

const saveMetadata = async () => {
  editor.value.saving = true
  const expiry = editor.value.expiryLocal ? Math.floor(new Date(editor.value.expiryLocal).getTime() / 1000) : 0
  const volume = Math.max(0, Math.round((Number(editor.value.volumeGB) || 0) * GIB))
  const msg = await HttpUtils.postJSON('api/updateClient', {
    id: editor.value.data.id,
    enable: Boolean(editor.value.data.enable),
    volume,
    multiplier: Number(editor.value.data.multiplier) || 1,
    expiry,
    resetDay: Number(editor.value.data.resetDay) || 0,
    desc: editor.value.data.desc ?? '',
  })
  editor.value.saving = false
  if (msg.success) {
    editor.value.visible = false
    await loadClients()
  }
}

const resetTraffic = async (item: ManagedClient) => {
  if (!window.confirm(i18n.global.t('client.resetConfirm'))) return
  const msg = await HttpUtils.postJSON('api/resetClientTraffic', { id: item.id })
  if (msg.success) {
    await loadClients()
    if (editor.value.visible && editor.value.data.id === item.id) {
      const refreshed = clients.value.find(client => client.id === item.id)
      if (refreshed) openEditor(refreshed)
    }
  }
}

const size = (value: number) => HumanReadable.sizeFormat(value || 0)
const normalizedMultiplier = (value: number) => value > 0 ? value : 1
const adjusted = (item: ManagedClient) => item.adjusted ?? Math.round((item.up + item.down) * normalizedMultiplier(item.multiplier))
const isOnline = (name: string) => Data().onlines?.user?.includes(name) ?? false

const inboundText = (item: ManagedClient) => {
  if (Array.isArray(item.inbounds)) return item.inbounds.join(', ')
  try { return JSON.parse(item.inbounds || '[]').join(', ') } catch { return '' }
}

const statusText = (item: ManagedClient) => {
  const reason = item.blockedReason || item.status
  if (!item.blocked && reason !== 'manual') return i18n.global.t('client.statusActive')
  if (reason === 'manual') return i18n.global.t('client.statusManual')
  if (reason === 'expired') return i18n.global.t('client.statusExpired')
  if (reason === 'quota') return i18n.global.t('client.statusQuota')
  return i18n.global.t('client.statusBlocked')
}

const statusColor = (item: ManagedClient) => {
  if (!item.blocked) return 'success'
  if (item.blockedReason === 'manual') return 'default'
  if (item.blockedReason === 'expired') return 'warning'
  return 'error'
}

const statusIcon = (item: ManagedClient) => {
  if (!item.blocked) return 'mdi-check-circle'
  if (item.blockedReason === 'expired') return 'mdi-clock-alert-outline'
  if (item.blockedReason === 'quota') return 'mdi-gauge-full'
  return 'mdi-account-cancel'
}

const quotaPercent = (item: ManagedClient) => item.volume > 0 ? Math.min(100, (adjusted(item) / item.volume) * 100) : 0
const quotaColor = (item: ManagedClient) => {
  const percent = quotaPercent(item)
  if (percent >= 100) return 'error'
  if (percent >= 80) return 'warning'
  return 'primary'
}

const expiryText = (expiry: number) => {
  if (!expiry) return i18n.global.t('unlimited')
  if (expiry <= Math.floor(Date.now() / 1000)) return i18n.global.t('date.expired')
  return new Date(expiry * 1000).toLocaleString()
}

const expiryColor = (expiry: number) => {
  if (!expiry) return 'default'
  const remaining = expiry - Math.floor(Date.now() / 1000)
  if (remaining <= 0) return 'error'
  if (remaining < 7 * 86400) return 'warning'
  return 'success'
}

const toLocalInput = (timestamp: number) => {
  if (!timestamp) return ''
  const date = new Date(timestamp * 1000)
  const offset = date.getTimezoneOffset() * 60000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

onMounted(() => {
  loadClients()
  timer = setInterval(loadClients, 10000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.users-page {
  max-width: 1900px;
}
</style>
