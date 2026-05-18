# NixHostForge

NixHostForge prebuilds and verifies NixOS host configurations from a flake. It watches a Git repository, discovers `nixosConfigurations`, builds selected hosts, records the results, and notifies you when a host configuration fails before the host needs to build it.

## Features

- Periodically checks a configured Git repository and branch.
- Discovers hosts from `nixosConfigurations`.
- Web UI for selecting which hosts to build.
- Web UI repository setup when no repository is configured by the module/static config.
- Web UI scheduler setup for interval and concurrency when they are not configured by the module/static config.
- Build history with logs and output paths.
- Pause selector that stops currently running builds and prevents new builds for the selected number of hours.
- First-use password setup and login sessions.
- Failure notifications through one or more shoutrrr URLs.
- Avoids automatically rebuilding a failed host for the same commit.
- Nix package, app, NixOS module, and devenv environment.

See `CHANGELOG.md` for version history.

## License

NixHostForge is licensed under the GNU Affero General Public License v3.0 or later. See `LICENSE`.

## Development

```bash
devenv shell
go test ./...
go run ./cmd/nixhostforge --config ./config.toml
```

Example local config:

```toml
repository = "https://github.com/example/nixos-config.git"
branch = "main"
interval = "15m"
listen_address = "0.0.0.0"
port = 9637
state_dir = "/tmp/nixhostforge"
concurrency = 1
```

## NixOS Module

```nix
{
  services.nixhostforge = {
    enable = true;
    repository = "https://github.com/example/nixos-config.git";
    branch = "main";
    listenAddress = "0.0.0.0";
    port = 9637;
    openFirewall = false;

    # Optional. Leave unset to configure them in the web UI.
    interval = "15m";
    concurrency = 1;
  };
}
```

`repository` is optional. If it is left empty, the first admin user can configure the repository URL and branch from the web UI under Settings.

`interval` and `concurrency` are optional. If either is left unset, it can be configured from the web UI under Settings. If set in the module/static config, the web UI shows it as read-only.

The web interface listens on all interfaces by default. `openFirewall` remains false by default, so expose the port intentionally.

## Notifications

NixHostForge uses shoutrrr notification URLs. Configure one or more URLs in the web UI under Settings. Each URL has its own enabled toggle and test button; disabled URLs are kept but skipped for build failure notifications.

Examples:

```text
smtp://user:pass@mail.example.com:587/?from=nixhostforge@example.com&to=admin@example.com
telegram://TOKEN@telegram?channels=CHAT_ID
matrix://user:pass@matrix.example.com:8448/?rooms=!roomid:matrix.example.com
```

Check shoutrrr's provider documentation for exact URL options for your service.

## Build Behavior

For every enabled host and current commit, NixHostForge builds:

```bash
nix build --print-out-paths .#nixosConfigurations.<host>.config.system.build.toplevel
```

If a host fails for a commit, NixHostForge will not automatically try that host again until the repository has a new commit. You can still trigger a manual build from the Hosts page.
