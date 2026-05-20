package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/containrrr/shoutrrr"
)

type runningBuild struct {
	cancel context.CancelFunc
	id     int64
}

type SchedulerStatus struct {
	LastCommit         string     `json:"lastCommit"`
	LastCommitMessage  string     `json:"lastCommitMessage"`
	LastCheck          time.Time  `json:"lastCheck"`
	LastError          string     `json:"lastError"`
	RunningBuilds      int        `json:"runningBuilds"`
	StaleRunningBuilds int        `json:"staleRunningBuilds"`
	PausedUntil        *time.Time `json:"pausedUntil"`
}

type notificationTarget struct {
	URL      string `json:"url"`
	Enabled  bool   `json:"enabled"`
	Success  bool   `json:"success"`
	Warnings bool   `json:"warnings"`
	Errors   bool   `json:"errors"`
}

func ShouldBuild(previous *Build, manual bool) bool {
	if manual {
		return true
	}
	if previous == nil {
		return true
	}
	return previous.Status == "cancelled"
}

func (a *App) Run(ctx context.Context) {
	for {
		a.checkOnce(ctx)
		timer := time.NewTimer(a.nextCheckDelay(ctx))
		select {
		case <-ctx.Done():
			stopTimer(timer)
			a.CancelRunning("service shutting down")
			return
		case <-a.wake:
			stopTimer(timer)
		case <-timer.C:
		}
	}
}

func (a *App) nextCheckDelay(ctx context.Context) time.Duration {
	interval := a.SchedulerConfig(ctx).Interval
	if paused, until := a.paused(ctx); paused {
		remaining := time.Until(until)
		if remaining < interval {
			return remaining
		}
	}
	return interval
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func (a *App) checkOnce(ctx context.Context) {
	if paused, until := a.paused(ctx); paused {
		a.setStatus(
			"",
			"",
			time.Now().UTC(),
			fmt.Sprintf("paused until %s", until.Format(time.RFC3339)),
		)
		return
	}
	repoDir, err := a.ensureRepo(ctx)
	if err != nil {
		a.setStatus("", "", time.Now().UTC(), err.Error())
		log.Printf("repo check failed: %v", err)
		return
	}
	commit, err := a.currentCommit(ctx, repoDir)
	if err != nil {
		a.setStatus("", "", time.Now().UTC(), err.Error())
		log.Printf("commit check failed: %v", err)
		return
	}
	commitMessage, err := a.currentCommitMessage(ctx, repoDir)
	if err != nil {
		a.setStatus(commit, "", time.Now().UTC(), err.Error())
		log.Printf("commit message check failed: %v", err)
		return
	}
	hosts, err := a.discoverHosts(ctx, repoDir)
	if err != nil {
		a.setStatus(commit, commitMessage, time.Now().UTC(), err.Error())
		log.Printf("host discovery failed: %v", err)
		return
	}
	if err := a.store.UpsertHosts(ctx, hosts); err != nil {
		a.setStatus(commit, commitMessage, time.Now().UTC(), err.Error())
		return
	}
	a.setStatus(commit, commitMessage, time.Now().UTC(), "")
	repoConfig := a.RepositoryConfig(ctx)

	enabled, err := a.store.EnabledHosts(ctx)
	if err != nil {
		log.Printf("load enabled hosts: %v", err)
		return
	}
	for _, host := range enabled {
		if ctx.Err() != nil {
			return
		}
		if paused, _ := a.paused(ctx); paused {
			return
		}
		previous, err := a.store.LatestBuildFor(
			ctx,
			host,
			repoConfig.Repository,
			repoConfig.Branch,
			commit,
		)
		if err != nil {
			log.Printf("load previous build for %s: %v", host, err)
			continue
		}
		if !ShouldBuild(previous, false) {
			continue
		}
		go a.runBuild(
			context.Background(),
			repoDir,
			repoConfig.Repository,
			repoConfig.Branch,
			host,
			commit,
			false,
		)
	}
}

func (a *App) runBuild(
	ctx context.Context,
	repoDir, repository, branch, host, commit string,
	manual bool,
) {
	a.acquireBuildSlot()
	defer a.releaseBuildSlot()

	if paused, _ := a.paused(ctx); paused && !manual {
		return
	}

	id, err := a.store.CreateBuildForRepository(
		ctx,
		host,
		repository,
		branch,
		commit,
		"running",
		manual,
	)
	if err != nil {
		log.Printf("create build: %v", err)
		return
	}

	buildCtx, cancel := context.WithCancel(ctx)
	a.runningMu.Lock()
	a.running[id] = runningBuild{cancel: cancel, id: id}
	a.runningMu.Unlock()
	defer func() {
		a.runningMu.Lock()
		delete(a.running, id)
		a.runningMu.Unlock()
		cancel()
	}()

	attr := fmt.Sprintf(".#nixosConfigurations.%s.config.system.build.toplevel", host)
	cmd := exec.CommandContext(
		buildCtx,
		"nix",
		"--extra-experimental-features",
		"nix-command",
		"--extra-experimental-features",
		"flakes",
		"build",
		"--print-out-paths",
		attr,
	)
	cmd.Dir = repoDir
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err = cmd.Run()

	logText := output.String()
	outputPath := lastStorePath(logText)
	status := "success"
	var exitCode *int
	if err != nil {
		status = "failed"
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code := exitErr.ExitCode()
			exitCode = &code
		} else if buildCtx.Err() != nil {
			status = "cancelled"
			logText += "\nBuild cancelled by NixHostForge.\n"
		} else {
			logText += "\n" + err.Error() + "\n"
		}
	}
	if err := a.store.FinishBuild(context.Background(), id, status, exitCode, outputPath, logText); err != nil {
		log.Printf("finish build: %v", err)
	}
	switch status {
	case "success":
		a.notifyBuildResult(context.Background(), id, host, commit, status)
	case "cancelled":
		a.notifyBuildResult(context.Background(), id, host, commit, status)
	case "failed":
		a.notifyBuildResult(context.Background(), id, host, commit, status)
	}
}

