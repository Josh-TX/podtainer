<script setup>
defineProps({
  tabs: { type: Array, required: true }, // [{ id, label }]
  modelValue: { type: [String, Number], required: true },
})
const emit = defineEmits(['update:modelValue', 'close'])
</script>

<template>
  <nav class="shell-tabs">
    <ul>
      <li v-for="tab in tabs" :key="tab.id" :class="{ active: tab.id === modelValue }">
        <a href="#" @click.prevent="emit('update:modelValue', tab.id)">
          {{ tab.label }}
          <span class="close-btn" title="Close session" @click.stop.prevent="emit('close', tab.id)">&times;</span>
        </a>
      </li>
    </ul>
  </nav>
</template>

<style scoped>
.shell-tabs {
  display: inline-block;
  background-color: var(--pico-code-background-color);
  border: var(--pico-border-width) solid var(--pico-table-border-color);
  border-radius: var(--pico-border-radius);
  overflow: hidden;
}
.shell-tabs ul {
  display: flex;
  list-style: none;
  margin: 0;
  padding: 0;
  gap: 0;
}
.shell-tabs li {
  position: relative;
  margin: 0;
  padding: 0;
}
.shell-tabs a {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin: 0;
  padding: 8px 0.75rem 8px 1rem;
  color: var(--pico-h1-color);
  text-decoration: none;
}
.shell-tabs a:hover {
  background-color: var(--pico-dropdown-hover-background-color);
}
.shell-tabs li.active::after {
  content: "";
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 2px;
  background-color: var(--pico-primary);
}
.close-btn {
  line-height: 1;
  padding: 0 0.15rem;
  border-radius: var(--pico-border-radius);
}
.close-btn:hover {
  background-color: var(--pico-del-color);
  color: var(--pico-contrast-inverse);
}
</style>
