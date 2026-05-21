package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type settingsResponse struct {
	Repository       settingsRepository   `json:"repository"`
	Scheduler        settingsScheduler    `json:"scheduler"`
	PublicURL        settingsPublicURL    `json:"publicUrl"`
	NotificationURL  string               `json:"notificationUrl"`
	NotificationURLs []notificationTarget `json:"notificationUrls"`
}

type settingsRepository struct {
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	Mutable    bool   `json:"mutable"`
	Configured bool   `json:"configured"`
}

type settingsScheduler struct {
	Interval           string `json:"interval"`
	IntervalMutable    bool   `json:"intervalMutable"`
	Concurrency        int    `json:"concurrency"`
	ConcurrencyMutable bool   `json:"concurrencyMutable"`
}

type settingsPublicURL struct {
	URL     string `json:"url"`
	Mutable bool   `json:"mutable"`
}

type repositoryRequest struct {
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
}

type schedulerRequest struct {
	Interval    string `json:"interval"`
	Concurrency int    `json:"concurrency"`
}

type publicURLRequest struct {
	URL string `json:"url"`
}

type notificationRequest struct {
	NotificationURL  string               `json:"notificationUrl"`
	NotificationURLs []notificationTarget `json:"notificationUrls"`
}

type authRequest struct {
	Password string `json:"password"`
	Confirm  string `json:"confirm"`
}

type authResponse struct {
	HasAdmin      bool `json:"hasAdmin"`
	Authenticated bool `json:"authenticated"`
}

type dashboardResponse struct {
	Repository settingsRepository `json:"repository"`
	Scheduler  settingsScheduler  `json:"scheduler"`
	Status     SchedulerStatus    `json:"status"`
	Hosts      []Host             `json:"hosts"`
	Builds     []Build            `json:"builds"`
	PauseHours []int              `json:"pauseHours"`
}

type buildsResponse struct {
	Builds         []Build        `json:"builds"`
	UpcomingBuilds []PendingBuild `json:"upcomingBuilds"`
}

type hostToggleRequest struct {
	Host    string `json:"host"`
	Enabled bool   `json:"enabled"`
}

type hostBuildRequest struct {
	Host string `json:"host"`
}

type pauseRequest struct {
	Hours int `json:"hours"`
}

type pageData struct {
	Title           string
	Error           string
	Config          Config
	Repo            RepositoryConfig
	Scheduler       SchedulerConfig
	PublicURL       PublicURLConfig
	Status          SchedulerStatus
	Hosts           []Host
	Builds          []Build
	UpcomingBuilds  []PendingBuild
	BuildGroups     []buildGroup
	GroupByHost     bool
	Build           *Build
	NotificationURL string
	PauseHours      []int
}

type buildGroup struct {
	Host   string
	Builds []Build
}

func groupBuildsByHost(builds []Build) []buildGroup {
	byHost := make(map[string][]Build)
	for _, build := range builds {
		host := strings.TrimSpace(build.Host)
		if host == "" {
			host = "unknown"
		}
		byHost[host] = append(byHost[host], build)
	}

	hosts := make([]string, 0, len(byHost))
	for host := range byHost {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)

	groups := make([]buildGroup, 0, len(hosts))
	for _, host := range hosts {
		groups = append(groups, buildGroup{Host: host, Builds: byHost[host]})
	}
	return groups
}