func (a *App) ManualBuild(ctx context.Context, host string) error {
	repoDir, err := a.ensureRepo(ctx)
	if err != nil {
		return err
	}
	commit, err := a.currentCommit(ctx, repoDir)
	if err != nil {
		return err
	}
	repoConfig := a.RepositoryConfig(ctx)
	go a.runBuild(
		context.Background(),
		repoDir,
		repoConfig.Repository,
		repoConfig.Branch,
		host,
		commit,
		true,
	)
	return nil
}

func (a *App) ManualBuildEnabledHosts(ctx context.Context) (int, string, error) {
	repoDir, err := a.ensureRepo(ctx)
	if err != nil {
		return 0, "", err
	}
	commit, err := a.currentCommit(ctx, repoDir)
	if err != nil {
		return 0, "", err
	}
	commitMessage, err := a.currentCommitMessage(ctx, repoDir)
	if err != nil {
		return 0, commit, err
	}
	hosts, err := a.discoverHosts(ctx, repoDir)
	if err != nil {
		return 0, commit, err
	}
	if err := a.store.UpsertHosts(ctx, hosts); err != nil {
		return 0, commit, err
	}
	a.setStatus(commit, commitMessage, time.Now().UTC(), "")
	repoConfig := a.RepositoryConfig(ctx)
	enabled, err := a.store.EnabledHosts(ctx)
	if err != nil {
		return 0, commit, err
	}
	for _, host := range enabled {
		go a.runBuild(
			context.Background(),
			repoDir,
			repoConfig.Repository,
			repoConfig.Branch,
			host,
			commit,
			true,
		)
	}
	return len(enabled), commit, nil
}

func (a *App) CancelRunning(reason string) {
	a.runningMu.Lock()
	running := make([]runningBuild, 0, len(a.running))
	for _, build := range a.running {
		running = append(running, build)
	}
	a.runningMu.Unlock()
	for _, build := range running {
		build.cancel()
	}
}

