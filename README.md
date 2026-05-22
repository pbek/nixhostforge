# NixHostForge

NixHostForge prebuilds and verifies NixOS host configurations from a flake. It watches a Git repository, discovers `nixosConfigurations`, builds selected hosts, records the results, warms local Nix caches, and notifies you when a host configuration fails before the host needs to build it.

![NixHostForge dashboard](./nixhostforge.webp)

## Features

- Periodically checks a configured Git repository and branch.
- Discovers hosts from `nixosConfigurations`.
- Web UI for selecting which hosts to build.
- Web UI repository setup when no repository is configured by the module/static config.
- Web UI scheduler setup for interval and concurrency when they are not configured by the module/static config.
- Build history with logs and output paths.
- Warms the builder's Nix store and local Nix cache proxies while prebuilding hosts.
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
public_url = "http://server.lan:9637"
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
    publicUrl = "http://server.lan:9637";
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

Set `publicUrl` when you want notifications to include absolute links back to build logs. If it is not set in static config or the NixOS module, it can be configured from the web UI under Settings.

## Notifications

NixHostForge uses shoutrrr notification URLs. Configure one or more URLs in the web UI under Settings. Each URL has its own enabled toggle, test button, and checkboxes for success messages, warnings, and errors; disabled URLs are kept but skipped for notifications.

Build result notifications include the build log link when `publicUrl` is configured, and include a GitHub commit link when the watched repository is hosted on GitHub.

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

## Local Nix Caches

Prebuilding host configurations is also useful for warming local Nix caches. When the NixHostForge builder uses a local substituter, host builds pull the required closures through that cache before the target machines need them.

One local cache proxy option is [ncps, the Nix Cache Proxy Server](https://github.com/kalbasit/ncps). A minimal NixOS setup looks like this:

```nix
{
  services.ncps = {
    enable = true;
    cache = {
      hostName = "server.lan";
      maxSize = "50G";
      lru.schedule = "0 2 * * *";
    };
    upstream = {
      caches = [
        "https://cache.nixos.org"
        "https://nix-community.cachix.org"
        "https://devenv.cachix.org"
      ];
      publicKeys = [
        "cache.nixos.org-1:6NCHdD59X431o0gWypbMrAURkbJ16ZPMQFGspcDShjY="
        "nix-community.cachix.org-1:mB9FSh9qf2dCimDSUo8Zy7bkq5CX+/rkCWyvRCYg3Fs="
        "devenv.cachix.org-1:w1cLUi8dv3hnoSPGAuibQv+f9TZLr6cv/Hm9XgU50cw="
      ];
    };
  };
}
```
