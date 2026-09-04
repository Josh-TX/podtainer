<script setup>
import { ref, onMounted } from 'vue'
import { quadletsApi } from '../api'
import { unitBadgeClass, unitStatusLabel, statusTitle } from '../unitBadge'
import CodeEditor from '../components/CodeEditor.vue'

const files = ref([])
const error = ref('')
const loading = ref(true)

const logsDialog = ref(null)
const generatorLogs = ref('')

async function load() {
  error.value = ''
  loading.value = true
  try {
    files.value = await quadletsApi.list()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function openLogs() {
  logsDialog.value.showModal()
  try {
    generatorLogs.value = (await quadletsApi.generatorLogs()).logs
  } catch (e) {
    generatorLogs.value = 'Error: ' + e.message
  }
}

function onDialogClick(e) {
  if (e.target === logsDialog.value) logsDialog.value.close()
}

onMounted(load)
</script>

<template>
  <div class="page-header">
    <h1>Quadlets</h1>
    <button class="secondary" @click="openLogs">View Generator Logs</button>
  </div>
  <p class="muted">Every quadlet unit file in the search path, including ones not managed by Podtainer.</p>

  <div v-if="error" class="error-banner">{{ error }}</div>

  <p v-if="loading" aria-busy="true">Loading…</p>
  <template v-else>
    <table class="rows">
      <thead>
        <tr>
          <th>Filename</th>
          <th>Type</th>
          <th>Stack</th>
          <th>Status</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="f in files" :key="f.filename">
          <td><RouterLink class="row-link" :to="`/quadlets/${encodeURIComponent(f.filename)}`">{{ f.filename }}</RouterLink></td>
          <td>{{ f.type }}</td>
          <td class="muted">{{ f.stack || 'external' }}</td>
          <td><span :class="unitBadgeClass(f)" :title="statusTitle(f)">{{ unitStatusLabel(f) }}</span></td>
        </tr>
      </tbody>
    </table>
    <p v-if="!files.length" class="muted">No quadlet files found.</p>
  </template>

  <dialog ref="logsDialog" @click="onDialogClick">
    <article style="width: min(90vw, 60rem)">
      <h3>Quadlet Generator Logs</h3>
      <p class="muted" style="margin-bottom: 0.5rem">
        Logs from podman's quadlet generator, which runs on every reload and reports files it couldn't parse.
      </p>
      <CodeEditor :model-value="generatorLogs" readonly autoscroll max-height="60vh" />
    </article>
  </dialog>
</template>
