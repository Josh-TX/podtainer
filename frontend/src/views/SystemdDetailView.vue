<script setup>
import { ref, onMounted } from 'vue'
import { systemdApi, quadletsApi, containersApi } from '../api'
import { unitBadgeClass, unitStatusLabel, statusTitle, isOrphaned } from '../unitBadge'
import CodeEditor from '../components/CodeEditor.vue'

const props = defineProps({ name: { type: String, required: true } })

const unit = ref(null)
const content = ref('')
const meta = ref(null)
const container = ref(null)
const error = ref('')
const loading = ref(true)
const logs = ref('')
const showLogs = ref(false)

async function load() {
  error.value = ''
  loading.value = true
  try {
    const units = await systemdApi.list()
    unit.value = units.find((u) => u.name === props.name) || null
    if (unit.value) {
      const orphaned = isOrphaned(unit.value)
      const [data, quadlets, containers] = await Promise.all([
        orphaned ? Promise.resolve(null) : systemdApi.content(props.name),
        quadletsApi.list(),
        containersApi.list(),
      ])
      content.value = data ? data.content : ''
      const filename = unit.value.sourcePath.split('/').pop()
      meta.value = quadlets.find((f) => f.filename === filename) || null
      container.value = containers.find((c) => c.systemdUnit === props.name) || null
    }
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function viewLogs() {
  showLogs.value = !showLogs.value
  if (!showLogs.value) return
  try {
    const data = await systemdApi.logs(props.name)
    logs.value = data.logs
  } catch (e) {
    logs.value = 'Error: ' + e.message
  }
}

onMounted(load)
</script>

<template>
  <div class="page-header">
    <h1 style="margin-bottom: 0.5rem">
      {{ name }}
      <span v-if="unit" :class="unitBadgeClass(unit)" :title="statusTitle(unit)">{{ unitStatusLabel(unit) }}</span>
    </h1>
  </div>

  <div v-if="error" class="error-banner">{{ error }}</div>
  <p v-else-if="loading" aria-busy="true">Loading…</p>
  <p v-else-if="!unit" class="muted">Unit not found.</p>

  <template v-else>
    <p v-if="isOrphaned(unit)" class="muted" style="margin-bottom: 0.25rem">
      No backing file — this unit's quadlet source failed to regenerate it (e.g. a syntax error), but it's still running from before.
    </p>
    <p v-else class="muted" style="margin-bottom: 0.25rem">{{ unit.fragmentPath }}</p>

    <div class="stack-columns">
      <div class="col">
        <p v-if="isOrphaned(unit)" class="muted">No unit file to display.</p>
        <CodeEditor v-else :model-value="content" language="unit" readonly />
      </div>

      <div class="col">
        <article>
          <p v-if="meta?.stack" style="margin-bottom: 0.25rem">
            Stack: <RouterLink :to="`/stacks/${encodeURIComponent(meta.stack)}`">{{ meta.stack }}</RouterLink>
          </p>

          <p style="margin-bottom: 0.25rem">
            Quadlet file:
            <RouterLink v-if="meta" :to="`/quadlets/${encodeURIComponent(meta.filename)}`">{{ meta.filename }}</RouterLink>
            <span v-else class="muted">Not found.</span>
          </p>

          <p v-if="meta?.type === 'container'" style="margin-bottom: 0">
            Podman container:
            <template v-if="container">
              <span :class="['badge', container.state]">{{ container.state }}</span>
              <RouterLink :to="`/containers/${encodeURIComponent(container.id)}`">{{ container.names[0] }}</RouterLink>
            </template>
            <span v-else class="muted">No running container.</span>
          </p>
        </article>

        <button class="secondary" @click="viewLogs">{{ showLogs ? 'Hide Logs' : 'View Logs' }}</button>
        <CodeEditor v-if="showLogs" :model-value="logs" readonly autoscroll />
      </div>
    </div>
  </template>
</template>
