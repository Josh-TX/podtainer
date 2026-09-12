import { ref } from 'vue'

// Module-scoped so filters survive navigating away from SystemdView and
// back, resetting only on a full page refresh.
export const showQuadlet = ref(true)
export const showFavorite = ref(true)
export const showAll = ref(false)
export const filterText = ref('')
