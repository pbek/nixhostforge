<script setup>
import { computed, ref, watch } from "vue";

defineEmits(["navigate"]);

const groupOptions = [
  { title: "None", value: "none" },
  { title: "Host", value: "host" },
];
const groupStorageKey = "nixhostforge.buildTable.groupBy";

const props = defineProps({
  builds: { type: Array, default: () => [] },
  groupable: { type: Boolean, default: false },
});

const groupBy = ref(savedGroupBy());
const groupedByHost = computed(
  () => props.groupable && groupBy.value === "host",
);

const hostGroups = computed(() => {
  const groups = new Map();
  for (const build of props.builds) {
    const host = build.host || "unknown";
    if (!groups.has(host)) groups.set(host, []);
    groups.get(host).push(build);
  }
  return [...groups]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([host, items]) => ({ host, builds: items }));
});

watch(groupBy, (value) => {
  if (!props.groupable || typeof localStorage === "undefined") return;
  localStorage.setItem(groupStorageKey, value);
});

function savedGroupBy() {
  if (typeof localStorage === "undefined") return "none";
  const value = localStorage.getItem(groupStorageKey);
  return groupOptions.some((option) => option.value === value) ? value : "none";
}

function shortCommit(value) {
  return value ? value.slice(0, 12) : "unknown";
}

function formatDate(value) {
  if (!value || value.startsWith("0001-")) return "never";
  return new Date(value).toLocaleString();
}

function duration(item) {
  if (!item?.startedAt) return "";
  const end = item.finishedAt ? new Date(item.finishedAt) : new Date();
  const seconds = Math.max(
    0,
    Math.round((end - new Date(item.startedAt)) / 1000),
  );
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ${seconds % 60}s`;
  const hours = Math.floor(minutes / 60);
  return `${hours}h ${minutes % 60}m`;
}
</script>

<template>
  <v-card class="settings-card" rounded="xl">
    <v-card-title class="build-table-title">
      <span>Recent builds</span>
      <v-select
        v-if="groupable"
        v-model="groupBy"
        class="build-group-select"
        density="compact"
        hide-details
        :items="groupOptions"
        label="Group by"
        variant="outlined"
      />
    </v-card-title>
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
      <tbody v-if="!groupedByHost">
        <tr v-for="build in builds" :key="build.id">
          <td>
            <a
              :href="`/builds/${build.id}`"
              @click.prevent="$emit('navigate', `/builds/${build.id}`)"
              >#{{ build.id }}</a
            >
          </td>
          <td>{{ build.host }}</td>
          <td>
            <span class="readonly-value">{{
              shortCommit(build.commitHash)
            }}</span>
          </td>
          <td>
            <v-chip size="small" :class="`status-${build.status}`">{{
              build.status
            }}</v-chip>
          </td>
          <td>{{ formatDate(build.startedAt) }}</td>
          <td>{{ duration(build) }}</td>
        </tr>
        <tr v-if="!builds.length">
          <td colspan="6">No builds yet.</td>
        </tr>
      </tbody>
      <template v-else-if="builds.length">
        <tbody v-for="group in hostGroups" :key="group.host">
          <tr class="build-host-group-row">
            <td colspan="6">{{ group.host }}</td>
          </tr>
          <tr v-for="build in group.builds" :key="build.id">
            <td>
              <a
                :href="`/builds/${build.id}`"
                @click.prevent="$emit('navigate', `/builds/${build.id}`)"
                >#{{ build.id }}</a
              >
            </td>
            <td>{{ build.host }}</td>
            <td>
              <span class="readonly-value">{{
                shortCommit(build.commitHash)
              }}</span>
            </td>
            <td>
              <v-chip size="small" :class="`status-${build.status}`">{{
                build.status
              }}</v-chip>
            </td>
            <td>{{ formatDate(build.startedAt) }}</td>
            <td>{{ duration(build) }}</td>
          </tr>
        </tbody>
      </template>
      <tbody v-else>
        <tr>
          <td colspan="6">No builds yet.</td>
        </tr>
      </tbody>
    </v-table>
  </v-card>
</template>
