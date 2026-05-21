//go:build !embed_settings

package app

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (a *App) settingsApp(w http.ResponseWriter, r *http.Request) {
	data := pageData{
		Config:     a.cfg,
		Status:     a.Status(r.Context()),
		Repo:       a.RepositoryConfig(r.Context()),
		Scheduler:  a.SchedulerConfig(r.Context()),
		PauseHours: []int{1, 2, 4, 8, 12, 24},
	}

	switch {
	case r.URL.Path == "/setup":
		data.Title = "Setup"
		a.render(w, r, "setup", data)
	case r.URL.Path == "/login":
		data.Title = "Login"
		a.render(w, r, "login", data)
	case r.URL.Path == "/settings":
		data.Title = "Settings"
		data.NotificationURL, _, _ = a.store.GetSetting(r.Context(), "notification_url")
		a.render(w, r, "settings", data)
	case r.URL.Path == "/hosts":
		hosts, err := a.store.Hosts(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data.Title = "Hosts"
		data.Hosts = hosts
		a.render(w, r, "hosts", data)
	case r.URL.Path == "/builds":
		builds, err := a.store.Builds(r.Context(), 100)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data.Title = "Builds"
		data.Builds = builds
		data.GroupByHost = r.URL.Query().Get("group") == "host"
		if data.GroupByHost {
			data.BuildGroups = groupBuildsByHost(builds)
		}
		a.render(w, r, "builds", data)
	case strings.HasPrefix(r.URL.Path, "/builds/"):
		id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/builds/"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		build, err := a.store.Build(r.Context(), id)
		if err != nil || build == nil {
			http.NotFound(w, r)
			return
		}
		data.Title = fmt.Sprintf("Build %d", build.ID)
		data.Build = build
		a.render(w, r, "build", data)
	default:
		hosts, _ := a.store.Hosts(r.Context())
		builds, _ := a.store.Builds(r.Context(), 8)
		data.Title = "Dashboard"
		data.Hosts = enabledHostDetails(hosts)
		data.Builds = builds
		a.render(w, r, "dashboard", data)
	}
}

func (a *App) settingsAsset(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
