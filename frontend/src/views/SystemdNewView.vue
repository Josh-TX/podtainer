<script setup>
import { ref, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { systemdApi } from '../api'
import CodeEditor from '../components/CodeEditor.vue'
import NavButtons from '../components/NavButtons.vue'
import TypeaheadInput from '../components/TypeaheadInput.vue'

const router = useRouter()

const UNIT_TYPES = ['service', 'timer', 'socket', 'path', 'mount']

const name = ref('')
const unitType = ref('service')
const filename = computed(() => name.value.trim() + '.' + unitType.value)

const description = ref('')
const execStart = ref('')
const workingDirectory = ref('')
const environment = ref('')
const restart = ref('on-failure')
const backoffRetry = ref(true)
const enableOnBoot = ref(true)

function execStartSuggestions(query) {
  return systemdApi.fsSuggestions(query, false)
}

function workingDirectorySuggestions(query) {
  return systemdApi.fsSuggestions(query, true)
}

const generatedFromEasy = computed(() => {
  const parts = []
  if (description.value.trim()) parts.push(`[Unit]\nDescription=${description.value.trim()}`)

  const service = [`ExecStart=${execStart.value.trim()}`]
  if (workingDirectory.value.trim()) service.push(`WorkingDirectory=${workingDirectory.value.trim()}`)
  for (const line of environment.value.split('\n')) {
    if (line.trim()) service.push(`Environment=${line.trim()}`)
  }
  service.push(`Restart=${restart.value}`)
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
  get: () => rawOverride.value ?? (unitType.value === 'service' ? generatedFromEasy.value : ''),
  set: (v) => { rawOverride.value = v },
})
const desynced = computed(
  () => unitType.value === 'service' && rawOverride.value !== null && rawOverride.value !== generatedFromEasy.value
)

function syncFromEasy() {
  rawOverride.value = null
}

watch(unitType, () => {
  if (unitType.value !== 'service') mode.value = 'raw'
  rawOverride.value = null
})

const modeOptions = computed(() => [
  { label: 'Easy Editor', value: 'easy', disabled: unitType.value !== 'service', title: unitType.value !== 'service' ? 'Easy editor only supports service units' : '' },
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
    await systemdApi.create(filename.value, content)
    await systemdApi.setFavorite(filename.value, true)
    router.push(`/systemd/${encodeURIComponent(filename.value)}`)
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
    <h1 style="margin-bottom: 0.5rem">New Systemd Unit</h1>
    <div class="toolbar" style="margin-bottom: 0">
      <button :disabled="busy || !name.trim()" @click="save">Save</button>
    </div>
  </div>

  <div v-if="error" class="error-banner">{{ error }}</div>

  <div style="margin-bottom: 0.5rem">
    <label>Unit name</label>
    <div role="group" style="margin-bottom: 0.25rem">
      <input v-model="name" placeholder="e.g. my-unit" />
      <select v-model="unitType" style="max-width: 9rem">
        <option v-for="t in UNIT_TYPES" :key="t" :value="t">.{{ t }}</option>
      </select>
    </div>
    <small class="muted">~/.config/systemd/user/{{ filename }}</small>
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
        <label>ExecStart</label>
        <TypeaheadInput v-model="execStart" :fetch-suggestions="execStartSuggestions" placeholder="e.g. /usr/bin/my-command --flag" />
      </div>
      <div>
        <label>Working directory</label>
        <TypeaheadInput v-model="workingDirectory" :fetch-suggestions="workingDirectorySuggestions" placeholder="e.g. /opt/my-app" />
      </div>
      <div>
        <label>Environment <small class="muted">(can be space or newline delimitted)</small></label>
        <textarea v-model="environment" rows="3" placeholder="e.g. FOO=bar"></textarea>
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
</style>