func (a *App) cancelStaleRunningBuilds(ctx context.Context, message string) error {
	cancelled, err := a.store.CancelStaleRunningBuilds(ctx, a.activeBuildIDs(), message)
	if err != nil {
		return err
	}
	if cancelled > 0 {
		log.Printf("cancelled %d stale running build(s): %s", cancelled, message)
	}
	return nil
}

func (a *App) activeBuildIDs() map[int64]struct{} {
	a.runningMu.Lock()
	defer a.runningMu.Unlock()
	ids := make(map[int64]struct{}, len(a.running))
	for id := range a.running {
		ids[id] = struct{}{}
	}
	return ids
}

func (a *App) staleRunningBuilds(ctx context.Context) int {
	running, err := a.store.RunningBuilds(ctx)
	if err != nil {
		log.Printf("load running builds: %v", err)
		return 0
	}
	activeIDs := a.activeBuildIDs()
	stale := 0
	for _, build := range running {
		if _, ok := activeIDs[build.ID]; !ok {
			stale++
		}
	}
	return stale
}

func (a *App) acquireBuildSlot() {
	a.slotsMu.Lock()
	defer a.slotsMu.Unlock()
	for {
		limit := a.SchedulerConfig(context.Background()).Concurrency
		if a.activeSlot < limit {
			a.activeSlot++
			return
		}
		a.slotsCond.Wait()
	}
}

func (a *App) releaseBuildSlot() {
	a.slotsMu.Lock()
	if a.activeSlot > 0 {
		a.activeSlot--
	}
	a.slotsCond.Broadcast()
	a.slotsMu.Unlock()
}

func (a *App) Pause(ctx context.Context, d time.Duration) error {
	until := time.Now().UTC().Add(d)
	if err := a.store.SetSetting(ctx, "pause_until", until.Format(time.RFC3339Nano)); err != nil {
		return err
	}
	a.CancelRunning("paused")
	return nil
}

func (a *App) Resume(ctx context.Context) error {
	if err := a.store.DeleteSetting(ctx, "pause_until"); err != nil {
		return err
	}
	a.signalScheduler()
	return nil
}

func (a *App) paused(ctx context.Context) (bool, time.Time) {
	value, ok, err := a.store.GetSetting(ctx, "pause_until")
	if err != nil || !ok {
		return false, time.Time{}
	}
	until, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return false, time.Time{}
	}
	if time.Now().UTC().Before(until) {
		return true, until
	}
	_ = a.store.DeleteSetting(ctx, "pause_until")
	return false, time.Time{}
}

func (a *App) setStatus(commit, commitMessage string, checked time.Time, lastErr string) {
	a.statusMu.Lock()
	defer a.statusMu.Unlock()
	if commit != "" {
		a.status.LastCommit = commit
		a.status.LastCommitMessage = commitMessage
	}
	a.status.LastCheck = checked
	a.status.LastError = lastErr
}

func (a *App) Status(ctx context.Context) SchedulerStatus {
	a.statusMu.Lock()
	status := a.status
	a.statusMu.Unlock()
	a.runningMu.Lock()
	status.RunningBuilds = len(a.running)
	a.runningMu.Unlock()
	status.StaleRunningBuilds = a.staleRunningBuilds(ctx)
	if paused, until := a.paused(ctx); paused {
		status.PausedUntil = &until
	}
	return status
}

func (a *App) notifyBuildResult(ctx context.Context, id int64, host, commit, status string) {
	value, ok, err := a.store.GetSetting(ctx, "notification_url")
	urls := notificationURLsForStatus(value, status)
	if err != nil || !ok || len(urls) == 0 {
		return
	}
	shortCommit := commit
	if len(shortCommit) > 12 {
		shortCommit = shortCommit[:12]
	}
	repoConfig := a.RepositoryConfig(ctx)
	title := map[string]string{
		"success":   "NixHostForge build succeeded",
		"cancelled": "NixHostForge build cancelled",
		"failed":    "NixHostForge build failed",
	}[status]
	if title == "" {
		title = "NixHostForge build update"
	}
	message := fmt.Sprintf(
		"%s\n\nHost: %s\nCommit: %s\nRepository: %s\n\nOpen NixHostForge for the full build log.",
		title,
		host,
		shortCommit,
		repoConfig.Repository,
	)
	sent := false
	for i, url := range urls {
		if err := shoutrrr.Send(url, message); err != nil {
			log.Printf("send notification to URL %d: %v", i+1, err)
			continue
		}
		sent = true
	}
	if sent {
		_ = a.store.MarkNotificationSent(ctx, id)
	}
}

