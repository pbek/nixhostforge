# Changelog

## 0.5.0

### Added

- Link the dashboard latest commit to GitHub when the configured repository is hosted on GitHub.
- Add search to the Hosts page.
- Add host sorting on the Hosts page, defaulting to enabled hosts first.

## 0.4.0

### Added

- Add a browser favicon to the embedded settings UI.

### Fixed

- Wake the scheduler when builds resume from a pause so cancelled builds are retried immediately.
- Build enabled hosts again when the configured repository or branch changes, even if the commit hash was built before.
- Wake the scheduler when a host is enabled so it can build the current commit immediately.

## 0.3.1

### Fixed

- Set the browser page title to the active page name.
- Cancel stale `running` builds on startup when the service restarted before their job could finish, and surface stale running build counts in status.

## 0.3.0

### Added

- Show each host's last build time on the dashboard.
- Show the application version in the web menu.

### Fixed

- Hide disabled hosts from the dashboard host list.

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
