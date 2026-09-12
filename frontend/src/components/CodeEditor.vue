<script setup>
import { onMounted, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { EditorState } from '@codemirror/state'
import { EditorView, lineNumbers, keymap } from '@codemirror/view'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { StreamLanguage, HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { yaml } from '@codemirror/lang-yaml'
import { toml } from '@codemirror/legacy-modes/mode/toml'
import { tags as t } from '@lezer/highlight'

const highlightStyle = HighlightStyle.define([
  { tag: t.comment, color: 'light-dark(#6a737d, #8b949e)', fontStyle: 'italic' },
  { tag: [t.keyword, t.bool, t.null], color: 'light-dark(#a626a4, #c678dd)' },
  { tag: [t.propertyName, t.attributeName, t.definition(t.propertyName)], color: 'light-dark(#005cc5, #79b8ff)' },
  { tag: [t.string, t.special(t.string)], color: 'light-dark(#22863a, #85e89d)' },
  { tag: [t.number], color: 'light-dark(#b08800, #e3b341)' },
  { tag: [t.atom, t.className, t.typeName], color: 'light-dark(#e36209, #ffab70)' },
  { tag: t.meta, color: 'light-dark(#6a737d, #8b949e)' },
])

const props = defineProps({
  modelValue: { type: String, default: '' },
  readonly: { type: Boolean, default: false },
  language: { type: String, default: 'text' }, // 'yaml' | 'unit' | 'text'
  autoscroll: { type: Boolean, default: false },
  maxHeight: { type: String, default: '400px' },
})
const emit = defineEmits(['update:modelValue'])

const el = ref(null)
const view = shallowRef(null)
let syncingFromProp = false

function languageExtension() {
  if (props.language === 'yaml') return yaml()
  if (props.language === 'unit') return StreamLanguage.define(toml)
  return []
}

function scrollToBottom() {
  if (!view.value) return
  const end = view.value.state.doc.length
  view.value.dispatch({ selection: { anchor: end }, scrollIntoView: true })
}

onMounted(() => {
  const updateListener = EditorView.updateListener.of((update) => {
    if (update.docChanged && !props.readonly && !syncingFromProp) {
      emit('update:modelValue', update.state.doc.toString())
    }
  })

  const state = EditorState.create({
    doc: props.modelValue,
    extensions: [
      lineNumbers(),
      EditorView.lineWrapping,
      history(),
      keymap.of([...defaultKeymap, ...historyKeymap]),
      languageExtension(),
      syntaxHighlighting(highlightStyle),
      EditorView.editable.of(!props.readonly),
      EditorState.readOnly.of(props.readonly),
      updateListener,
      EditorView.theme({
        '&': { fontSize: '0.8rem', height: props.maxHeight },
        '.cm-scroller': { overflow: 'auto', fontFamily: 'var(--pico-font-family-monospace, monospace)' },
        '.cm-gutters': { backgroundColor: 'var(--pico-code-background-color)', color: 'var(--pico-muted-color)', border: 'none' },
        '&, .cm-content': { backgroundColor: 'var(--pico-code-background-color)', color: 'var(--pico-color)', caretColor: 'var(--pico-color)' },
        '.cm-cursor': { borderLeftColor: 'var(--pico-color)' },
      }),
    ],
  })

  view.value = new EditorView({ state, parent: el.value })

  if (props.autoscroll) scrollToBottom()
})

onBeforeUnmount(() => {
  view.value?.destroy()
})

watch(
  () => props.modelValue,
  (newValue) => {
    if (!view.value) return
    const current = view.value.state.doc.toString()
    if (newValue === current) return
    syncingFromProp = true
    view.value.dispatch({
      changes: { from: 0, to: current.length, insert: newValue },
    })
    syncingFromProp = false
    if (props.autoscroll) scrollToBottom()
  }
)
</script>

<template>
  <div ref="el" class="code-editor" :class="{ readonly }"></div>
</template>

<style scoped>
.code-editor {
  border-radius: var(--pico-border-radius);
  overflow: hidden;
  border: var(--pico-border-width) solid var(--pico-form-element-border-color);
}
.code-editor.readonly {
  border: none;
}
</style>
