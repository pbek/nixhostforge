<script setup>
defineProps({
  title: { type: String, required: true },
  subtitle: { type: String, default: '' },
})
</script>

<template>
  <v-card class="hero-card mb-4" rounded="xl">
    <v-card-text class="pa-6 pa-md-8">
      <p class="eyebrow mb-2">NixHostForge</p>
      <h1 class="text-h3 text-md-h2 font-weight-bold mb-3">{{ title }}</h1>
      <p class="text-medium-emphasis mb-0">{{ subtitle }}</p>
    </v-card-text>
  </v-card>
</template>