func (a *App) setup(w http.ResponseWriter, r *http.Request) {
	hasAdmin, err := a.hasAdmin(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if r.Method == http.MethodGet {
		if hasAdmin {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		a.settingsApp(w, r)
		return
	}
	if hasAdmin {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodPost {
		password := r.FormValue("password")
		confirm := r.FormValue("confirm")
		if len(password) < 10 {
			a.render(
				w,
				r,
				"setup",
				pageData{Title: "Setup", Error: "Password must be at least 10 characters."},
			)
			return
		}
		if password != confirm {
			a.render(w, r, "setup", pageData{Title: "Setup", Error: "Passwords do not match."})
			return
		}
		if err := a.setAdminPassword(r.Context(), password); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := a.createSession(r.Context(), w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	a.render(w, r, "setup", pageData{Title: "Setup"})
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	hasAdmin, err := a.hasAdmin(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if r.Method == http.MethodGet {
		if !hasAdmin {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}
		a.settingsApp(w, r)
		return
	}
	if !hasAdmin {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodPost {
		ok, err := a.verifyAdminPassword(r.Context(), r.FormValue("password"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !ok {
			a.render(w, r, "login", pageData{Title: "Login", Error: "Invalid password."})
			return
		}
		if err := a.createSession(r.Context(), w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	a.render(w, r, "login", pageData{Title: "Login"})
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	a.destroySession(r.Context(), w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) dashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.settingsApp(w, r)
		return
	}
	hosts, _ := a.store.Hosts(r.Context())
	builds, _ := a.store.Builds(r.Context(), 8)
	a.render(
		w,
		r,
		"dashboard",
		pageData{
			Title:      "Dashboard",
			Config:     a.cfg,
			Repo:       a.RepositoryConfig(r.Context()),
			Scheduler:  a.SchedulerConfig(r.Context()),
			Status:     a.Status(r.Context()),
			Hosts:      enabledHostDetails(hosts),
			Builds:     builds,
			PauseHours: []int{1, 2, 4, 8, 12, 24},
		},
	)
}

func (a *App) hosts(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.settingsApp(w, r)
		return
	}
	hosts, err := a.store.Hosts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.render(
		w,
		r,
		"hosts",
		pageData{Title: "Hosts", Config: a.cfg, Status: a.Status(r.Context()), Hosts: hosts},
	)
}

func (a *App) toggleHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	enabled := r.FormValue("enabled") == "on"
	if err := a.store.SetHostEnabled(r.Context(), r.FormValue("host"), enabled); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if enabled {
		a.signalScheduler()
	}
	http.Redirect(w, r, "/hosts", http.StatusSeeOther)
}

func (a *App) buildHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := a.ManualBuild(r.Context(), r.FormValue("host")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/hosts", http.StatusSeeOther)
}

func (a *App) buildCurrentHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, _, err := a.ManualBuildEnabledHosts(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) builds(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.settingsApp(w, r)
		return
	}
	builds, err := a.store.Builds(r.Context(), 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	groupByHost := r.URL.Query().Get("group") == "host"
	a.render(
		w,
		r,
		"builds",
		pageData{
			Title:          "Builds",
			Config:         a.cfg,
			Status:         a.Status(r.Context()),
			Builds:         builds,
			UpcomingBuilds: a.PendingBuilds(),
			BuildGroups:    groupBuildsByHost(builds),
			GroupByHost:    groupByHost,
		},
	)
}

func (a *App) buildDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.settingsApp(w, r)
		return
	}
	idText := strings.TrimPrefix(r.URL.Path, "/builds/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	build, err := a.store.Build(r.Context(), id)
	if err != nil || build == nil {
		http.NotFound(w, r)
		return
	}
	a.render(
		w,
		r,
		"build",
		pageData{
			Title:  fmt.Sprintf("Build %d", build.ID),
			Config: a.cfg,
			Status: a.Status(r.Context()),
			Build:  build,
		},
	)
}

func (a *App) settings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		switch r.FormValue("section") {
		case "repository":
			if err := a.SaveRepositoryConfig(r.Context(), r.FormValue("repository"), r.FormValue("branch")); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			a.TriggerCheck(r.Context())
		case "scheduler":
			if err := a.SaveSchedulerConfig(r.Context(), r.FormValue("interval"), r.FormValue("concurrency")); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		case "public_url":
			if err := a.SavePublicURLConfig(r.Context(), r.FormValue("public_url")); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		default:
			notificationValue, err := encodeNotificationTargets(
				[]notificationTarget{defaultNotificationTarget(r.FormValue("notification_url"))},
			)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := a.store.SetSetting(r.Context(), "notification_url", notificationValue); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if r.FormValue("action") == "test" {
				if err := a.SendTestNotification(r.Context(), notificationValue); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
		}
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	a.settingsApp(w, r)
}

func (a *App) settingsData(ctx context.Context) settingsResponse {
	repo := a.RepositoryConfig(ctx)
	scheduler := a.SchedulerConfig(ctx)
	publicURL := a.PublicURLConfig(ctx)
	notificationValue, _, _ := a.store.GetSetting(ctx, "notification_url")
	targets := notificationTargets(notificationValue)
	var notificationURLs []string
	for _, target := range targets {
		notificationURLs = append(notificationURLs, target.URL)
	}
	return settingsResponse{
		Repository: settingsRepository(repo),
		Scheduler: settingsScheduler{
			Interval:           scheduler.Interval.String(),
			IntervalMutable:    scheduler.IntervalMutable,
			Concurrency:        scheduler.Concurrency,
			ConcurrencyMutable: scheduler.ConcurrencyMutable,
		},
		PublicURL:        settingsPublicURL(publicURL),
		NotificationURL:  strings.Join(notificationURLs, "\n"),
		NotificationURLs: targets,
	}
}

func (a *App) apiSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, a.settingsData(r.Context()))
}

func (a *App) apiSettingsRepository(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req repositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if err := a.SaveRepositoryConfig(r.Context(), req.Repository, req.Branch); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.TriggerCheck(r.Context())
	writeJSON(w, http.StatusOK, a.settingsData(r.Context()))
}

func (a *App) apiSettingsScheduler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req schedulerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if err := a.SaveSchedulerConfig(r.Context(), req.Interval, strconv.Itoa(req.Concurrency)); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, a.settingsData(r.Context()))
}

func (a *App) apiSettingsPublicURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req publicURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if err := a.SavePublicURLConfig(r.Context(), req.URL); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, a.settingsData(r.Context()))
}

func (a *App) apiSettingsNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req notificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	notificationValue, err := notificationValueFromRequest(req)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := a.store.SetSetting(r.Context(), "notification_url", notificationValue); err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, a.settingsData(r.Context()))
}

func (a *App) apiSettingsNotificationsTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req notificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	notificationValue, err := notificationValueFromRequest(req)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := a.SendTestNotification(r.Context(), notificationValue); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(
		w,
		http.StatusOK,
		map[string]any{
			"message":  "Test notification sent",
			"settings": a.settingsData(r.Context()),
		},
	)
}

