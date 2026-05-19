package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func (a *App) ensureRepo(ctx context.Context) (string, error) {
	repoConfig := a.RepositoryConfig(ctx)
	if !repoConfig.Configured {
		return "", fmt.Errorf("repository is not configured")
	}
	repoDir := filepath.Join(a.cfg.StateDir, "repo")
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
		if err := os.RemoveAll(repoDir); err != nil {
			return "", err
		}
		cmd := exec.CommandContext(
			ctx,
			"git",
			"clone",
			"--branch",
			repoConfig.Branch,
			repoConfig.Repository,
			repoDir,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git clone: %w\n%s", err, string(out))
		}
	} else {
		commands := [][]string{
			{"fetch", "origin", repoConfig.Branch},
			{"checkout", repoConfig.Branch},
			{"reset", "--hard", "origin/" + repoConfig.Branch},
		}
		for _, args := range commands {
			cmd := exec.CommandContext(ctx, "git", args...)
			cmd.Dir = repoDir
			out, err := cmd.CombinedOutput()
			if err != nil {
				return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, string(out))
			}
		}
	}
	return repoDir, nil
}

func (a *App) currentCommit(ctx context.Context, repoDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (a *App) currentCommitMessage(ctx context.Context, repoDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "log", "-1", "--pretty=%s")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (a *App) discoverHosts(ctx context.Context, repoDir string) ([]string, error) {
	cmd := exec.CommandContext(
		ctx,
		"nix",
		"--extra-experimental-features",
		"nix-command",
		"--extra-experimental-features",
		"flakes",
		"eval",
		"--json",
		".#nixosConfigurations",
		"--apply",
		"builtins.attrNames",
	)
	cmd.Dir = repoDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("nix eval hosts: %w\n%s", err, stderr.String())
	}
	var hosts []string
	if err := json.Unmarshal(out, &hosts); err != nil {
		return nil, err
	}
	sort.Strings(hosts)
	return hosts, nil
}
