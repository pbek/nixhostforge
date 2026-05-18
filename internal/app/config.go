package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Repository            string
	Branch                string
	Interval              time.Duration
	IntervalConfigured    bool
	ListenAddress         string
	Port                  int
	StateDir              string
	Concurrency           int
	ConcurrencyConfigured bool
}

type RepositoryConfig struct {
	Repository string
	Branch     string
	Mutable    bool
	Configured bool
}

type SchedulerConfig struct {
	Interval           time.Duration
	IntervalMutable    bool
	Concurrency        int
	ConcurrencyMutable bool
}

func DefaultConfig() Config {
	return Config{
		Branch:        "main",
		Interval:      15 * time.Minute,
		ListenAddress: "0.0.0.0",
		Port:          9637,
		StateDir:      "/var/lib/nixhostforge",
		Concurrency:   1,
	}
}

func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		if env := os.Getenv("NIXHOSTFORGE_CONFIG"); env != "" {
			path = env
		}
	}
	if path == "" {
		return cfg, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return cfg, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return cfg, fmt.Errorf("%s:%d: expected key = value", path, lineNo)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"`)

		switch key {
		case "repository":
			cfg.Repository = value
		case "branch":
			cfg.Branch = value
		case "interval":
			d, err := time.ParseDuration(value)
			if err != nil {
				return cfg, fmt.Errorf("parse interval: %w", err)
			}
			cfg.Interval = d
			cfg.IntervalConfigured = true
		case "listen_address":
			cfg.ListenAddress = value
		case "port":
			port, err := strconv.Atoi(value)
			if err != nil {
				return cfg, fmt.Errorf("parse port: %w", err)
			}
			cfg.Port = port
		case "state_dir":
			cfg.StateDir = value
		case "concurrency":
			concurrency, err := strconv.Atoi(value)
			if err != nil {
				return cfg, fmt.Errorf("parse concurrency: %w", err)
			}
			cfg.Concurrency = concurrency
			cfg.ConcurrencyConfigured = true
		}
	}
	if err := scanner.Err(); err != nil {
		return cfg, err
	}

	if cfg.Concurrency < 1 {
		cfg.Concurrency = 1
	}
	if cfg.StateDir == "" {
		return cfg, errors.New("state_dir must not be empty")
	}
	cfg.StateDir = filepath.Clean(cfg.StateDir)
	return cfg, nil
}

func (a *App) SchedulerConfig(ctx context.Context) SchedulerConfig {
	interval := a.cfg.Interval
	intervalMutable := !a.cfg.IntervalConfigured
	if intervalMutable {
		if value, ok, _ := a.store.GetSetting(ctx, "interval"); ok {
			if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
				interval = parsed
			}
		}
	}
	if interval <= 0 {
		interval = 15 * time.Minute
	}

	concurrency := a.cfg.Concurrency
	concurrencyMutable := !a.cfg.ConcurrencyConfigured
	if concurrencyMutable {
		if value, ok, _ := a.store.GetSetting(ctx, "concurrency"); ok {
			if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
				concurrency = parsed
			}
		}
	}
	if concurrency < 1 {
		concurrency = 1
	}

	return SchedulerConfig{
		Interval:           interval,
		IntervalMutable:    intervalMutable,
		Concurrency:        concurrency,
		ConcurrencyMutable: concurrencyMutable,
	}
}

func (a *App) SaveSchedulerConfig(ctx context.Context, intervalText, concurrencyText string) error {
	current := a.SchedulerConfig(ctx)
	if current.IntervalMutable {
		intervalText = strings.TrimSpace(intervalText)
		if intervalText == "" {
			return errors.New("interval must not be empty")
		}
		interval, err := time.ParseDuration(intervalText)
		if err != nil {
			return fmt.Errorf("parse interval: %w", err)
		}
		if interval < time.Minute {
			return errors.New("interval must be at least 1m")
		}
		if err := a.store.SetSetting(ctx, "interval", interval.String()); err != nil {
			return err
		}
	}

	if current.ConcurrencyMutable {
		concurrency, err := strconv.Atoi(strings.TrimSpace(concurrencyText))
		if err != nil {
			return fmt.Errorf("parse concurrency: %w", err)
		}
		if concurrency < 1 || concurrency > 64 {
			return errors.New("concurrency must be between 1 and 64")
		}
		if err := a.store.SetSetting(ctx, "concurrency", strconv.Itoa(concurrency)); err != nil {
			return err
		}
	}

	a.signalScheduler()
	a.slotsMu.Lock()
	a.slotsCond.Broadcast()
	a.slotsMu.Unlock()
	return nil
}

func (a *App) RepositoryConfig(ctx context.Context) RepositoryConfig {
	if strings.TrimSpace(a.cfg.Repository) != "" {
		branch := strings.TrimSpace(a.cfg.Branch)
		if branch == "" {
			branch = "main"
		}
		return RepositoryConfig{Repository: strings.TrimSpace(a.cfg.Repository), Branch: branch, Configured: true}
	}

	repository, _, _ := a.store.GetSetting(ctx, "repository")
	branch, ok, _ := a.store.GetSetting(ctx, "repository_branch")
	if !ok || strings.TrimSpace(branch) == "" {
		branch = a.cfg.Branch
	}
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}

	repository = strings.TrimSpace(repository)
	branch = strings.TrimSpace(branch)
	return RepositoryConfig{
		Repository: repository,
		Branch:     branch,
		Mutable:    true,
		Configured: repository != "",
	}
}

func (a *App) SaveRepositoryConfig(ctx context.Context, repository, branch string) error {
	if strings.TrimSpace(a.cfg.Repository) != "" {
		return errors.New("repository is configured by static config and cannot be changed in the web UI")
	}
	repository = strings.TrimSpace(repository)
	branch = strings.TrimSpace(branch)
	if repository == "" {
		return errors.New("repository URL must not be empty")
	}
	if branch == "" {
		branch = "main"
	}

	previous := a.RepositoryConfig(ctx)
	if err := a.store.SetSetting(ctx, "repository", repository); err != nil {
		return err
	}
	if err := a.store.SetSetting(ctx, "repository_branch", branch); err != nil {
		return err
	}

	if previous.Repository != repository || previous.Branch != branch {
		a.CancelRunning("repository changed")
		_ = os.RemoveAll(filepath.Join(a.cfg.StateDir, "repo"))
		a.setStatus("", time.Now().UTC(), "repository changed; waiting for next check")
	}
	return nil
}
