package app

import (
	"context"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type App struct {
	cfg       Config
	store     *Store
	templates *template.Template

	wake chan struct{}

	slotsMu    sync.Mutex
	slotsCond  *sync.Cond
	activeSlot int

	runningMu sync.Mutex
	running   map[int64]runningBuild

	statusMu sync.Mutex
	status   SchedulerStatus
}

func New(cfg Config) (*App, error) {
	if err := os.MkdirAll(cfg.StateDir, 0750); err != nil {
		return nil, err
	}
	store, err := OpenStore(filepath.Join(cfg.StateDir, "nixhostforge.db"))
	if err != nil {
		return nil, err
	}
	tmpl, err := parseTemplates()
	if err != nil {
		store.Close()
		return nil, err
	}
	app := &App{
		cfg:       cfg,
		store:     store,
		templates: tmpl,
		wake:      make(chan struct{}, 1),
		running:   map[int64]runningBuild{},
	}
	app.slotsCond = sync.NewCond(&app.slotsMu)
	return app, nil
}

func (a *App) Close() error { return a.store.Close() }

func (a *App) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/static/app.css", a.staticCSS)
	mux.HandleFunc("/setup", a.setup)
	mux.HandleFunc("/login", a.login)
	mux.HandleFunc("/logout", a.logout)
	mux.HandleFunc("/", a.requireAuth(a.dashboard))
	mux.HandleFunc("/hosts", a.requireAuth(a.hosts))
	mux.HandleFunc("/hosts/toggle", a.requireAuth(a.toggleHost))
	mux.HandleFunc("/hosts/build", a.requireAuth(a.buildHost))
	mux.HandleFunc("/builds", a.requireAuth(a.builds))
	mux.HandleFunc("/builds/", a.requireAuth(a.buildDetail))
	mux.HandleFunc("/settings", a.requireAuth(a.settings))
	mux.HandleFunc("/pause", a.requireAuth(a.pause))
	mux.HandleFunc("/resume", a.requireAuth(a.resume))
	mux.HandleFunc("/check-now", a.requireAuth(a.checkNow))
	return mux
}

func (a *App) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hasAdmin, err := a.hasAdmin(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !hasAdmin && r.URL.Path != "/setup" {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}
		if hasAdmin && !a.authenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (a *App) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) staticCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = w.Write([]byte(appCSS))
}

func (a *App) TriggerCheck(ctx context.Context) {
	go a.checkOnce(context.Background())
}

func (a *App) signalScheduler() {
	select {
	case a.wake <- struct{}{}:
	default:
	}
}
