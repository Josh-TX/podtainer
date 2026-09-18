<script setup>
import { ref, nextTick } from 'vue'

const props = defineProps({
  modelValue: { type: String, required: true },
  fetchSuggestions: { type: Function, required: true }, // async (query) => string[]
  placeholder: { type: String, default: '' },
})
const emit = defineEmits(['update:modelValue'])

const DEBOUNCE_MS = 300

const matches = ref([])
const showDropdown = ref(false)
const highlightedIndex = ref(-1)
let debounceTimer = null
let requestId = 0

async function runFetch(query) {
  const id = ++requestId
  let results = []
  try {
    results = await props.fetchSuggestions(query)
  } catch {
    results = []
  }
  if (id !== requestId) return // a newer keystroke/selection superseded this request
  matches.value = results
  showDropdown.value = results.length > 0
  highlightedIndex.value = -1
}

function scheduleFetch(query) {
  clearTimeout(debounceTimer)
  if (!query) {
    matches.value = []
    showDropdown.value = false
    highlightedIndex.value = -1
    return
  }
  debounceTimer = setTimeout(() => runFetch(query), DEBOUNCE_MS)
}

function onInput(e) {
  emit('update:modelValue', e.target.value)
  scheduleFetch(e.target.value)
}

function select(value) {
  emit('update:modelValue', value)
  if (value.endsWith('/')) {
    // drilled into a directory - skip the debounce and list it immediately
    clearTimeout(debounceTimer)
    runFetch(value)
  } else {
    showDropdown.value = false
    highlightedIndex.value = -1
  }
}

function onBlur() {
  setTimeout(() => { showDropdown.value = false }, 150)
}

function scrollHighlightedIntoView(container) {
  nextTick(() => container?.querySelector('li.active')?.scrollIntoView({ block: 'nearest' }))
}

function onKeydown(e) {
  if (!showDropdown.value || matches.value.length === 0) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    highlightedIndex.value = Math.min(highlightedIndex.value + 1, matches.value.length - 1)
    scrollHighlightedIntoView(e.target.nextElementSibling)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    highlightedIndex.value = Math.max(highlightedIndex.value - 1, -1)
    scrollHighlightedIntoView(e.target.nextElementSibling)
  } else if (e.key === 'Enter') {
    if (highlightedIndex.value < 0) return
    e.preventDefault()
    select(matches.value[highlightedIndex.value])
  } else if (e.key === 'Tab') {
    if (highlightedIndex.value < 0) return
    select(matches.value[highlightedIndex.value])
  } else if (e.key === 'Escape') {
    showDropdown.value = false
    highlightedIndex.value = -1
  }
}
</script>

<template>
  <div class="typeahead">
    <input
      :value="modelValue"
      :placeholder="placeholder"
      @input="onInput"
      @blur="onBlur"
      @focus="scheduleFetch(modelValue)"
      @keydown="onKeydown"
    />
    <ul v-if="showDropdown" class="typeahead-menu">
      <li
        v-for="(opt, i) in matches"
        :key="opt"
        :class="{ active: i === highlightedIndex }"
        @mousedown.prevent="select(opt)"
        @mouseenter="highlightedIndex = i"
      >{{ opt }}</li>
    </ul>
  </div>
</template>

<style scoped>
.typeahead {
  position: relative;
  margin-bottom: var(--pico-spacing);
}
.typeahead input {
  margin-bottom: 0;
}
.typeahead-menu {
  position: absolute;
  z-index: 10;
  top: 100%;
  left: 0;
  right: 0;
  margin: 0.15rem 0 0;
  padding: 0;
  list-style: none;
  max-height: 12rem;
  overflow-y: auto;
  background-color: color-mix(in srgb, var(--pico-background-color) 80%, black);
  border: var(--pico-border-width) solid var(--pico-table-border-color);
  border-radius: var(--pico-border-radius);
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.45);
}
.typeahead-menu li {
  padding: 0.4rem 0.75rem;
  cursor: pointer;
}
.typeahead-menu li:hover,
.typeahead-menu li.active {
  background-color: color-mix(in srgb, var(--pico-primary) 5%, transparent);
}
</style>
