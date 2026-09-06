import { reactive } from 'vue'

// Shared across the app so a 401 from any API call (e.g. an expired
// session) can bounce the user back to the login screen immediately.
export const authState = reactive({
  checked: false,
  authenticated: false,
  needsSetup: false,
})
