<script setup>
import { ref, onMounted } from 'vue'
import { quadletsApi } from '../api'

const files = ref([])
const error = ref('')
const expanded = ref(null)
const editContent = ref('')
const logs = ref('')
const showLogs = ref(false)

async function load() {
  error.value = ''
  try {
    files.value = await quadletsApi.list()
  } catch (e) {
    error.value = e.message
  }
}

async function toggleExpand(f) {
  if (expanded.value === f.filename) {
    expanded.value = null
    return
  }
  error.value = ''
  showLogs.value = false
  try {
    const data = await quadletsApi.get(f.filename)
    editContent.value = data.content
    expanded.value = f.filename
  } catch (e) {
    error.value = e.message
  }
}

async function save(filename) {
  error.value = ''
  try {
    await quadletsApi.write(filename, editContent.value)
  } catch (e) {
    error.value = e.message
  }
}

async function action(fn, filename) {
  error.value = ''
  try {
    await fn(filename)
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function remove(f) {
  if (!confirm(`Delete quadlet file "${f.filename}"? This stops the unit and removes the file.`)) return
  await action(quadletsApi.delete, f.filename)
  if (expanded.value === f.filename) expanded.value = null
}

async function viewLogs(filename) {
  showLogs.value = !showLogs.value
  if (!showLogs.value) return
  try {
    const data = await quadletsApi.logs(filename)
    logs.value = data.logs
  } catch (e) {
    logs.value = 'Error: ' + e.message
  }
}

onMounted(load)
</script>

<template>
  <h1>Quadlets</h1>
  <p class="muted">Every quadlet unit file in the search path, including ones not managed by Podtainer.</p>

  <div v-if="error" class="error-banner">{{ error }}</div>

  <table>
    <thead>
      <tr>
        <th>Filename</th>
        <th>Type</th>
        <th>Stack</th>
        <th>Status</th>
        <th></th>
      </tr>
    </thead>
    <tbody>
      <template v-for="f in files" :key="f.filename">
        <tr>
          <td>{{ f.filename }}</td>
          <td>{{ f.type }}</td>
          <td>
            <RouterLink v-if="f.stack" :to="`/stacks/${f.stack}`">{{ f.stack }}</RouterLink>
            <span v-else class="muted">external</span>
          </td>
          <td><span :class="['badge', f.active]">{{ f.active }}</span></td>
          <td class="toolbar" style="margin:0;">
            <button @click="action(quadletsApi.start, f.filename)">Start</button>
            <button @click="action(quadletsApi.stop, f.filename)">Stop</button>
            <button @click="action(quadletsApi.restart, f.filename)">Restart</button>
            <button @click="toggleExpand(f)">{{ expanded === f.filename ? 'Close' : 'Edit' }}</button>
            <button class="danger" @click="remove(f)">Delete</button>
          </td>
        </tr>
        <tr v-if="expanded === f.filename">
          <td colspan="5">
            <div v-if="f.stack" class="warn-banner">
              This file is owned by stack "{{ f.stack }}" — edits here will be overwritten on the stack's next redeploy.
            </div>
            <textarea v-model="editContent" rows="12"></textarea>
            <div class="toolbar" style="margin-top: 0.5rem;">
              <button class="primary" @click="save(f.filename)">Save</button>
              <button @click="viewLogs(f.filename)">{{ showLogs ? 'Hide Logs' : 'View Logs' }}</button>
            </div>
            <pre v-if="showLogs" class="logs">{{ logs }}</pre>
          </td>
        </tr>
      </template>
      <tr v-if="!files.length">
        <td colspan="5" class="muted">No quadlet files found.</td>
      </tr>
    </tbody>
  </table>
</template>
