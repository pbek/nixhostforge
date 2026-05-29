# NixHostForge: prebuild NixOS hosts and warm local caches before deploying

Hi everyone,

I would like to present **NixHostForge**, a small self-hosted service for people who manage one or more NixOS machines from a flake and want to know earlier when a host configuration breaks.

Project repository:

https://github.com/pbek/nixhostforge

NixHostForge watches a Git repository, discovers its `nixosConfigurations`, lets you select which hosts should be built, and periodically prebuilds those host configurations. The goal is simple: catch broken host configs before the actual machine needs to rebuild or deploy them.

For every enabled host, NixHostForge runs the equivalent of:

```bash
nix build --print-out-paths .#nixosConfigurations.<host>.config.system.build.toplevel
```

It records the build result, keeps logs and output paths, shows everything in a web UI, and can notify you via shoutrrr-supported notification URLs such as SMTP, Matrix, Telegram, etc.

## Why I built it

I wanted something between "just run `nixos-rebuild` on the host and hope it works" and a full CI system.

The intended use case is:

- You have a flake with multiple NixOS hosts.
- You want one small service to watch the main branch.
- You want to know if `laptop`, `server`, `router`, or `workstation` no longer builds.
- You want the machines themselves to spend less time downloading or building when they update.
- You do not necessarily want to set up a full forge-integrated CI system.

NixHostForge is especially useful for personal infrastructure, homelabs, small fleets, and self-hosted NixOS setups.

## Features

Current features include:

- Periodic checks of a configured Git repository and branch.
- Automatic discovery of hosts from `nixosConfigurations`.
- Web UI for enabling and disabling hosts.
- Build history with logs and output paths.
- Host build priority ordering.
- Configurable scheduler interval and concurrency.
- Pause button that stops running builds and prevents new ones temporarily.
- First-use password setup and login sessions.
- Notifications through one or more shoutrrr URLs.
- Suppression of automatic rebuilds for a host that already failed at the same commit.
- Real-time UI updates via Server-Sent Events.
- Nix package, Nix app, NixOS module, and devenv environment.

A minimal NixOS module setup looks like this:

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

    interval = "15m";
    concurrency = 1;
  };
}
```

The repository, interval, concurrency, and public URL can also be configured from the web UI if they are not set statically.

## Cache warming

One important part of NixHostForge is that it is not only a "does this host build?" checker. It also helps warm caches.

When NixHostForge prebuilds a host configuration, the builder has to fetch or build the closure needed for that host's system toplevel. That has two practical effects:

- The builder's own Nix store is warmed.
- If the builder uses a local Nix cache proxy, that proxy is warmed too.

For example, you can run NixHostForge on a machine that is configured to use a local cache proxy such as [`ncps`](https://github.com/kalbasit/ncps). When NixHostForge builds `.#nixosConfigurations.server.config.system.build.toplevel`, Nix pulls the needed paths through the proxy. Later, when the actual `server` machine rebuilds, those same paths are already available from the local cache, usually over the LAN.

So the flow becomes:

1. A commit lands in your NixOS config repo.
2. NixHostForge notices the new commit.
3. It builds the selected host configurations.
4. During that build, dependencies are fetched through your local cache proxy.
5. The real machines later rebuild and can substitute already-warmed paths from the local cache.

This is not the same as pushing signed build outputs to a public binary cache. NixHostForge currently focuses on **prebuilding and warming local caches**, especially cache proxies. Binary cache upload hooks may be a future addition, but the current model is intentionally simple: make the local network cache hot before the machines need it.

A minimal `ncps` setup could look like this:

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

Then configure the NixHostForge machine to use that cache as a substituter.

## How it differs from Argunix

A natural comparison is [`argunix`](https://codeberg.org/tfc/argunix), which is a much more complete Nix CI system.

The short version:

**Argunix is a forge-integrated Nix CI system. NixHostForge is a focused NixOS host prebuilder and local cache warmer.**

Argunix watches repositories through forge webhooks, evaluates pushes and PRs, builds flake outputs such as:

- `packages.<system>`
- `checks.<system>`
- `devShells.<system>`
- `nixosConfigurations.<name>`

It posts statuses back to GitHub, GitLab, Forgejo/Gitea/Codeberg, supports remote builders, has PR trust handling, can publish successful builds to binary caches, and is designed as a general CI service.

NixHostForge intentionally does less:

- It focuses only on `nixosConfigurations`.
- It builds host system toplevels.
- It polls a configured repository and branch instead of integrating deeply with forges.
- It does not manage PR checks or commit statuses.
- It does not try to be a full CI replacement.
- It has a web UI for selecting hosts, viewing logs, pausing builds, and configuring simple runtime settings.
- It is designed around "are my NixOS hosts still buildable, and did I warm my local cache?"

So if you want a general Nix CI that talks to your forge, handles PRs, posts commit statuses, builds packages/checks/devShells, uses remote builders, and pushes to binary caches, Argunix is the more appropriate project.

If you mainly want a small service that sits next to your NixOS config repo, prebuilds selected hosts from `nixosConfigurations`, notifies you when a host breaks, and warms your local cache before your machines update, NixHostForge is aimed at that narrower use case.

## License

NixHostForge is licensed under the GNU Affero General Public License v3.0 or later.

Feedback, ideas, and bug reports are very welcome.
