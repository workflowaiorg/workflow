<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { getHealth, type HealthStatus } from '@/services/healthService'

const health = ref<HealthStatus>()
const failed = ref(false)

onMounted(async () => {
  try {
    health.value = await getHealth()
  } catch {
    failed.value = true
  }
})
</script>

<template>
  <main>
    <h1>It works</h1>
    <p v-if="failed">The API did not answer.</p>
    <p v-else-if="health">The API says: {{ health.status }}</p>
    <p v-else>Asking the API…</p>
  </main>
</template>
