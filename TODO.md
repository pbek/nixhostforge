# NixHostForge 0.1 Todos

## Version 0.1 Scope

- [x] Choose project name: NixHostForge.
- [x] Create Go daemon scaffold.
- [x] Create a Nix flake exposing the package and NixOS module.
- [x] Create a devenv environment.
- [x] Add first-use web password setup.
- [x] Add login/logout session handling.
- [x] Store mutable state in SQLite.
- [x] Clone/fetch a configured Git repository.
- [x] Discover hosts from `nixosConfigurations`.
- [x] Let the user select enabled hosts in the web UI.
- [x] Build selected host configurations.
- [x] Show build results and logs in the web UI.
- [x] Pause builds for a selected number of hours.
- [x] Stop currently running build jobs when pausing.
- [x] Configure shoutrrr notification URL in the web UI.
- [x] Send failure notifications once per host/commit.
- [x] Do not automatically rebuild a failed host for the same commit.
- [x] Add basic scheduler tests.
- [x] Add README usage documentation.

## Post-0.1 Ideas

- [ ] Add Prometheus metrics.
- [ ] Add per-host concurrency limits.
- [ ] Add binary cache upload hooks.
- [ ] Add OIDC/reverse-proxy auth support.
- [ ] Add multiple repositories.
- [ ] Add richer Matrix/Telegram/SMTP examples after testing provider-specific URLs.
- [ ] Add log streaming over server-sent events.