func notificationValueFromRequest(req notificationRequest) (string, error) {
	if req.NotificationURLs != nil {
		return encodeNotificationTargets(req.NotificationURLs)
	}
	return encodeNotificationTargets(
		[]notificationTarget{defaultNotificationTarget(req.NotificationURL)},
	)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(w http.ResponseWriter, message string, status int) {
	writeJSON(w, status, map[string]string{"error": message})
}

func (a *App) apiAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	hasAdmin, err := a.hasAdmin(r.Context())
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(
		w,
		http.StatusOK,
		authResponse{HasAdmin: hasAdmin, Authenticated: hasAdmin && a.authenticated(r)},
	)
}

func (a *App) apiSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	hasAdmin, err := a.hasAdmin(r.Context())
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if hasAdmin {
		writeJSONError(w, "admin user already exists", http.StatusBadRequest)
		return
	}
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 10 {
		writeJSONError(w, "password must be at least 10 characters", http.StatusBadRequest)
		return
	}
	if req.Password != req.Confirm {
		writeJSONError(w, "passwords do not match", http.StatusBadRequest)
		return
	}
	if err := a.setAdminPassword(r.Context(), req.Password); err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := a.createSession(r.Context(), w); err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{HasAdmin: true, Authenticated: true})
}

func (a *App) apiLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	ok, err := a.verifyAdminPassword(r.Context(), req.Password)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		writeJSONError(w, "invalid password", http.StatusUnauthorized)
		return
	}
	if err := a.createSession(r.Context(), w); err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{HasAdmin: true, Authenticated: true})
}

func (a *App) dashboardData(ctx context.Context, buildLimit int) (dashboardResponse, error) {
	hosts, err := a.store.Hosts(ctx)
	if err != nil {
		return dashboardResponse{}, err
	}
	builds, err := a.store.Builds(ctx, buildLimit)
	if err != nil {
		return dashboardResponse{}, err
	}
	settings := a.settingsData(ctx)
	return dashboardResponse{
		Repository: settings.Repository,
		Scheduler:  settings.Scheduler,
		Status:     a.Status(ctx),
		Hosts:      enabledHostDetails(hosts),
		Builds:     builds,
		PauseHours: []int{1, 2, 4, 8, 12, 24},
	}, nil
}

func enabledHostDetails(hosts []Host) []Host {
	enabled := make([]Host, 0, len(hosts))
	for _, host := range hosts {
		if host.Enabled {
			enabled = append(enabled, host)
		}
	}
	return enabled
}

func (a *App) apiDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := a.dashboardData(r.Context(), 8)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (a *App) apiHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	hosts, err := a.store.Hosts(r.Context())
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"hosts": hosts})
}

func (a *App) apiHostsToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req hostToggleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if err := a.store.SetHostEnabled(r.Context(), req.Host, req.Enabled); err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if req.Enabled {
		a.signalScheduler()
	}
	hosts, err := a.store.Hosts(r.Context())
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"hosts": hosts})
}

func (a *App) apiHostsBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req hostBuildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if err := a.ManualBuild(r.Context(), req.Host); err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "build started"})
}

func (a *App) apiHostsBuildCurrent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	count, commit, err := a.ManualBuildEnabledHosts(r.Context())
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(
		w,
		http.StatusOK,
		map[string]any{"message": "builds started", "count": count, "commit": commit},
	)
}

func (a *App) apiBuilds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	builds, err := a.store.Builds(r.Context(), 100)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(
		w,
		http.StatusOK,
		buildsResponse{Builds: builds, UpcomingBuilds: a.PendingBuilds()},
	)
}

func (a *App) apiBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idText := strings.TrimPrefix(r.URL.Path, "/api/builds/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		writeJSONError(w, "invalid build id", http.StatusBadRequest)
		return
	}
	build, err := a.store.Build(r.Context(), id)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if build == nil {
		writeJSONError(w, "build not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"build": build})
}

func (a *App) apiCheckNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.CheckNow(r.Context())
	data, err := a.dashboardData(r.Context(), 8)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (a *App) apiPause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req pauseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Hours < 1 || req.Hours > 168 {
		writeJSONError(w, "hours must be between 1 and 168", http.StatusBadRequest)
		return
	}
	if err := a.Pause(r.Context(), time.Duration(req.Hours)*time.Hour); err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": a.Status(r.Context())})
}

func (a *App) apiResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := a.Resume(r.Context()); err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": a.Status(r.Context())})
}

func (a *App) pause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	hours, err := strconv.Atoi(r.FormValue("hours"))
	if err != nil || hours < 1 || hours > 168 {
		http.Error(w, "hours must be between 1 and 168", http.StatusBadRequest)
		return
	}
	if err := a.Pause(r.Context(), time.Duration(hours)*time.Hour); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redirect := r.Header.Get("Referer")
	if redirect == "" {
		redirect = "/"
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func (a *App) resume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := a.Resume(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) checkNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.TriggerCheck(r.Context())
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
