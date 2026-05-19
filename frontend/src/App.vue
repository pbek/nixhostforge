<script setup>
import { computed, onMounted, onUnmounted, reactive, ref } from "vue";
import BuildTable from "./BuildTable.vue";
import PageTitle from "./PageTitle.vue";

const path = ref(window.location.pathname);
const loading = ref(true);
const auth = reactive({ hasAdmin: true, authenticated: false });
const loginForm = reactive({ password: "", confirm: "" });
const dashboard = ref(null);
const hosts = ref([]);
const builds = ref([]);
const build = ref(null);
const selectedPauseHours = ref(1);
const saving = reactive({
  repository: false,
  scheduler: false,
  notifications: false,
  test: false,
  login: false,
  setup: false,
  host: "",
  build: "",
  buildCurrent: false,
  pause: false,
  check: false,
});
const snackbar = reactive({ color: "success", message: "", show: false });
const settings = reactive({
  repository: {
    repository: "",
    branch: "main",
    mutable: false,
    configured: false,
  },
  scheduler: {
    interval: "15m0s",
    intervalMutable: false,
    concurrency: 1,
    concurrencyMutable: false,
  },
  notificationUrls: [{ url: "", enabled: true }],
});
const appVersion = import.meta.env.VITE_NIXHOSTFORGE_VERSION || "dev";

const authenticatedPage = computed(
  () => !["/login", "/setup"].includes(path.value),
);
const currentBuildId = computed(() =>
  path.value.startsWith("/builds/") ? path.value.replace("/builds/", "") : "",
);
const schedulerMutable = computed(
  () =>
    settings.scheduler.intervalMutable || settings.scheduler.concurrencyMutable,
);

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

function notify(message, color = "success") {
  snackbar.message = message;
  snackbar.color = color;
  snackbar.show = true;
}

function newNotificationUrl() {
  return {
    url: "",
    enabled: true,
    success: false,
    warnings: false,
    errors: true,
  };
}

function applySettings(next) {
  settings.repository = { ...next.repository };
  settings.scheduler = { ...next.scheduler };
  const targets = Array.isArray(next.notificationUrls)
    ? next.notificationUrls
    : (next.notificationUrl || "")
        .split(/\r?\n/)
        .filter(Boolean)
        .map((url) => ({ url, enabled: true, errors: true }));
  settings.notificationUrls = targets.length
    ? targets.map((target) => ({
        url: target.url || "",
        enabled: target.enabled !== false,
        success: target.success === true,
        warnings: target.warnings === true,
        errors: target.errors !== false,
      }))
    : [newNotificationUrl()];
}

function applyDashboard(data) {
  dashboard.value = data;
  hosts.value = data.hosts || [];
  builds.value = data.builds || [];
  settings.repository = { ...data.repository };
  settings.scheduler = { ...data.scheduler };
}

