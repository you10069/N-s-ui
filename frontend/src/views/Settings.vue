<template>
  <v-card :loading="loading" class="rounded-lg">
    <v-tabs v-model="tab" align-tabs="center" bg-color="secondary" show-arrows>
      <v-tab value="interface" prepend-icon="mdi-monitor-cog">
        {{ $t('setting.interface') }}
      </v-tab>
      <v-tab value="admin" prepend-icon="mdi-account-cog">
        {{ $t('pages.admins') }}
      </v-tab>
    </v-tabs>

    <v-divider />

    <v-window v-model="tab">
      <v-window-item value="interface">
        <v-card-text>
          <v-row align="center" justify="center" class="mb-3">
            <v-col cols="auto">
              <v-btn color="primary" @click="saveChanges" :loading="loading" :disabled="!stateChange">
                {{ $t('actions.save') }}
              </v-btn>
            </v-col>
            <v-col cols="auto">
              <v-btn variant="outlined" color="warning" @click="restartApp" :loading="loading" :disabled="stateChange">
                {{ $t('actions.restartApp') }}
              </v-btn>
            </v-col>
          </v-row>

          <v-row>
            <v-col cols="12">
              <div class="text-subtitle-1 font-weight-medium mb-2">
                {{ $t('setting.interface') }}
              </div>
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-text-field v-model="settings.webListen" :label="$t('setting.addr')" hide-details />
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-text-field v-model.number="webPort" min="1" type="number" :label="$t('setting.port')" hide-details />
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-text-field v-model="settings.webPath" :label="$t('setting.webPath')" hide-details />
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-text-field v-model="settings.webDomain" :label="$t('setting.domain')" hide-details />
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-text-field v-model="settings.webKeyFile" :label="$t('setting.sslKey')" hide-details />
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-text-field v-model="settings.webCertFile" :label="$t('setting.sslCert')" hide-details />
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-text-field v-model="settings.webURI" :label="$t('setting.webUri')" hide-details />
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-text-field
                v-model.number="sessionMaxAge"
                type="number"
                min="0"
                :label="$t('setting.sessionAge')"
                :suffix="$t('date.m')"
                hide-details
              />
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-text-field
                v-model.number="trafficAge"
                type="number"
                min="0"
                :label="$t('setting.trafficAge')"
                :suffix="$t('date.d')"
                hide-details
              />
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-text-field v-model="settings.timeLocation" :label="$t('setting.timeLoc')" hide-details />
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-select
                v-model="$i18n.locale"
                :items="languages"
                :label="$t('setting.language')"
                prepend-inner-icon="mdi-translate"
                hide-details
                @update:model-value="changeLocale"
              />
            </v-col>
          </v-row>
        </v-card-text>
      </v-window-item>

      <v-window-item value="admin">
        <v-card-text>
          <AdminsView />
        </v-card-text>
      </v-window-item>
    </v-window>
  </v-card>
</template>

<script lang="ts" setup>
import { useLocale } from 'vuetify'
import { useRoute } from 'vue-router'
import { languages } from '@/locales'
import { Ref, computed, inject, onMounted, ref, watch } from 'vue'
import HttpUtils from '@/plugins/httputil'
import { FindDiff } from '@/plugins/utils'
import AdminsView from '@/views/Admins.vue'

const locale = useLocale()
const route = useRoute()
const tab = ref(route.query.tab === 'admin' ? 'admin' : 'interface')
const loading: Ref = inject('loading') ?? ref(false)
const oldSettings = ref<Record<string, string>>({})

const settings = ref({
  webListen: '',
  webDomain: '',
  webPort: '2095',
  webCertFile: '',
  webKeyFile: '',
  webPath: '/app/',
  webURI: '',
  sessionMaxAge: '0',
  trafficAge: '30',
  timeLocation: 'Asia/Tehran',
})

watch(
  () => route.query.tab,
  value => {
    tab.value = value === 'admin' ? 'admin' : 'interface'
  },
)

const changeLocale = (value: any) => {
  locale.current.value = value === 'zhHans' ? 'zhHans' : 'en'
  localStorage.setItem('locale', locale.current.value)
}

const loadData = async () => {
  loading.value = true
  const msg = await HttpUtils.get('api/setting')
  loading.value = false
  if (msg.success) {
    settings.value = { ...settings.value, ...msg.obj }
    oldSettings.value = JSON.parse(JSON.stringify(settings.value))
  }
}

onMounted(loadData)

const saveChanges = async () => {
  loading.value = true
  const diff = {
    settings: JSON.stringify(FindDiff.Settings(settings.value, oldSettings.value)),
  }
  const msg = await HttpUtils.post('api/save', diff)
  if (msg.success) {
    await loadData()
  }
  loading.value = false
}

const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))

const restartApp = async () => {
  loading.value = true
  const msg = await HttpUtils.post('api/restartApp', {})
  if (msg.success) {
    let url = settings.value.webURI
    if (url === '') {
      const isTLS = settings.value.webCertFile !== '' || settings.value.webKeyFile !== ''
      url = buildURL(settings.value.webDomain, settings.value.webPort, isTLS, settings.value.webPath)
    }
    await sleep(3000)
    window.location.replace(url)
  }
  loading.value = false
}

const buildURL = (host: string, port: string, isTLS: boolean, path: string) => {
  if (!host) host = window.location.hostname
  if (!port) port = window.location.port

  const protocol = isTLS ? 'https:' : 'http:'
  const portPart = port === '' || (isTLS && port === '443') || (!isTLS && port === '80') ? '' : `:${port}`
  return `${protocol}//${host}${portPart}${path}settings`
}

const webPort = computed({
  get: () => (settings.value.webPort.length > 0 ? parseInt(settings.value.webPort) : 2095),
  set: (value: number) => {
    settings.value.webPort = value > 0 ? value.toString() : '2095'
  },
})

const sessionMaxAge = computed({
  get: () => (settings.value.sessionMaxAge.length > 0 ? parseInt(settings.value.sessionMaxAge) : 0),
  set: (value: number) => {
    settings.value.sessionMaxAge = value > 0 ? value.toString() : '0'
  },
})

const trafficAge = computed({
  get: () => (settings.value.trafficAge.length > 0 ? parseInt(settings.value.trafficAge) : 0),
  set: (value: number) => {
    settings.value.trafficAge = value > 0 ? value.toString() : '0'
  },
})

const stateChange = computed(() => !FindDiff.deepCompare(settings.value, oldSettings.value))
</script>
