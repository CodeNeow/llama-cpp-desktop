<template>
  <!-- Neutral pass-through panel: the smart default resolves within one cheap
       probe round-trip (the backend caches GetLlamaCpp), so no chrome is
       rendered here. -->
  <div class="env-default" aria-hidden="true"></div>
</template>

<script setup lang="ts">
import { onActivated, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getLlamaCpp } from '../wails'

// Smart default for the bare / route (the '' child of the Home shell):
// llama.cpp missing → land on the Runtime tab (install actions first);
// installed → land on the System Info tab. A probe failure (e.g. standalone
// vite without the Wails backend) is treated as installed so a backend-less
// dev page keeps the hardware default instead of being forced onto the
// runtime tab.
const route = useRoute()
const router = useRouter()

// Guards against a second concurrent resolution: onMounted and onActivated
// both fire on the first keep-alive mount
let resolving = false

async function resolveDefaultTab() {
  if (resolving) return
  // Only act while still on the bare route: if the user already navigated to
  // a tab (or away) during the probe, do not yank them around
  if (route.path !== '/') return
  resolving = true
  let installed = true
  try {
    installed = (await getLlamaCpp())?.installed === true
  } catch {
    // Probe failure keeps installed=true (see the smart-default comment above)
  }
  resolving = false
  // Re-check after the await: the user may have navigated meanwhile
  if (route.path !== '/') return
  void router.replace(installed ? '/system' : '/runtime')
}

// onActivated covers keep-alive re-entries (returning to / re-runs the
// resolution; onMounted alone would strand the user on this blank panel)
onMounted(resolveDefaultTab)
onActivated(resolveDefaultTab)
</script>

<style scoped>
/* Empty placeholder body: just reserves a little vertical room so the panel
   never renders zero-height */
.env-default {
  min-height: 120px;
}
</style>