async function request(url, options = {}) {
  const response = await fetch(url, {
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
    ...options,
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error || "Request failed");
  return data;
}

function navigate(next) {
  window.history.pushState({}, "", next);
  path.value = window.location.pathname;
  loadPage();
}

async function loadAuth() {
  const data = await request("/api/auth");
  auth.hasAdmin = data.hasAdmin;
  auth.authenticated = data.authenticated;
  if (!auth.hasAdmin && path.value !== "/setup") navigate("/setup");
  if (auth.hasAdmin && !auth.authenticated && authenticatedPage.value)
    navigate("/login");
}

async function loadPage() {
  loading.value = true;
  try {
    await loadAuth();
    if (!auth.authenticated && auth.hasAdmin) return;
    if (!auth.hasAdmin) return;
    if (path.value === "/" || path.value === "/settings") {
      if (path.value === "/") applyDashboard(await request("/api/dashboard"));
      if (path.value === "/settings")
        applySettings(await request("/api/settings"));
    } else if (path.value === "/hosts") {
      hosts.value = (await request("/api/hosts")).hosts || [];
    } else if (path.value === "/builds") {
      builds.value = (await request("/api/builds")).builds || [];
    } else if (path.value.startsWith("/builds/")) {
      build.value = (
        await request(`/api/builds/${currentBuildId.value}`)
      ).build;
    }
  } catch (error) {
    notify(error.message, "error");
  } finally {
    loading.value = false;
  }
}

async function setup() {
  saving.setup = true;
  try {
    const data = await request("/api/setup", {
      method: "POST",
      body: JSON.stringify(loginForm),
    });
    auth.hasAdmin = data.hasAdmin;
    auth.authenticated = data.authenticated;
    navigate("/");
  } catch (error) {
    notify(error.message, "error");
  } finally {
    saving.setup = false;
  }
}

async function login() {
  saving.login = true;
  try {
    const data = await request("/api/login", {
      method: "POST",
      body: JSON.stringify(loginForm),
    });
    auth.hasAdmin = data.hasAdmin;
    auth.authenticated = data.authenticated;
    navigate("/");
  } catch (error) {
    notify(error.message, "error");
  } finally {
    saving.login = false;
  }
}

async function checkNow() {
  saving.check = true;
  try {
    await request("/api/check-now", { method: "POST", body: "{}" });
    notify("Check started.");
  } catch (error) {
    notify(error.message, "error");
  } finally {
    saving.check = false;
  }
}

async function pauseBuilds() {
  saving.pause = true;
  try {
    const data = await request("/api/pause", {
      method: "POST",
      body: JSON.stringify({ hours: selectedPauseHours.value }),
    });
    if (dashboard.value) dashboard.value.status = data.status;
    notify("Builds paused and running jobs cancelled.");
  } catch (error) {
    notify(error.message, "error");
  } finally {
    saving.pause = false;
  }
}

async function resumeBuilds() {
  saving.pause = true;
  try {
    const data = await request("/api/resume", { method: "POST", body: "{}" });
    if (dashboard.value) dashboard.value.status = data.status;
    notify("Builds resumed.");
  } catch (error) {
    notify(error.message, "error");
  } finally {
    saving.pause = false;
  }
}

async function toggleHost(host) {
  saving.host = host.name;
  try {
    hosts.value =
      (
        await request("/api/hosts/toggle", {
          method: "POST",
          body: JSON.stringify({ host: host.name, enabled: host.enabled }),
        })
      ).hosts || [];
  } catch (error) {
    host.enabled = !host.enabled;
    notify(error.message, "error");
  } finally {
    saving.host = "";
  }
}

async function buildHost(host) {
  saving.build = host.name;
  try {
    await request("/api/hosts/build", {
      method: "POST",
      body: JSON.stringify({ host: host.name }),
    });
    notify(`Build started for ${host.name}.`);
  } catch (error) {
    notify(error.message, "error");
  } finally {
    saving.build = "";
  }
}

async function buildCurrentHosts() {
  saving.buildCurrent = true;
  try {
    const data = await request("/api/hosts/build-current", {
      method: "POST",
      body: "{}",
    });
    const count = data.count || 0;
    notify(
      count === 0
        ? "No enabled hosts to build."
        : count === 1
          ? "Build started for 1 enabled host."
          : `Builds started for ${count} enabled hosts.`,
    );
    await loadPage();
  } catch (error) {
    notify(error.message, "error");
  } finally {
    saving.buildCurrent = false;
  }
}

async function saveRepository() {
  saving.repository = true;
  try {
    applySettings(
      await request("/api/settings/repository", {
        method: "POST",
        body: JSON.stringify(settings.repository),
      }),
    );
    notify("Repository settings saved. A check has been started.");
  } catch (error) {
    notify(error.message, "error");
  } finally {
    saving.repository = false;
  }
}

async function saveScheduler() {
  saving.scheduler = true;
  try {
    applySettings(
      await request("/api/settings/scheduler", {
        method: "POST",
        body: JSON.stringify(settings.scheduler),
      }),
    );
    notify("Scheduler settings saved.");
  } catch (error) {
    notify(error.message, "error");
  } finally {
    saving.scheduler = false;
  }
}

async function saveNotifications() {
  saving.notifications = true;
  try {
    applySettings(
      await request("/api/settings/notifications", {
        method: "POST",
        body: JSON.stringify({ notificationUrls: settings.notificationUrls }),
      }),
    );
    notify("Notification settings saved.");
  } catch (error) {
    notify(error.message, "error");
  } finally {
    saving.notifications = false;
  }
}

async function testNotification(index) {
  const target = settings.notificationUrls[index];
  if (!target?.url?.trim()) {
    notify("Notification URL must not be empty.", "error");
    return;
  }
  saving.test = index + 1;
  try {
    const response = await request("/api/settings/notifications/test", {
      method: "POST",
      body: JSON.stringify({
        notificationUrls: [{ url: target.url, enabled: true }],
      }),
    });
    notify(response.message || "Test notification sent.");
  } catch (error) {
    notify(error.message, "error");
  } finally {
    saving.test = false;
  }
}

function addNotificationUrl() {
  settings.notificationUrls.push(newNotificationUrl());
}

function removeNotificationUrl(index) {
  settings.notificationUrls.splice(index, 1);
  if (!settings.notificationUrls.length)
    settings.notificationUrls.push(newNotificationUrl());
}

function onPopState() {
  path.value = window.location.pathname;
  loadPage();
}

onMounted(() => {
  window.addEventListener("popstate", onPopState);
  loadPage();
});
onUnmounted(() => window.removeEventListener("popstate", onPopState));
</script>

<template>
  <v-app>
    <nav v-if="authenticatedPage" class="settings-nav">
      <a class="settings-brand" href="/" @click.prevent="navigate('/')"
        >NixHostForge <span class="settings-version">v{{ appVersion }}</span></a
      >
      <a
        href="/hosts"
        :class="{ active: path === '/hosts' }"
        @click.prevent="navigate('/hosts')"
        >Hosts</a
      >
      <a
        href="/builds"
        :class="{ active: path.startsWith('/builds') }"
        @click.prevent="navigate('/builds')"
        >Builds</a
      >
      <a
        href="/settings"
        :class="{ active: path === '/settings' }"
        @click.prevent="navigate('/settings')"
        >Settings</a
      >
      <a class="settings-logout" href="/logout">Logout</a>
    </nav>

    <v-main>
      <div v-if="path === '/setup'" class="auth-shell">
        <v-card class="settings-card auth-card" rounded="xl">
          <v-card-title class="text-h4">NixHostForge</v-card-title>
          <v-card-subtitle
            >Create the first admin password to protect the web
            interface.</v-card-subtitle
          >
          <v-card-text>
            <v-form @submit.prevent="setup">
              <v-text-field
                v-model="loginForm.password"
                label="Password"
                type="password"
                required
                variant="outlined"
              />
              <v-text-field
                v-model="loginForm.confirm"
                label="Confirm password"
                type="password"
                required
                variant="outlined"
              />
              <v-btn type="submit" color="primary" block :loading="saving.setup"
                >Start forging hosts</v-btn
              >
            </v-form>
          </v-card-text>
        </v-card>
      </div>

      <div v-else-if="path === '/login'" class="auth-shell">
        <v-card class="settings-card auth-card" rounded="xl">
          <v-card-title class="text-h4">NixHostForge</v-card-title>
          <v-card-subtitle>Sign in to manage host prebuilds.</v-card-subtitle>
          <v-card-text>
            <v-form @submit.prevent="login">
              <v-text-field
                v-model="loginForm.password"
                label="Password"
                type="password"
                required
                variant="outlined"
              />
              <v-btn type="submit" color="primary" block :loading="saving.login"
                >Sign in</v-btn
              >
            </v-form>
          </v-card-text>
        </v-card>
      </div>

      <div v-else class="settings-shell">
        <v-alert v-if="loading" class="mb-4" color="primary" variant="tonal"
          >Loading...</v-alert
        >

        <template v-if="path === '/' && dashboard">
          <v-card class="hero-card mb-4" rounded="xl">
            <v-card-text
              class="pa-6 pa-md-8 d-flex flex-column flex-md-row align-md-center justify-space-between ga-4"
            >
              <div>
                <p class="eyebrow mb-2">Prebuild NixOS hosts</p>
                <h1 class="text-h3 text-md-h2 font-weight-bold mb-3">
                  Catch broken host configs before your machines need them.
                </h1>
                <p
                  v-if="dashboard.repository.configured"
                  class="text-medium-emphasis mb-0"
                >
                  Watching
                  <span class="readonly-value">{{
                    dashboard.repository.repository
                  }}</span>
                  on
                  <span class="readonly-value">{{
                    dashboard.repository.branch
                  }}</span
                  >.
                </p>
                <p v-else class="text-medium-emphasis mb-0">
                  No repository configured yet. Add one in Settings to start
                  discovering hosts.
                </p>
              </div>
              <div class="d-flex flex-wrap ga-3">
                <v-btn
                  color="primary"
                  size="large"
                  :loading="saving.check"
                  @click="checkNow"
                  >Check now</v-btn
                >
                <v-btn
                  color="primary"
                  size="large"
                  variant="tonal"
                  :disabled="!dashboard.repository.configured"
                  :loading="saving.buildCurrent"
                  @click="buildCurrentHosts"
                  >Build current commit</v-btn
                >
              </div>
            </v-card-text>
          </v-card>

          <v-alert
            v-if="dashboard.status.lastError"
            class="mb-4"
            color="error"
            variant="tonal"
            >{{ dashboard.status.lastError }}</v-alert
          >

          <v-row class="mb-2">
            <v-col cols="12" md="3"
              ><v-card class="settings-card stat-card" rounded="lg"
                ><v-card-text
                  ><div class="stat-label">Latest commit</div>
                  <div class="stat-value">
                    {{ shortCommit(dashboard.status.lastCommit) }}
                  </div>
                  <div
                    v-if="dashboard.status.lastCommitMessage"
                    class="stat-detail"
                  >
                    {{ dashboard.status.lastCommitMessage }}
                  </div></v-card-text
                ></v-card
              ></v-col
            >
            <v-col cols="12" md="3"
              ><v-card class="settings-card stat-card" rounded="lg"
                ><v-card-text
                  ><div class="stat-label">Running</div>
                  <div class="stat-value">
                    {{ dashboard.status.runningBuilds }}
                  </div></v-card-text
                ></v-card
              ></v-col
            >
            <v-col cols="12" md="3"
              ><v-card class="settings-card stat-card" rounded="lg"
                ><v-card-text
                  ><div class="stat-label">Last check</div>
                  <div class="stat-value">
                    {{ formatDate(dashboard.status.lastCheck) }}
                  </div></v-card-text
                ></v-card
              ></v-col
            >
            <v-col cols="12" md="3"
              ><v-card class="settings-card stat-card" rounded="lg"
                ><v-card-text
                  ><div class="stat-label">Pause</div>
                  <div class="stat-value">
                    {{
                      dashboard.status.pausedUntil
                        ? `until ${formatDate(dashboard.status.pausedUntil)}`
                        : "inactive"
                    }}
                  </div></v-card-text
                ></v-card
              ></v-col
            >
          </v-row>

          <v-card class="settings-card mb-4" rounded="xl">
            <v-card-title>Pause builds</v-card-title>
            <v-card-text class="d-flex flex-wrap align-center ga-3">
              <v-select
                v-model="selectedPauseHours"
                :items="dashboard.pauseHours"
                label="Hours"
                max-width="180"
                variant="outlined"
                hide-details
              />
              <v-btn
                color="warning"
                :loading="saving.pause"
                @click="pauseBuilds"
                >Pause and stop running jobs</v-btn
              >
              <v-btn
                v-if="dashboard.status.pausedUntil"
                variant="tonal"
                :loading="saving.pause"
                @click="resumeBuilds"
                >Resume</v-btn
              >
            </v-card-text>
          </v-card>

          <v-card class="settings-card mb-4" rounded="xl">
            <v-card-title>Hosts</v-card-title>
            <v-card-text
              ><v-row
                ><v-col v-for="host in hosts" :key="host.name" cols="12" md="4"
                  ><v-card class="settings-card host-tile" rounded="lg"
                    ><v-card-text
                      ><div class="text-h6">{{ host.name }}</div>
                      <div class="text-medium-emphasis">
                        Last: {{ host.lastStatus || "no build" }}
                      </div></v-card-text
                    ></v-card
                  ></v-col
                ><v-col v-if="!hosts.length" cols="12"
                  ><p class="text-medium-emphasis">
                    No hosts discovered yet.
                  </p></v-col
                ></v-row
              ></v-card-text
            >
          </v-card>

          <build-table :builds="builds" @navigate="navigate" />
        </template>

        <template v-else-if="path === '/hosts'">
          <page-title
            title="Hosts"
            subtitle="Select which discovered NixOS hosts should be prebuilt."
          />
          <v-card class="settings-card" rounded="xl"
            ><v-list bg-color="transparent"
              ><v-list-item
                v-for="host in hosts"
                :key="host.name"
                class="host-list-item"
                ><template #prepend
                  ><v-switch
                    v-model="host.enabled"
                    color="primary"
                    hide-details
                    :loading="saving.host === host.name"
                    @change="toggleHost(host)" /></template
                ><v-list-item-title>{{ host.name }}</v-list-item-title
                ><v-list-item-subtitle
                  >Last result: {{ host.lastStatus || "none" }}
                  {{
                    host.lastCommit ? `at ${shortCommit(host.lastCommit)}` : ""
                  }}</v-list-item-subtitle
                ><template #append
                  ><v-btn
                    variant="tonal"
                    :loading="saving.build === host.name"
                    @click="buildHost(host)"
                    >Build now</v-btn
                  ></template
                ></v-list-item
              ><v-list-item
                v-if="!hosts.length"
                title="No hosts discovered yet." /></v-list
          ></v-card>
        </template>

        <template v-else-if="path === '/builds'">
          <page-title
            title="Builds"
            subtitle="Recent host prebuild attempts."
          />
          <build-table :builds="builds" @navigate="navigate" />
        </template>

        <template v-else-if="path.startsWith('/builds/') && build">
          <page-title
            :title="`Build #${build.id}`"
            :subtitle="`${build.host} at ${shortCommit(build.commitHash)}`"
          />
          <v-row class="mb-4"
            ><v-col cols="12" md="3"
              ><v-card class="settings-card"
                ><v-card-text
                  ><div class="text-medium-emphasis">Host</div>
                  <div class="text-h6">{{ build.host }}</div></v-card-text
                ></v-card
              ></v-col
            ><v-col cols="12" md="3"
              ><v-card class="settings-card"
                ><v-card-text
                  ><div class="text-medium-emphasis">Status</div>
                  <v-chip :class="`status-${build.status}`">{{
                    build.status
                  }}</v-chip></v-card-text
                ></v-card
              ></v-col
            ><v-col cols="12" md="3"
              ><v-card class="settings-card"
                ><v-card-text
                  ><div class="text-medium-emphasis">Commit</div>
                  <div class="text-h6">
                    {{ shortCommit(build.commitHash) }}
                  </div></v-card-text
                ></v-card
              ></v-col
            ><v-col cols="12" md="3"
              ><v-card class="settings-card"
                ><v-card-text
                  ><div class="text-medium-emphasis">Duration</div>
                  <div class="text-h6">{{ duration(build) }}</div></v-card-text
                ></v-card
              ></v-col
            ></v-row
          >
          <v-card class="settings-card" rounded="xl"
            ><v-card-text
              ><p v-if="build.outputPath">
                Output:
                <span class="readonly-value">{{ build.outputPath }}</span>
              </p>
              <p v-if="build.exitCode !== null">
                Exit code:
                <span class="readonly-value">{{ build.exitCode }}</span>
              </p>
              <pre class="log-block">{{ build.log }}</pre>
            </v-card-text></v-card
          >
        </template>

        <template v-else-if="path === '/settings'">
          <page-title
            title="Settings"
            subtitle="Configure the watched flake, scheduler capacity, and shoutrrr notification delivery."
          />
          <v-row>
            <v-col cols="12" lg="6"
              ><v-card class="settings-card" rounded="xl"
                ><v-card-title>Repository</v-card-title
                ><v-card-subtitle
                  >The flake repository that NixHostForge checks for NixOS
                  hosts.</v-card-subtitle
                ><v-card-text
                  ><v-form
                    v-if="settings.repository.mutable"
                    @submit.prevent="saveRepository"
                    ><v-text-field
                      v-model="settings.repository.repository"
                      label="Repository URL"
                      placeholder="https://github.com/example/nixos-config.git"
                      required
                      variant="outlined"
                    /><v-text-field
                      v-model="settings.repository.branch"
                      label="Branch"
                      placeholder="main"
                      variant="outlined"
                    /><v-btn
                      type="submit"
                      color="primary"
                      :loading="saving.repository"
                      >Save Repository</v-btn
                    ></v-form
                  >
                  <div v-else class="d-flex flex-column ga-3">
                    <div>
                      Repository:
                      <span class="readonly-value">{{
                        settings.repository.repository
                      }}</span>
                    </div>
                    <div>
                      Branch:
                      <span class="readonly-value">{{
                        settings.repository.branch
                      }}</span>
                    </div>
                    <v-alert color="info" variant="tonal"
                      >Configured by static config or the NixOS module.</v-alert
                    >
                  </div></v-card-text
                ></v-card
              ></v-col
            >
            <v-col cols="12" lg="6"
              ><v-card class="settings-card" rounded="xl"
                ><v-card-title>Scheduler</v-card-title
                ><v-card-subtitle
                  >Control how often checks run and how many builds can run in
                  parallel.</v-card-subtitle
                ><v-card-text
                  ><v-form
                    v-if="schedulerMutable"
                    @submit.prevent="saveScheduler"
                    ><v-text-field
                      v-if="settings.scheduler.intervalMutable"
                      v-model="settings.scheduler.interval"
                      label="Interval"
                      placeholder="15m"
                      variant="outlined"
                    />
                    <div v-else class="mb-4">
                      Interval:
                      <span class="readonly-value">{{
                        settings.scheduler.interval
                      }}</span>
                    </div>
                    <v-text-field
                      v-if="settings.scheduler.concurrencyMutable"
                      v-model.number="settings.scheduler.concurrency"
                      label="Concurrency"
                      min="1"
                      max="64"
                      type="number"
                      variant="outlined"
                    />
                    <div v-else class="mb-4">
                      Concurrency:
                      <span class="readonly-value">{{
                        settings.scheduler.concurrency
                      }}</span>
                    </div>
                    <v-btn
                      type="submit"
                      color="primary"
                      :loading="saving.scheduler"
                      >Save Scheduler Settings</v-btn
                    ></v-form
                  >
                  <div v-else class="d-flex flex-column ga-3">
                    <div>
                      Interval:
                      <span class="readonly-value">{{
                        settings.scheduler.interval
                      }}</span>
                    </div>
                    <div>
                      Concurrency:
                      <span class="readonly-value">{{
                        settings.scheduler.concurrency
                      }}</span>
                    </div>
                    <v-alert color="info" variant="tonal"
                      >Configured by static config or the NixOS module.</v-alert
                    >
                  </div></v-card-text
                ></v-card
              ></v-col
            >
            <v-col cols="12">
              <v-card class="settings-card" rounded="xl">
                <v-card-title>Notifications</v-card-title>
                <v-card-subtitle
                  >NixHostForge uses shoutrrr URLs. SMTP, Matrix, Telegram, and
                  other shoutrrr services are supported.</v-card-subtitle
                >
                <v-card-text>
                  <v-form @submit.prevent="saveNotifications">
                    <div
                      v-for="(target, index) in settings.notificationUrls"
                      :key="index"
                      class="d-flex flex-column flex-md-row align-md-center ga-3 mb-3"
                    >
                      <v-switch
                        v-model="target.enabled"
                        color="primary"
                        label="Enabled"
                        hide-details
                        class="notification-enabled"
                      />
                      <v-text-field
                        v-model="target.url"
                        label="Notification URL"
                        placeholder="smtp://user:pass@mail.example.com:587/?from=nix@example.com&to=ops@example.com"
                        variant="outlined"
                        hide-details
                        class="flex-grow-1"
                      />
                      <div class="notification-levels d-flex flex-wrap ga-2">
                        <v-switch
                          v-model="target.success"
                          color="success"
                          label="Success messages"
                          hide-details
                        />
                        <v-switch
                          v-model="target.warnings"
                          color="warning"
                          label="Warnings"
                          hide-details
                        />
                        <v-switch
                          v-model="target.errors"
                          color="error"
                          label="Errors"
                          hide-details
                        />
                      </div>
                      <div class="notification-actions d-flex ga-2">
                        <v-btn
                          type="button"
                          color="secondary"
                          variant="tonal"
                          :loading="saving.test === index + 1"
                          @click="testNotification(index)"
                          >Test</v-btn
                        >
                        <v-btn
                          type="button"
                          icon="mdi-delete"
                          variant="text"
                          color="error"
                          :aria-label="`Remove notification URL ${index + 1}`"
                          @click="removeNotificationUrl(index)"
                        />
                      </div>
                    </div>
                    <div class="d-flex flex-wrap ga-3 mb-5">
                      <v-btn
                        type="button"
                        variant="tonal"
                        @click="addNotificationUrl"
                        >Add URL</v-btn
                      >
                      <v-btn
                        type="submit"
                        color="primary"
                        :loading="saving.notifications"
                        >Save Notifications</v-btn
                      >
                    </div>
                  </v-form>
                  <v-alert color="info" variant="tonal" class="mb-4"
                    >Configure one shoutrrr URL per row. Disabled rows are saved
                    but skipped for notifications. Success messages are sent for
                    successful builds, warnings for cancelled builds, and errors
                    for failed builds. See the
                    <a
                      href="https://containrrr.dev/shoutrrr/v0.8/services/overview/"
                      target="_blank"
                      rel="noreferrer"
                      >shoutrrr service documentation and examples</a
                    >.</v-alert
                  >
                  <div class="example-grid">
                    <div class="example-card">
                      <strong>SMTP</strong
                      ><span class="readonly-value"
                        >smtp://user:pass@mail.example.com:587/?from=nix@example.com&amp;to=ops@example.com</span
                      >
                    </div>
                    <div class="example-card">
                      <strong>Telegram</strong
                      ><span class="readonly-value"
                        >telegram://token@telegram?channels=123456789</span
                      >
                    </div>
                    <div class="example-card">
                      <strong>Matrix</strong
                      ><span class="readonly-value"
                        >matrix://user:pass@matrix.example.com/%23ops:matrix.example.com</span
                      >
                    </div>
                  </div>
                </v-card-text>
              </v-card>
            </v-col>
          </v-row>
        </template>
      </div>
    </v-main>

    <v-snackbar v-model="snackbar.show" :color="snackbar.color" timeout="5000"
      >{{ snackbar.message
      }}<template #actions
        ><v-btn variant="text" @click="snackbar.show = false"
          >Close</v-btn
        ></template
      ></v-snackbar
    >
  </v-app>
</template>
