<script setup>
defineProps({
  builds: { type: Array, default: () => [] },
})

defineEmits(['navigate'])

function shortCommit(value) {
  return value ? value.slice(0, 12) : 'unknown'
}

function formatDate(value) {
  if (!value || value.startsWith('0001-')) return 'never'
  return new Date(value).toLocaleString()
}

function duration(item) {
  if (!item?.startedAt) return ''
  const end = item.finishedAt ? new Date(item.finishedAt) : new Date()
  const seconds = Math.max(0, Math.round((end - new Date(item.startedAt)) / 1000))
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ${seconds % 60}s`
  const hours = Math.floor(minutes / 60)
  return `${hours}h ${minutes % 60}m`
}
</script>

<template>
  <v-card class="settings-card" rounded="xl">
    <v-card-title>Recent builds</v-card-title>
    <v-table>
      <thead>
        <tr>
          <th>ID</th>
          <th>Host</th>
          <th>Commit</th>
          <th>Status</th>
          <th>Started</th>
          <th>Duration</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="build in builds" :key="build.id">
          <td>
            <a :href="`/builds/${build.id}`" @click.prevent="$emit('navigate', `/builds/${build.id}`)">#{{ build.id }}</a>
          </td>
          <td>{{ build.host }}</td>
          <td><span class="readonly-value">{{ shortCommit(build.commitHash) }}</span></td>
          <td><v-chip size="small" :class="`status-${build.status}`">{{ build.status }}</v-chip></td>
          <td>{{ formatDate(build.startedAt) }}</td>
          <td>{{ duration(build) }}</td>
        </tr>
        <tr v-if="!builds.length">
          <td colspan="6">No builds yet.</td>
        </tr>
      </tbody>
    </v-table>
  </v-card>
</template>
