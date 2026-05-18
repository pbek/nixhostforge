package app

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type pageData struct {
	Title           string
	Error           string
	Config          Config
	Repo            RepositoryConfig
	Scheduler       SchedulerConfig
	Status          SchedulerStatus
	Hosts           []Host
	Builds          []Build
	Build           *Build
	NotificationURL string
	PauseHours      []int
}

func (a *App) setup(w http.ResponseWriter, r *http.Request) {
	hasAdmin, err := a.hasAdmin(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
			a.render(w, r, "setup", pageData{Title: "Setup", Error: "Password must be at least 10 characters."})
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
	hosts, _ := a.store.Hosts(r.Context())
	builds, _ := a.store.Builds(r.Context(), 8)
	a.render(w, r, "dashboard", pageData{Title: "Dashboard", Config: a.cfg, Repo: a.RepositoryConfig(r.Context()), Scheduler: a.SchedulerConfig(r.Context()), Status: a.Status(r.Context()), Hosts: hosts, Builds: builds, PauseHours: []int{1, 2, 4, 8, 12, 24}})
}

func (a *App) hosts(w http.ResponseWriter, r *http.Request) {
	hosts, err := a.store.Hosts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.render(w, r, "hosts", pageData{Title: "Hosts", Config: a.cfg, Status: a.Status(r.Context()), Hosts: hosts})
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

func (a *App) builds(w http.ResponseWriter, r *http.Request) {
	builds, err := a.store.Builds(r.Context(), 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.render(w, r, "builds", pageData{Title: "Builds", Config: a.cfg, Status: a.Status(r.Context()), Builds: builds})
}

func (a *App) buildDetail(w http.ResponseWriter, r *http.Request) {
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
	a.render(w, r, "build", pageData{Title: fmt.Sprintf("Build %d", build.ID), Config: a.cfg, Status: a.Status(r.Context()), Build: build})
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
		default:
			notificationURL := strings.TrimSpace(r.FormValue("notification_url"))
			if err := a.store.SetSetting(r.Context(), "notification_url", notificationURL); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if r.FormValue("action") == "test" {
				if err := a.SendTestNotification(r.Context(), notificationURL); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
		}
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	notificationURL, _, _ := a.store.GetSetting(r.Context(), "notification_url")
	a.render(w, r, "settings", pageData{Title: "Settings", Config: a.cfg, Repo: a.RepositoryConfig(r.Context()), Scheduler: a.SchedulerConfig(r.Context()), Status: a.Status(r.Context()), NotificationURL: notificationURL, PauseHours: []int{1, 2, 4, 8, 12, 24}})
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
