# Changelog

## 0.3.0

### Added

- Show the application version in the web menu.

## 0.2.0

### Added

- Show the current commit message on the dashboard.
- Added a dashboard button to manually build all enabled hosts at the current repository commit.
- Added per-notification URL checkboxes for success messages, warnings, and errors.

### Fixed

- Fixed the dashboard `Check now` button so decorative hero-card styling no longer blocks clicks.
- Centered the notification settings `Test` button vertically with the URL input.

## 0.1.0

Initial release of NixHostForge.

### Added

- Periodic checks for a configured Git repository and branch.
- Host discovery from `nixosConfigurations`.
- Web UI for selecting which hosts to build.
- Web UI repository setup when no repository is configured by the module/static config.
- Web UI scheduler setup for interval and concurrency when they are not configured by the module/static config.
- Test button for notification settings.
- Build history with logs and output paths.
- Pause selector that stops currently running builds and prevents new builds for the selected number of hours.
- First-use password setup and login sessions.
- Failure notifications through shoutrrr.
- Automatic rebuild suppression for hosts that failed on the same commit.
- Nix package, app, NixOS module, and devenv environment.
