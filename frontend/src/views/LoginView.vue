<script setup>
import { ref } from 'vue'
import { authApi } from '../api'
import { authState } from '../api/authState.js'

const password = ref('')
const confirm = ref('')
const error = ref('')
const submitting = ref(false)

async function submit() {
  error.value = ''
  if (authState.needsSetup && password.value !== confirm.value) {
    error.value = 'Passwords do not match'
    return
  }
  submitting.value = true
  try {
    if (authState.needsSetup) {
      await authApi.setup(password.value)
      authState.needsSetup = false
    } else {
      await authApi.login(password.value)
    }
    authState.authenticated = true
  } catch (e) {
    error.value = e.message
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <main class="container">
    <article style="max-width: 24rem; margin: 4rem auto">
      <h1>Podtainer</h1>
      <h2 v-if="authState.needsSetup">Enter a new password for Podtainer</h2>
      <h2 v-else>Log in</h2>
      <form @submit.prevent="submit">
        <label>
          Password
          <input type="password" v-model="password" autofocus required />
        </label>
        <label v-if="authState.needsSetup">
          Confirm password
          <input type="password" v-model="confirm" required />
        </label>
        <p v-if="error" style="color: var(--pico-del-color)">{{ error }}</p>
        <button type="submit" :aria-busy="submitting">
          {{ authState.needsSetup ? 'Set password' : 'Log in' }}
        </button>
      </form>
    </article>
  </main>
</template>
