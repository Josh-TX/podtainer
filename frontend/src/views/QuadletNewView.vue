<script setup>
import { ref, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { systemdApi, quadletsApi } from '../api'
import CodeEditor from '../components/CodeEditor.vue'
import NavButtons from '../components/NavButtons.vue'
import TypeaheadInput from '../components/TypeaheadInput.vue'

const router = useRouter()

const UNIT_TYPES = ['container', 'volume', 'network', 'kube', 'image', 'pod', 'build']

const name = ref('')
const unitType = ref('container')
const filename = computed(() => name.value.trim() + '.' + unitType.value)

const description = ref('')
const image = ref('')
const network = ref('')
const ports = ref([{ host: '', container: '' }])
const volumes = ref([{ host: '', container: '', mode: 'rw' }])
const restart = ref('on-failure')
const backoffRetry = ref(true)
const enableOnBoot = ref(true)

function addPort() {
  ports.value.push({ host: '', container: '' })
}
function addVolume() {
  volumes.value.push({ host: '', container: '', mode: 'rw' })
}
function volumeHostSuggestions(query) {
  return systemdApi.fsSuggestions(query, true)
}

const generatedFromEasy = computed(() => {
  const parts = []
  if (description.value.trim()) parts.push(`[Unit]\nDescription=${description.value.trim()}`)

  const container = [`Image=${image.value.trim()}`, `ContainerName=${name.value.trim()}`]
  if (network.value.trim()) container.push(`Network=${network.value.trim()}`)
  for (const p of ports.value) {
    if (p.host.trim() && p.container.trim()) container.push(`PublishPort=${p.host.trim()}:${p.container.trim()}`)
  }
  for (const v of volumes.value) {
    if (v.host.trim() && v.container.trim()) {
      const suffix = v.mode === 'ro' ? ':ro' : ''
      container.push(`Volume=${v.host.trim()}:${v.container.trim()}${suffix}`)
    }
  }
  parts.push(`[Container]\n${container.join('\n')}`)

  const service = [`Restart=${restart.value}`]
  if (backoffRetry.value) {
    service.push('RestartSec=5s', 'RestartSteps=8', 'RestartMaxDelaySec=600s')
  }
  parts.push(`[Service]\n${service.join('\n')}`)

  if (enableOnBoot.value) parts.push('[Install]\nWantedBy=default.target')
  return parts.join('\n\n') + '\n'
})

const mode = ref('easy')
const rawOverride = ref(null)
const rawContent = computed({
  get: () => rawOverride.value ?? (unitType.value === 'container' ? generatedFromEasy.value : ''),
  set: (v) => { rawOverride.value = v },
})
const desynced = computed(
  () => unitType.value === 'container' && rawOverride.value !== null && rawOverride.value !== generatedFromEasy.value
)

function syncFromEasy() {
  rawOverride.value = null
}

watch(unitType, () => {
  if (unitType.value !== 'container') mode.value = 'raw'
  rawOverride.value = null
})

const modeOptions = computed(() => [
  { label: 'Easy Editor', value: 'easy', disabled: unitType.value !== 'container', title: unitType.value !== 'container' ? 'Easy editor only supports container units' : '' },
  { label: 'Raw Editor', value: 'raw' },
])

const error = ref('')
const busy = ref(false)

async function save() {
  error.value = ''
  if (!name.value.trim()) {
    error.value = 'Unit name is required.'
    return
  }
  busy.value = true
  try {
    const content = mode.value === 'easy' ? generatedFromEasy.value : rawContent.value
    await quadletsApi.create(filename.value, content)
    router.push(`/quadlets/${encodeURIComponent(filename.value)}`)
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="page-container">
  <div class="page-header">
    <h1 style="margin-bottom: 0.5rem">New Quadlet Unit</h1>
    <div class="toolbar" style="margin-bottom: 0">
      <button :disabled="busy || !name.trim()" @click="save">Save</button>
    </div>
  </div>

  <div v-if="error" class="error-banner">{{ error }}</div>

  <div style="margin-bottom: 0.5rem">
    <label>Unit name</label>
    <div role="group" style="margin-bottom: 0.25rem">
      <input v-model="name" placeholder="e.g. my-app" />
      <select v-model="unitType" style="max-width: 9rem">
        <option v-for="t in UNIT_TYPES" :key="t" :value="t">.{{ t }}</option>
      </select>
    </div>
    <small class="muted">~/.config/containers/systemd/{{ filename }}</small>
  </div>

  <div class="toolbar">
    <NavButtons :options="modeOptions" :model-value="mode" @update:model-value="mode = $event" />
    <span v-if="desynced" class="badge notdeployed">Desynced</span>
    <button v-if="desynced" class="secondary" @click="syncFromEasy">Sync from Easy</button>
  </div>

  <div v-if="mode === 'easy'" class="stack-columns">
    <div class="col">
      <div>
        <label>Description <small class="muted">(optional, will default to unit name)</small></label>
        <input v-model="description" placeholder="What this unit does" />
      </div>
      <div>
        <label>Image</label>
        <input v-model="image" placeholder="e.g. docker.io/library/nginx:latest" />
      </div>
      <div>
        <label>Network <small class="muted">(optional)</small></label>
        <input v-model="network" placeholder="e.g. my-app.network" />
      </div>
      <div>
        <label>Port Bindings</label>
        <div class="port-grid">
          <small class="muted">Host Port</small>
          <small class="muted">Container Port</small>
          <template v-for="(p, i) in ports" :key="i">
            <input v-model="p.host" placeholder="e.g. 8080" />
            <input v-model="p.container" placeholder="e.g. 80" />
          </template>
        </div>
        <div class="text-right">
          <button type="button" class="link-button" @click="addPort">+ Add another port binding</button>
        </div>
      </div>
      <div>
        <label>Volumes</label>
        <div class="volume-grid">
          <small class="muted">Host Volume</small>
          <small class="muted">Container Volume Path</small>
          <small class="muted">Mode</small>
          <template v-for="(v, i) in volumes" :key="i">
            <TypeaheadInput v-model="v.host" :fetch-suggestions="volumeHostSuggestions" placeholder="e.g. /opt/my-app/data or myvolume" />
            <input v-model="v.container" placeholder="e.g. /data" />
            <select v-model="v.mode">
              <option value="rw">rw</option>
              <option value="ro">ro</option>
            </select>
          </template>
        </div>
        <div class="text-right">
          <button type="button" class="link-button" @click="addVolume">+ Add another volume</button>
        </div>
      </div>
      <div>
        <label>Restart</label>
        <select v-model="restart">
          <option value="no">no</option>
          <option value="on-failure">on-failure</option>
          <option value="always">always</option>
        </select>
      </div>
      <label>
        <input type="checkbox" v-model="backoffRetry" />
        Backoff retry
      </label>
      <label>
        <input type="checkbox" v-model="enableOnBoot" />
        Enable on boot
      </label>
    </div>
  </div>

  <CodeEditor v-else v-model="rawContent" language="unit" max-height="26rem" />
  </div>
</template>

<style scoped>
.page-container {
  max-width: 1200px;
  margin: 0 auto;
}
.stack-columns label {
  margin-bottom: 0;
}
.stack-columns label + input,
.stack-columns label + select,
.stack-columns label + textarea {
  margin-top: calc(var(--pico-spacing) * 0.25);
}
.port-grid small,
.volume-grid small {
  margin-bottom: -4px;
}
.port-grid,
.volume-grid {
  display: grid;
  column-gap: 0.5rem;
  row-gap: 0.25rem;
  align-items: center;
  margin-bottom: 0.5rem;
}
.port-grid {
  grid-template-columns: 1fr 1fr;
}
.volume-grid {
  grid-template-columns: 1.2fr 1fr 6rem;
}
.port-grid input,
.volume-grid input,
.volume-grid select {
  min-width: 0;
  margin-bottom: 0;
}
.volume-grid :deep(.typeahead) {
  min-width: 0;
  margin-bottom: 0;
}
.text-right {
  text-align: right;
  margin-top: -0.5rem;
}
.link-button {
  width: auto;
  padding: 0;
  margin: 0;
  border: none;
  background: none;
  color: var(--pico-primary);
  cursor: pointer;
}
.link-button:hover {
  text-decoration: underline;
  background: none;
}
</style>
