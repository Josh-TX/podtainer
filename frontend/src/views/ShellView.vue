<script setup>
import { ref, computed, onMounted } from 'vue'
import { shellApi } from '../api'
import ShellTerminal from '../components/ShellTerminal.vue'
import ShellTabs from '../components/ShellTabs.vue'

const sessions = ref([])
const sessionNums = ref({}) // id -> number
const activeId = ref(null)
const error = ref('')
const loading = ref(true)

function nextSessionNum() {
  const used = new Set(Object.values(sessionNums.value))
  let n = 1
  while (used.has(n)) n++
  return n
}

async function load() {
  error.value = ''
  try {
    sessions.value = await shellApi.list()
    for (const s of sessions.value) {
      sessionNums.value[s.id] = nextSessionNum()
    }
    if (!sessions.value.length) {
      await addSession()
    } else {
      activeId.value = sessions.value[0].id
    }
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function addSession() {
  error.value = ''
  try {
    const s = await shellApi.create()
    sessionNums.value[s.id] = nextSessionNum()
    sessions.value.push(s)
    activeId.value = s.id
  } catch (e) {
    error.value = e.message
  }
}

async function closeSession(id) {
  error.value = ''
  try {
    await shellApi.close(id)
    sessions.value = sessions.value.filter((s) => s.id !== id)
    delete sessionNums.value[id]
    if (activeId.value === id) {
      activeId.value = sessions.value[0]?.id || null
    }
  } catch (e) {
    error.value = e.message
  }
}

const sessionTabs = computed(() =>
  sessions.value.map((s) => ({ label: `Session ${sessionNums.value[s.id]}`, id: s.id }))
)

onMounted(load)
</script>

<template>
  <div class="shell-view">
    <div v-if="error" class="error-banner">{{ error }}</div>
    <p v-if="loading" aria-busy="true">Loading…</p>
    <template v-else>
      <div class="toolbar tabs-row">
        <ShellTabs :tabs="sessionTabs" :model-value="activeId" @update:model-value="(v) => (activeId = v)" @close="closeSession" />
        <button class="icon-btn" title="New session" @click="addSession">+</button>
      </div>
      <ShellTerminal v-if="activeId" :key="activeId" :ws-url="shellApi.wsUrl(activeId)" />
    </template>
  </div>
</template>

<style scoped>
.shell-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}
.tabs-row {
  margin-bottom: 0.5rem;
  flex: none;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.icon-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  margin: 0;
  padding: 0;
  background: none;
  border: none;
  border-radius: var(--pico-border-radius);
  color: var(--pico-h1-color);
  font-size: 1.1rem;
  line-height: 1;
  cursor: pointer;
}
.icon-btn:hover {
  background-color: var(--pico-dropdown-hover-background-color);
}
</style>
