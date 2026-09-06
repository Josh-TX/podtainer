<script setup>
import { ref, computed, onMounted } from 'vue'
import { volumesApi } from '../api'
import CodeEditor from '../components/CodeEditor.vue'
import { humanSize } from '../utils/format'

const props = defineProps({ name: { type: String, required: true } })

const MAX_EDIT_SIZE = 1024 * 1024

const volume = ref(null)
const containers = ref([])
const error = ref('')
const loading = ref(true)

const currentPath = ref('')
const entries = ref([])
const dirError = ref('')
const dirLoading = ref(false)

const selectedName = ref('')
const fileContent = ref('')
const originalContent = ref('')
const fileTooLarge = ref(false)
const fileError = ref('')
const fileBusy = ref(false)

const fileInput = ref(null)
const fileDialog = ref(null)

const isUnchanged = computed(() => fileTooLarge.value || fileContent.value === originalContent.value)

const breadcrumbs = computed(() => {
  const parts = currentPath.value.split('/').filter(Boolean)
  const crumbs = [{ name: props.name, path: '' }]
  let acc = ''
  for (const part of parts) {
    acc = acc ? `${acc}/${part}` : part
    crumbs.push({ name: part, path: acc })
  }
  return crumbs
})

function joinPath(dir, name) {
  return dir ? `${dir}/${name}` : name
}

