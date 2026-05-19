//go:build embed_settings

package app

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed settings/dist
var settingsDist embed.FS

func (a *App) settingsApp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFileFS(w, r, settingsDist, "settings/dist/index.html")
}

func (a *App) settingsAsset(w http.ResponseWriter, r *http.Request) {
	assets, err := fs.Sub(settingsDist, "settings/dist")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.StripPrefix("/settings-static/", http.FileServerFS(assets)).ServeHTTP(w, r)
}
