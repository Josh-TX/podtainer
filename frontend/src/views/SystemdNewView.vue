<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { systemdApi } from '../api'
import CodeEditor from '../components/CodeEditor.vue'

const router = useRouter()

const name = ref('')
const filename = computed(() => name.value.trim() + '.service')

const description = ref('')
const execStart = ref('')
const workingDirectory = ref('')
const environment = ref('')
const restart = ref('on-failure')
const backoffRetry = ref(true)
const enableOnBoot = ref(true)

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
  get: () => rawOverride.value ?? generatedFromEasy.value,
  set: (v) => { rawOverride.value = v },
})
const desynced = computed(() => rawOverride.value !== null && rawOverride.value !== generatedFromEasy.value)

function syncFromEasy() {
  rawOverride.value = null
}

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
  <div class="page-header">
    <h1 style="margin-bottom: 0.5rem">New Systemd Unit</h1>
    <div class="toolbar" style="margin-bottom: 0">
      <button :disabled="busy || !name.trim()" @click="save">Save</button>
    </div>
  </div>

  <div v-if="error" class="error-banner">{{ error }}</div>

  <label>
    Unit name
    <input v-model="name" placeholder="e.g. my-unit" />
    <small class="muted">Saved as {{ filename }}</small>
  </label>

  <div class="toolbar">
    <button :class="{ secondary: mode !== 'easy' }" @click="mode = 'easy'">Easy Editor</button>
    <button :class="{ secondary: mode !== 'raw' }" @click="mode = 'raw'">Raw Editor</button>
    <span v-if="desynced" class="badge notdeployed">Desynced</span>
    <button v-if="desynced" class="secondary" @click="syncFromEasy">Sync from Easy</button>
  </div>

  <div v-if="mode === 'easy'" class="stack-columns">
    <div class="col">
      <label>
        Description <small class="muted">(optional, will default to unit name)</small>
        <input v-model="description" placeholder="What this unit does" />
      </label>
      <label>
        ExecStart
        <input v-model="execStart" placeholder="e.g. /usr/bin/my-command --flag" />
      </label>
      <label>
        Working directory
        <input v-model="workingDirectory" placeholder="e.g. /opt/my-app" />
      </label>
      <label>
        Environment <small class="muted">(can be space or newline delimitted)</small>
        <textarea v-model="environment" rows="3" placeholder="e.g. FOO=bar"></textarea>
      </label>
      <label>
        Restart
        <select v-model="restart">
          <option value="no">no</option>
          <option value="on-failure">on-failure</option>
          <option value="always">always</option>
        </select>
      </label>
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
</template>