async function load() {
  error.value = ''
  loading.value = true
  try {
    const data = await volumesApi.get(props.name)
    volume.value = data.volume
    containers.value = data.containers
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function loadDir(path) {
  currentPath.value = path
  closeFile()
  dirError.value = ''
  dirLoading.value = true
  try {
    entries.value = await volumesApi.listDir(props.name, path)
  } catch (e) {
    dirError.value = e.message
    entries.value = []
  } finally {
    dirLoading.value = false
  }
}

function openEntry(entry) {
  if (entry.isDir) {
    loadDir(joinPath(currentPath.value, entry.name))
    return
  }
  openFile(entry)
}

async function openFile(entry) {
  fileError.value = ''
  selectedName.value = entry.name
  fileDialog.value.showModal()
  if (entry.size > MAX_EDIT_SIZE) {
    fileTooLarge.value = true
    fileContent.value = ''
    return
  }
  fileTooLarge.value = false
  try {
    const data = await volumesApi.readFile(props.name, joinPath(currentPath.value, entry.name))
    fileContent.value = data.content
    originalContent.value = data.content
  } catch (e) {
    fileError.value = e.message
  }
}

function closeFile() {
  fileDialog.value?.close()
  selectedName.value = ''
  fileContent.value = ''
  originalContent.value = ''
  fileTooLarge.value = false
  fileError.value = ''
}

function onFileDialogClick(e) {
  if (e.target === fileDialog.value) closeFile()
}

async function saveFile() {
  fileError.value = ''
  fileBusy.value = true
  try {
    await volumesApi.writeFile(props.name, joinPath(currentPath.value, selectedName.value), fileContent.value)
    await loadDir(currentPath.value)
  } catch (e) {
    fileError.value = e.message
  } finally {
    fileBusy.value = false
  }
}

function triggerDownload(path) {
  const a = document.createElement('a')
  a.href = volumesApi.downloadUrl(props.name, path)
  a.download = ''
  document.body.appendChild(a)
  a.click()
  a.remove()
}

async function saveAndDownload() {
  const path = joinPath(currentPath.value, selectedName.value)
  if (isUnchanged.value) {
    triggerDownload(path)
    return
  }
  fileError.value = ''
  fileBusy.value = true
  try {
    await volumesApi.writeFile(props.name, path, fileContent.value)
    triggerDownload(path)
    await loadDir(currentPath.value)
  } catch (e) {
    fileError.value = e.message
  } finally {
    fileBusy.value = false
  }
}

async function newFile() {
  const filename = prompt('New file name:')
  if (!filename) return
  dirError.value = ''
  try {
    await volumesApi.createFile(props.name, joinPath(currentPath.value, filename))
    await loadDir(currentPath.value)
  } catch (e) {
    dirError.value = e.message
  }
}

async function newFolder() {
  const folderName = prompt('New folder name:')
  if (!folderName) return
  dirError.value = ''
  try {
    await volumesApi.mkdir(props.name, joinPath(currentPath.value, folderName))
    await loadDir(currentPath.value)
  } catch (e) {
    dirError.value = e.message
  }
}

async function moveEntry(entry, isCopy) {
  const src = joinPath(currentPath.value, entry.name)
  const dest = prompt(isCopy ? 'Copy to:' : 'Move to:', src)
  if (!dest || dest === src) return
  dirError.value = ''
  try {
    await volumesApi.move(props.name, src, dest, isCopy)
    await loadDir(currentPath.value)
  } catch (e) {
    dirError.value = e.message
  }
}

async function deleteEntry(entry) {
  if (entry.isDir && !confirm(`Delete folder "${entry.name}" and everything in it? This cannot be undone.`)) return
  dirError.value = ''
  try {
    await volumesApi.deleteEntry(props.name, joinPath(currentPath.value, entry.name))
    await loadDir(currentPath.value)
  } catch (e) {
    dirError.value = e.message
  }
}

function downloadUrl(entry) {
  return volumesApi.downloadUrl(props.name, joinPath(currentPath.value, entry.name))
}

function triggerUpload() {
  fileInput.value.click()
}

async function onUploadChange(e) {
  const file = e.target.files[0]
  e.target.value = ''
  if (!file) return
  dirError.value = ''
  try {
    await volumesApi.upload(props.name, joinPath(currentPath.value, file.name), file)
    await loadDir(currentPath.value)
  } catch (err) {
    dirError.value = err.message
  }
}

onMounted(async () => {
  await load()
  await loadDir('')
})
</script>

<template>
  <div class="page-header">
    <h1>{{ name }}</h1>
    <RouterLink to="/volumes" role="button" class="secondary">Back</RouterLink>
  </div>

  <div v-if="error" class="error-banner">{{ error }}</div>
  <p v-else-if="loading" aria-busy="true">Loading…</p>

  <template v-else>
    <article>
      <p style="margin-bottom: 0.25rem">Driver: {{ volume.driver }}</p>
      <p style="margin-bottom: 0.25rem">Mountpoint: <span class="muted">{{ volume.mountpoint }}</span></p>
      <p style="margin-bottom: 0.25rem">Created: <span class="muted">{{ volume.createdAt }}</span></p>
      <p style="margin-bottom: 0">
        Used by:
        <template v-if="containers.length">
          <template v-for="(c, i) in containers" :key="c.id">
            <span v-if="i" class="muted">, </span>
            <span :class="['badge', c.state]">{{ c.state }}</span>
            <RouterLink :to="`/containers/${encodeURIComponent(c.id)}`">{{ c.names?.[0] || c.id.slice(0, 12) }}</RouterLink>
          </template>
        </template>
        <span v-else class="muted">no containers</span>
      </p>
    </article>

    <div class="toolbar">
      <nav aria-label="breadcrumb">
        <ul>
          <li v-for="(crumb, i) in breadcrumbs" :key="crumb.path">
            <a v-if="i < breadcrumbs.length - 1" href="#" @click.prevent="loadDir(crumb.path)">{{ crumb.name }}</a>
            <strong v-else>{{ crumb.name }}</strong>
          </li>
        </ul>
      </nav>
    </div>

    <div class="toolbar">
      <button class="secondary" @click="newFile">New File</button>
      <button class="secondary" @click="newFolder">New Folder</button>
      <button class="secondary" @click="triggerUpload">Upload</button>
      <input ref="fileInput" type="file" style="display: none" @change="onUploadChange" />
    </div>

    <div v-if="dirError" class="error-banner">{{ dirError }}</div>
    <p v-if="dirLoading" aria-busy="true">Loading…</p>
    <template v-else>
      <table class="rows">
        <thead>
          <tr>
            <th>Name</th>
            <th>Size</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="entry in entries" :key="entry.name">
            <td>
              <a href="#" class="row-link" @click.prevent="openEntry(entry)">{{ entry.isDir ? '📁 ' : '📄 ' }}{{ entry.name }}</a>
            </td>
            <td class="muted">{{ entry.isDir ? '—' : humanSize(entry.size) }}</td>
            <td style="position: relative; z-index: 1; white-space: nowrap">
              <a href="#" @click.prevent="moveEntry(entry, false)">Move</a>
              &nbsp;
              <a href="#" @click.prevent="moveEntry(entry, true)">Copy</a>
              &nbsp;
              <a href="#" class="danger-link" @click.prevent="deleteEntry(entry)">Delete</a>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!entries.length" class="muted">Empty directory.</p>
    </template>

    <dialog ref="fileDialog" @click="onFileDialogClick">
      <article style="width: min(90vw, 60rem); max-width: min(90vw, 60rem)">
        <div class="page-header">
          <h4>{{ selectedName }}</h4>
          <div class="toolbar" style="margin-bottom: 0">
            <button v-if="!fileTooLarge" :disabled="fileBusy || isUnchanged" @click="saveFile">Save</button>
            <details class="dropdown">
              <summary role="button" class="secondary">Actions</summary>
              <ul>
                <li><a href="#" @click.prevent="!fileBusy && saveAndDownload()">{{ isUnchanged ? 'Download' : 'Save & Download' }}</a></li>
                <li><a href="#" @click.prevent="moveEntry({ name: selectedName }, false)">Move</a></li>
                <li><a href="#" @click.prevent="moveEntry({ name: selectedName }, true)">Copy</a></li>
                <li><a href="#" class="danger-link" @click.prevent="deleteEntry({ name: selectedName, isDir: false })">Delete</a></li>
              </ul>
            </details>
            <button class="secondary" @click="closeFile">Close</button>
          </div>
        </div>
        <div v-if="fileError" class="error-banner">{{ fileError }}</div>
        <p v-if="fileTooLarge" class="muted">File is larger than 1MB — too big to edit here.</p>
        <CodeEditor v-else v-model="fileContent" max-height="60vh" />
      </article>
    </dialog>
  </template>
</template>
