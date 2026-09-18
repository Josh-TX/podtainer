<script setup>
defineProps({
  options: { type: Array, required: true }, // [{ label, value, disabled, title }]
  modelValue: { type: [String, Number], required: true },
})
const emit = defineEmits(['update:modelValue'])
</script>

<template>
  <nav class="nav-buttons">
    <ul>
      <li v-for="opt in options" :key="opt.value" :class="{ active: opt.value === modelValue, disabled: opt.disabled }">
        <a href="#" :title="opt.title || ''" @click.prevent="!opt.disabled && emit('update:modelValue', opt.value)">{{ opt.label }}</a>
      </li>
    </ul>
  </nav>
</template>

<style scoped>
.nav-buttons {
  display: inline-block;
  background-color: var(--pico-code-background-color);
  border: var(--pico-border-width) solid var(--pico-table-border-color);
  border-radius: var(--pico-border-radius);
  overflow: hidden;
}
.nav-buttons ul {
  display: flex;
  list-style: none;
  margin: 0;
  padding: 0;
  gap: 0;
}
.nav-buttons li {
  position: relative;
  margin: 0;
  padding: 0;
}
.nav-buttons li.disabled {
  opacity: 0.5;
  pointer-events: none;
}
.nav-buttons a {
  display: block;
  margin: 0;
  padding: 8px 1rem;
  color: var(--pico-h1-color);
  text-decoration: none;
}
.nav-buttons a:hover {
  background-color: var(--pico-dropdown-hover-background-color);
}
.nav-buttons li.active::after {
  content: "";
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 2px;
  background-color: var(--pico-primary);
}
</style>
