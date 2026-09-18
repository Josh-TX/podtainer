<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { containersApi, containerExecApi } from '../api'
import CodeEditor from '../components/CodeEditor.vue'
import ShellTerminal from '../components/ShellTerminal.vue'
import NavButtons from '../components/NavButtons.vue'

const props = defineProps({ id: { type: String, required: true } })
const router = useRouter()

const container = ref(null)
const stats = ref(null)
const logs = ref('')
const error = ref('')
const loading = ref(true)
let firstLoad = true

const tab = ref('logs')
const execSessionId = ref(null)
const execError = ref('')

const tabOptions = computed(() => [
  { label: 'Logs', value: 'logs' },
  {
    label: 'Console',
    value: 'console',
    disabled: container.value?.state !== 'running',
    title: container.value?.state !== 'running' ? 'Container must be running' : '',
  },
])

async function selectTab(value) {
  tab.value = value
  if (value === 'console') await openConsole()
}

async function openConsole() {
  if (execSessionId.value) return
  execError.value = ''
  try {
    const s = await containerExecApi.create(props.id)
    execSessionId.value = s.id
  } catch (e) {
    execError.value = e.message
  }
}

onBeforeUnmount(() => {
  if (execSessionId.value) {
    containerExecApi.close(execSessionId.value).catch(() => {})
  }
})

async function load() {
  error.value = ''
  if (firstLoad) loading.value = true
  try {
    const list = await containersApi.list()
    container.value = list.find((c) => c.id === props.id || c.names?.includes(props.id)) || null
  } catch (e) {
    error.value = e.message
  }
  try {
    stats.value = await containersApi.stats(props.id)
  } catch (e) {
    stats.value = { error: e.message }
  }
  try {
    const data = await containersApi.logs(props.id)
    logs.value = data.logs
  } catch (e) {
    logs.value = 'Error: ' + e.message
  } finally {
    loading.value = false
    firstLoad = false
  }
}

async function action(fn) {
  error.value = ''
  try {
    await fn(props.id)
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function remove() {
  if (!confirm(`Remove container "${container.value?.names?.[0] || props.id}"?`)) return
  error.value = ''
  try {
    await containersApi.remove(props.id)
    router.push('/containers')
  } catch (e) {
    error.value = e.message
  }
}

onMounted(load)
</script>

<template>
  <div class="container-detail-view">
    <div class="page-header">
      <h1>{{ container?.names?.[0] || id.slice(0, 12) }}</h1>
      <RouterLink to="/containers" role="button" class="secondary">Back</RouterLink>
    </div>

    <div v-if="error" class="error-banner">{{ error }}</div>
    <p v-else-if="loading" aria-busy="true">Loading…</p>
    <p v-else-if="!container" class="muted">Container not found.</p>

    <template v-else>
      <div class="toolbar">
        <span :class="['badge', container.state]">{{ container.state }}</span>
        <span class="muted">{{ container.image }}</span>
        <span class="muted">{{ container.status }}</span>
        <span v-if="container.systemdUnit" class="muted">managed by {{ container.systemdUnit }}</span>
      </div>

      <div class="toolbar">
        <button class="secondary" @click="action(containersApi.start)">Start</button>
        <button class="secondary" @click="action(containersApi.stop)">Stop</button>
        <button class="secondary" @click="action(containersApi.restart)">Restart</button>
        <button class="danger" @click="remove">Remove</button>
      </div>

      <p class="muted">CPU: {{ stats?.CPU ?? '—' }} &nbsp; Mem: {{ stats?.MemUsage ?? '—' }}</p>

      <div class="toolbar">
        <NavButtons :options="tabOptions" :model-value="tab" @update:model-value="selectTab" />
      </div>

      <CodeEditor v-if="tab === 'logs'" :model-value="logs" readonly autoscroll />
      <template v-else>
        <div v-if="execError" class="error-banner">{{ execError }}</div>
        <ShellTerminal v-if="execSessionId" :key="execSessionId" :ws-url="containerExecApi.wsUrl(execSessionId)" />
      </template>
    </template>
  </div>
</template>

<style scoped>
.container-detail-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}
</style>
