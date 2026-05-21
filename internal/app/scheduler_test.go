package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestShouldBuild(t *testing.T) {
	tests := []struct {
		name     string
		previous *Build
		manual   bool
		want     bool
	}{
		{name: "no previous", want: true},
		{name: "success skipped", previous: &Build{Status: "success"}, want: false},
		{name: "failed skipped", previous: &Build{Status: "failed"}, want: false},
		{name: "running skipped", previous: &Build{Status: "running"}, want: false},
		{name: "cancelled retried", previous: &Build{Status: "cancelled"}, want: true},
		{
			name:     "manual retries failure",
			previous: &Build{Status: "failed"},
			manual:   true,
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldBuild(tt.previous, tt.manual); got != tt.want {
				t.Fatalf("ShouldBuild() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLastStorePath(t *testing.T) {
	got := lastStorePath("building\n/nix/store/abc-host\n")
	if got != "/nix/store/abc-host" {
		t.Fatalf("got %q", got)
	}
}

func TestLatestBuildForIsScopedToRepository(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenStore() error = %v", err)
	}
	defer func() { _ = store.Close() }()

	_, err = store.CreateBuildForRepository(
		ctx,
		"host",
		"https://example.test/one.git",
		"main",
		"abc",
		"success",
		false,
	)
	if err != nil {
		t.Fatalf("CreateBuildForRepository() error = %v", err)
	}

	previous, err := store.LatestBuildFor(
		ctx,
		"host",
		"https://example.test/two.git",
		"main",
		"abc",
	)
	if err != nil {
		t.Fatalf("LatestBuildFor() different repository error = %v", err)
	}
	if previous != nil {
		t.Fatalf("LatestBuildFor() different repository = %+v, want nil", previous)
	}

	previous, err = store.LatestBuildFor(ctx, "host", "https://example.test/one.git", "main", "abc")
	if err != nil {
		t.Fatalf("LatestBuildFor() same repository error = %v", err)
	}
	if previous == nil || previous.Status != "success" {
		t.Fatalf("LatestBuildFor() same repository = %+v, want success", previous)
	}
}

func TestEnabledHostDetails(t *testing.T) {
	got := enabledHostDetails([]Host{
		{Name: "disabled", Enabled: false},
		{Name: "enabled", Enabled: true},
	})
	if len(got) != 1 || got[0].Name != "enabled" {
		t.Fatalf("got %v", got)
	}
}

func TestNotificationTargetsFromLegacyLines(t *testing.T) {
	got := notificationTargets(
		"\n smtp://one.example.test \n\ntelegram://token@telegram?channels=1\r\n",
	)
	want := []notificationTarget{
		{URL: "smtp://one.example.test", Enabled: true, Errors: true},
		{URL: "telegram://token@telegram?channels=1", Enabled: true, Errors: true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestNotificationTargetsFromJSON(t *testing.T) {
	got := notificationTargets(
		`[{"url":"smtp://one.example.test","enabled":true},{"url":"telegram://token@telegram?channels=1","enabled":false}]`,
	)
	want := []notificationTarget{
		{URL: "smtp://one.example.test", Enabled: true, Errors: true},
		{URL: "telegram://token@telegram?channels=1", Enabled: false, Errors: true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestNotificationURLsForStatus(t *testing.T) {
	value := `[{"url":"smtp://one.example.test","enabled":true,"success":true,"warnings":false,"errors":true},{"url":"telegram://token@telegram?channels=1","enabled":true,"success":false,"warnings":true,"errors":false},{"url":"matrix://example.test","enabled":false,"success":true,"warnings":true,"errors":true}]`
	tests := []struct {
		status string
		want   []string
	}{
		{status: "success", want: []string{"smtp://one.example.test"}},
		{status: "cancelled", want: []string{"telegram://token@telegram?channels=1"}},
		{status: "failed", want: []string{"smtp://one.example.test"}},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := notificationURLsForStatus(value, tt.status)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestNotificationBuildURL(t *testing.T) {
	if got := notificationBuildURL("https://nixhostforge.example.com/", 42); got != "https://nixhostforge.example.com/builds/42" {
		t.Fatalf("notificationBuildURL() = %q", got)
	}
	if got := notificationBuildURL("", 42); got != "" {
		t.Fatalf("notificationBuildURL() without public URL = %q, want empty", got)
	}
}

func TestSavePublicURLConfig(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenStore() error = %v", err)
	}
	defer func() { _ = store.Close() }()

	app := &App{cfg: DefaultConfig(), store: store}
	if err := app.SavePublicURLConfig(ctx, "https://nixhostforge.example.com/"); err != nil {
		t.Fatalf("SavePublicURLConfig() error = %v", err)
	}
	if got := app.PublicURLConfig(ctx).URL; got != "https://nixhostforge.example.com" {
		t.Fatalf("PublicURLConfig().URL = %q", got)
	}
	if err := app.SavePublicURLConfig(ctx, "ftp://nixhostforge.example.com"); err == nil {
		t.Fatalf("SavePublicURLConfig() accepted non-HTTP URL")
	}
	if err := app.SavePublicURLConfig(ctx, "https://nixhostforge.example.com?token=secret"); err == nil {
		t.Fatalf("SavePublicURLConfig() accepted URL with query string")
	}
}

func TestGitHubCommitURL(t *testing.T) {
	commit := "abcdef1234567890"
	if got := githubCommitURL("git@github.com:owner/repo.git", commit); got != "https://github.com/owner/repo/commit/"+commit {
		t.Fatalf("githubCommitURL() = %q", got)
	}
	if got := githubCommitURL("https://example.com/owner/repo.git", commit); got != "" {
		t.Fatalf("githubCommitURL() for non-GitHub repo = %q, want empty", got)
	}
}

func TestEnabledNotificationURLs(t *testing.T) {
	got := enabledNotificationURLs(
		`[{"url":"smtp://one.example.test","enabled":true},{"url":"telegram://token@telegram?channels=1","enabled":false}]`,
	)
	want := []string{"smtp://one.example.test"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestResumeSignalsScheduler(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenStore() error = %v", err)
	}
	defer func() { _ = store.Close() }()

	app := &App{
		cfg:     DefaultConfig(),
		store:   store,
		wake:    make(chan struct{}, 1),
		running: map[int64]runningBuild{},
	}
	if err := app.Resume(ctx); err != nil {
		t.Fatalf("Resume() error = %v", err)
	}

	select {
	case <-app.wake:
	default:
		t.Fatalf("Resume() did not signal scheduler")
	}
}

func TestEnableHostSignalsScheduler(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenStore() error = %v", err)
	}
	defer func() { _ = store.Close() }()
	if err := store.UpsertHosts(ctx, []string{"host"}); err != nil {
		t.Fatalf("UpsertHosts() error = %v", err)
	}

	app := &App{
		cfg:     DefaultConfig(),
		store:   store,
		wake:    make(chan struct{}, 1),
		running: map[int64]runningBuild{},
	}
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/hosts/toggle",
		bytes.NewBufferString(`{"host":"host","enabled":true}`),
	)
	rr := httptest.NewRecorder()
	app.apiHostsToggle(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("apiHostsToggle() status = %d, want %d", rr.Code, http.StatusOK)
	}

	select {
	case <-app.wake:
	default:
		t.Fatalf("apiHostsToggle() did not signal scheduler")
	}
}

func TestNextCheckDelayUsesPauseExpiry(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenStore() error = %v", err)
	}
	defer func() { _ = store.Close() }()

	interval := time.Hour
	app := &App{
		cfg:   Config{Interval: interval, Concurrency: 1},
		store: store,
	}
	until := time.Now().UTC().Add(5 * time.Minute)
	if err := store.SetSetting(ctx, "pause_until", until.Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("SetSetting() error = %v", err)
	}

	got := app.nextCheckDelay(ctx)
	if got <= 0 || got >= interval {
		t.Fatalf("nextCheckDelay() = %v, want positive delay below %v", got, interval)
	}
}

func TestPendingBuilds(t *testing.T) {
	app := &App{}
	first := app.addPendingBuild("host-a", "repo", "main", "abc", false)
	time.Sleep(time.Millisecond)
	second := app.addPendingBuild("host-b", "repo", "main", "def", true)

	if !app.hasPendingBuild("host-a", "repo", "main", "abc") {
		t.Fatalf("hasPendingBuild() = false, want true")
	}

	builds := app.PendingBuilds()
	if len(builds) != 2 {
		t.Fatalf("PendingBuilds() length = %d, want 2", len(builds))
	}
	if builds[0].ID != first || builds[1].ID != second {
		t.Fatalf("PendingBuilds() = %+v, want queued order", builds)
	}
	if !builds[1].Manual {
		t.Fatalf("manual pending build = false, want true")
	}

	app.removePendingBuild(first)
	if app.hasPendingBuild("host-a", "repo", "main", "abc") {
		t.Fatalf("hasPendingBuild() = true after remove, want false")
	}
}

func TestCancelStaleRunningBuilds(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenStore() error = %v", err)
	}
	defer func() { _ = store.Close() }()

	activeID, err := store.CreateBuild(ctx, "active", "abc", "running", false)
	if err != nil {
		t.Fatalf("CreateBuild() active error = %v", err)
	}
	staleID, err := store.CreateBuild(ctx, "stale", "abc", "running", false)
	if err != nil {
		t.Fatalf("CreateBuild() stale error = %v", err)
	}

	cancelled, err := store.CancelStaleRunningBuilds(
		ctx,
		map[int64]struct{}{activeID: {}},
		"Build cancelled because NixHostForge restarted before this build finished.",
	)
	if err != nil {
		t.Fatalf("CancelStaleRunningBuilds() error = %v", err)
	}
	if cancelled != 1 {
		t.Fatalf("CancelStaleRunningBuilds() = %d, want 1", cancelled)
	}

	active, err := store.Build(ctx, activeID)
	if err != nil {
		t.Fatalf("Build(active) error = %v", err)
	}
	if active.Status != "running" || active.FinishedAt != nil {
		t.Fatalf("active build = %+v, want still running", active)
	}

	stale, err := store.Build(ctx, staleID)
	if err != nil {
		t.Fatalf("Build(stale) error = %v", err)
	}
	if stale.Status != "cancelled" || stale.FinishedAt == nil {
		t.Fatalf("stale build = %+v, want cancelled with finish time", stale)
	}
	if stale.Log == "" {
		t.Fatalf("stale build log is empty")
	}
}
