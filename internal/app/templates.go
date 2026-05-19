package app

import "html/template"

func parseTemplates() (*template.Template, error) {
	funcs := template.FuncMap{
		"short": func(s string) string {
			if len(s) > 12 {
				return s[:12]
			}
			return s
		},
	}
	return template.New("root").Funcs(funcs).Parse(templates)
}

const templates = `
{{define "nav"}}
<nav class="nav">
  <a class="brand" href="/">NixHostForge</a>
  <a href="/hosts">Hosts</a>
  <a href="/builds">Builds</a>
  <a href="/settings">Settings</a>
  <a class="logout" href="/logout">Logout</a>
</nav>
{{end}}

{{define "base-start"}}
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}} - NixHostForge</title>
  <link rel="stylesheet" href="/static/app.css">
</head>
<body>
{{template "nav" .}}
<main class="shell">
{{end}}

{{define "base-end"}}
</main>
</body>
</html>
{{end}}

{{define "setup"}}
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Setup - NixHostForge</title><link rel="stylesheet" href="/static/app.css"></head>
<body class="auth"><section class="auth-card"><h1>NixHostForge</h1><p class="muted">Create the first admin password to protect the web interface.</p>{{if .Error}}<p class="error">{{.Error}}</p>{{end}}<form method="post"><label>Password<input name="password" type="password" required autofocus></label><label>Confirm password<input name="confirm" type="password" required></label><button>Start forging hosts</button></form></section></body></html>
{{end}}

{{define "login"}}
<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Login - NixHostForge</title><link rel="stylesheet" href="/static/app.css"></head>
<body class="auth"><section class="auth-card"><h1>NixHostForge</h1><p class="muted">Sign in to manage host prebuilds.</p>{{if .Error}}<p class="error">{{.Error}}</p>{{end}}<form method="post"><label>Password<input name="password" type="password" required autofocus></label><button>Sign in</button></form></section></body></html>
{{end}}

{{define "dashboard"}}
{{template "base-start" .}}
<section class="hero"><div><p class="eyebrow">Prebuild NixOS hosts</p><h1>Catch broken host configs before your machines need them.</h1>{{if .Repo.Configured}}<p>Watching <code>{{.Repo.Repository}}</code> on <code>{{.Repo.Branch}}</code>.</p>{{else}}<p>No repository configured yet. Add one in Settings to start discovering hosts.</p>{{end}}</div><div class="inline"><form method="post" action="/check-now"><button>Check now</button></form><form method="post" action="/hosts/build-current"><button class="secondary" {{if not .Repo.Configured}}disabled{{end}}>Build current commit</button></form></div></section>
<section class="grid stats"><article><span>Latest commit</span><strong>{{if .Status.LastCommit}}{{short .Status.LastCommit}}{{else}}unknown{{end}}</strong>{{if .Status.LastCommitMessage}}<p>{{.Status.LastCommitMessage}}</p>{{end}}</article><article><span>Running</span><strong>{{.Status.RunningBuilds}}</strong>{{if .Status.StaleRunningBuilds}}<p class="error">{{.Status.StaleRunningBuilds}} stale running build(s)</p>{{end}}</article><article><span>Last check</span><strong>{{if .Status.LastCheck.IsZero}}never{{else}}{{.Status.LastCheck.Format "15:04:05"}}{{end}}</strong></article><article><span>Pause</span><strong>{{if .Status.PausedUntil}}until {{.Status.PausedUntil.Format "Jan 02 15:04"}}{{else}}inactive{{end}}</strong></article></section>
{{if .Status.LastError}}<p class="error">{{.Status.LastError}}</p>{{end}}
{{if .Status.StaleRunningBuilds}}<p class="error">Some builds are marked running in the database but have no active job. Restart NixHostForge to reconcile them automatically.</p>{{end}}
{{if not .Repo.Configured}}<section class="panel"><h2>Repository setup</h2><p class="muted">Set the flake repository to watch. This is available because no repository was provided by static config or the NixOS module.</p><form method="post" action="/settings"><input type="hidden" name="section" value="repository"><label>Repository URL<input name="repository" placeholder="https://github.com/example/nixos-config.git" required></label><label>Branch<input name="branch" value="{{.Repo.Branch}}" placeholder="main"></label><button>Save repository and check now</button></form></section>{{end}}
<section class="panel"><div class="panel-head"><h2>Pause builds</h2>{{if .Status.PausedUntil}}<form method="post" action="/resume"><button class="secondary">Resume</button></form>{{end}}</div><form class="inline" method="post" action="/pause"><select name="hours">{{range .PauseHours}}<option value="{{.}}">{{.}} hours</option>{{end}}</select><button>Pause and stop running jobs</button></form></section>
<section class="panel"><h2>Hosts</h2><div class="host-grid">{{range .Hosts}}<article class="host-card {{.LastStatus}}"><h3>{{.Name}}</h3><p>enabled</p><p>Last: {{if .LastStatus}}{{.LastStatus}} at {{.LastBuildAt.Format "Jan 02 15:04"}}{{else}}no build{{end}}</p></article>{{else}}<p class="muted">No enabled hosts to show.</p>{{end}}</div></section>
<section class="panel"><h2>Recent builds</h2>{{template "build-table" .}}</section>
{{template "base-end" .}}
{{end}}

{{define "hosts"}}
{{template "base-start" .}}
<section class="panel"><h1>Hosts</h1><p class="muted">Select which discovered NixOS hosts should be prebuilt.</p><div class="host-list">{{range .Hosts}}<article><div><h3>{{.Name}}</h3><p>Last result: {{if .LastStatus}}{{.LastStatus}} at {{short .LastCommit}}{{else}}none{{end}}</p></div><form method="post" action="/hosts/toggle"><input type="hidden" name="host" value="{{.Name}}"><label class="switch"><input type="checkbox" name="enabled" {{if .Enabled}}checked{{end}} onchange="this.form.submit()"><span></span></label></form><form method="post" action="/hosts/build"><input type="hidden" name="host" value="{{.Name}}"><button class="secondary">Build now</button></form></article>{{else}}<p class="muted">No hosts discovered yet.</p>{{end}}</div></section>
{{template "base-end" .}}
{{end}}

{{define "builds"}}
{{template "base-start" .}}
<section class="panel"><h1>Builds</h1>{{template "build-table" .}}</section>
{{template "base-end" .}}
{{end}}

{{define "build-table"}}
<div class="table-wrap"><table><thead><tr><th>ID</th><th>Host</th><th>Commit</th><th>Status</th><th>Started</th><th>Duration</th></tr></thead><tbody>{{range .Builds}}<tr><td><a href="/builds/{{.ID}}">#{{.ID}}</a></td><td>{{.Host}}</td><td><code>{{.ShortCommit}}</code></td><td><span class="badge {{.Status}}">{{.Status}}</span></td><td>{{.StartedAt.Format "Jan 02 15:04"}}</td><td>{{.Duration}}</td></tr>{{else}}<tr><td colspan="6">No builds yet.</td></tr>{{end}}</tbody></table></div>
{{end}}

{{define "build"}}
{{template "base-start" .}}
<section class="panel"><h1>Build #{{.Build.ID}}</h1><div class="grid stats"><article><span>Host</span><strong>{{.Build.Host}}</strong></article><article><span>Status</span><strong>{{.Build.Status}}</strong></article><article><span>Commit</span><strong>{{.Build.ShortCommit}}</strong></article><article><span>Duration</span><strong>{{.Build.Duration}}</strong></article></div>{{if .Build.OutputPath}}<p>Output: <code>{{.Build.OutputPath}}</code></p>{{end}}{{if .Build.ExitCode}}<p>Exit code: <code>{{.Build.ExitCodeString}}</code></p>{{end}}<pre class="log">{{.Build.Log}}</pre></section>
{{template "base-end" .}}
{{end}}

{{define "settings"}}
{{template "base-start" .}}
<section class="panel"><h1>Settings</h1><h2>Repository</h2>{{if .Repo.Mutable}}<p class="muted">No repository was provided by static config, so this instance can be configured from the web UI.</p><form method="post"><input type="hidden" name="section" value="repository"><label>Repository URL<input name="repository" value="{{.Repo.Repository}}" placeholder="https://github.com/example/nixos-config.git" required></label><label>Branch<input name="branch" value="{{.Repo.Branch}}" placeholder="main"></label><button>Save repository</button></form>{{else}}<p>Repository: <code>{{.Repo.Repository}}</code></p><p>Branch: <code>{{.Repo.Branch}}</code></p><p class="muted">Configured by static config or the NixOS module.</p>{{end}}</section><section class="panel"><h2>Scheduler</h2>{{if or .Scheduler.IntervalMutable .Scheduler.ConcurrencyMutable}}<p class="muted">Settings not provided by static config can be changed here.</p><form method="post"><input type="hidden" name="section" value="scheduler"><label>Interval{{if .Scheduler.IntervalMutable}}<input name="interval" value="{{.Scheduler.Interval}}" placeholder="15m">{{else}}<code>{{.Scheduler.Interval}}</code><input type="hidden" name="interval" value="{{.Scheduler.Interval}}">{{end}}</label><label>Concurrency{{if .Scheduler.ConcurrencyMutable}}<input name="concurrency" type="number" min="1" max="64" value="{{.Scheduler.Concurrency}}">{{else}}<code>{{.Scheduler.Concurrency}}</code><input type="hidden" name="concurrency" value="{{.Scheduler.Concurrency}}">{{end}}</label><button>Save scheduler settings</button></form>{{else}}<p>Interval: <code>{{.Scheduler.Interval}}</code></p><p>Concurrency: <code>{{.Scheduler.Concurrency}}</code></p><p class="muted">Configured by static config or the NixOS module.</p>{{end}}</section><section class="panel"><h2>Notifications</h2><p class="muted">NixHostForge uses shoutrrr URLs. SMTP, Matrix, and Telegram are supported by shoutrrr.</p><form method="post"><input type="hidden" name="section" value="notifications"><label>Notification URL<input name="notification_url" value="{{.NotificationURL}}" placeholder="telegram://TOKEN@telegram?channels=CHAT_ID"></label><div class="inline"><button name="action" value="save">Save notification settings</button><button class="secondary" name="action" value="test">Test</button></div></form><div class="examples"><p><code>smtp://user:pass@mail.example.com:587/?from=nixhostforge@example.com&to=admin@example.com</code></p><p><code>telegram://TOKEN@telegram?channels=CHAT_ID</code></p><p><code>matrix://user:pass@matrix.example.com:8448/?rooms=!roomid:matrix.example.com</code></p></div></section><section class="panel"><h2>Service config</h2><p>Listen address: <code>{{.Config.ListenAddress}}</code></p><p>Port: <code>{{.Config.Port}}</code></p><p>State directory: <code>{{.Config.StateDir}}</code></p></section>
{{template "base-end" .}}
{{end}}
`