func notificationTargets(value string) []notificationTarget {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	if strings.HasPrefix(value, "[") {
		var raw []struct {
			URL      string `json:"url"`
			Enabled  *bool  `json:"enabled"`
			Success  *bool  `json:"success"`
			Warnings *bool  `json:"warnings"`
			Errors   *bool  `json:"errors"`
		}
		if err := json.Unmarshal([]byte(value), &raw); err == nil {
			var targets []notificationTarget
			for _, target := range raw {
				url := strings.TrimSpace(target.URL)
				if url == "" {
					continue
				}
				enabled := true
				if target.Enabled != nil {
					enabled = *target.Enabled
				}
				targets = append(
					targets,
					notificationTarget{
						URL:      url,
						Enabled:  enabled,
						Success:  boolValue(target.Success, false),
						Warnings: boolValue(target.Warnings, false),
						Errors:   boolValue(target.Errors, true),
					},
				)
			}
			return targets
		}
	}

	var targets []notificationTarget
	for _, line := range strings.FieldsFunc(value, func(r rune) bool { return r == '\n' || r == '\r' }) {
		url := strings.TrimSpace(line)
		if url != "" {
			targets = append(targets, defaultNotificationTarget(url))
		}
	}
	return targets
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func defaultNotificationTarget(url string) notificationTarget {
	return notificationTarget{URL: url, Enabled: true, Errors: true}
}

func normalizeNotificationTargets(targets []notificationTarget) []notificationTarget {
	var normalized []notificationTarget
	for _, target := range targets {
		url := strings.TrimSpace(target.URL)
		if url != "" {
			normalized = append(normalized, notificationTarget{
				URL:      url,
				Enabled:  target.Enabled,
				Success:  target.Success,
				Warnings: target.Warnings,
				Errors:   target.Errors,
			})
		}
	}
	return normalized
}

func encodeNotificationTargets(targets []notificationTarget) (string, error) {
	targets = normalizeNotificationTargets(targets)
	if len(targets) == 0 {
		return "", nil
	}
	data, err := json.Marshal(targets)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func enabledNotificationURLs(value string) []string {
	var urls []string
	for _, target := range notificationTargets(value) {
		if target.Enabled {
			urls = append(urls, target.URL)
		}
	}
	return urls
}

func notificationURLsForStatus(value, status string) []string {
	var urls []string
	for _, target := range notificationTargets(value) {
		if !target.Enabled {
			continue
		}
		if status == "success" && target.Success {
			urls = append(urls, target.URL)
		}
		if status == "cancelled" && target.Warnings {
			urls = append(urls, target.URL)
		}
		if status == "failed" && target.Errors {
			urls = append(urls, target.URL)
		}
	}
	return urls
}

func (a *App) SendTestNotification(ctx context.Context, value string) error {
	urls := enabledNotificationURLs(value)
	if len(urls) == 0 {
		return errors.New("at least one enabled notification URL must be configured")
	}
	repoConfig := a.RepositoryConfig(ctx)
	repository := repoConfig.Repository
	if repository == "" {
		repository = "not configured"
	}
	message := fmt.Sprintf("NixHostForge test notification\n\nRepository: %s", repository)
	var failed []string
	for i, url := range urls {
		if err := shoutrrr.Send(url, message); err != nil {
			failed = append(failed, fmt.Sprintf("URL %d: %v", i+1, err))
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("send notification: %s", strings.Join(failed, "; "))
	}
	return nil
}

func lastStorePath(output string) string {
	fields := strings.Fields(output)
	for i := len(fields) - 1; i >= 0; i-- {
		if strings.HasPrefix(fields[i], "/nix/store/") {
			return filepath.Clean(fields[i])
		}
	}
	return ""
}
