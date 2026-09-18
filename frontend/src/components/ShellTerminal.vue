<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'

const props = defineProps({ wsUrl: { type: String, required: true } })

const containerRef = ref(null)
const connected = ref(false)
let term = null
let ws = null
let resizeObserver = null

function sendResize(fitAddon) {
  fitAddon.fit()
  if (ws?.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
  }
}

onMounted(() => {
  term = new Terminal({ convertEol: true, cursorBlink: true })
  const fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(containerRef.value)
  fitAddon.fit()

  ws = new WebSocket(props.wsUrl)
  ws.binaryType = 'arraybuffer'
  ws.onmessage = (ev) => {
    term.write(typeof ev.data === 'string' ? ev.data : new Uint8Array(ev.data))
  }
  term.onData((data) => {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'input', data }))
    }
  })
  ws.onopen = () => {
    connected.value = true
    sendResize(fitAddon)
  }
  ws.onclose = () => {
    connected.value = false
  }
  ws.onerror = () => {
    connected.value = false
  }

  resizeObserver = new ResizeObserver(() => sendResize(fitAddon))
  resizeObserver.observe(containerRef.value)
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  ws?.close()
  term?.dispose()
})
</script>

<template>
  <div class="shell-terminal-wrap">
    <span class="status-dot" :class="{ connected }" :title="connected ? 'Connected' : 'Disconnected'"></span>
    <div ref="containerRef" class="shell-terminal"></div>
  </div>
</template>

<style scoped>
.shell-terminal-wrap {
  position: relative;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 20rem;
}
.shell-terminal {
  flex: 1;
  min-height: 0;
}
.status-dot {
  position: absolute;
  top: 0.5rem;
  right: 0.5rem;
  width: 0.35rem;
  height: 0.35rem;
  border-radius: 50%;
  background: #d33;
  z-index: 1;
}
.status-dot.connected {
  background: #2ea043;
}
</style>
