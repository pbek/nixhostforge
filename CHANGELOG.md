# Changelog

## 0.6.0

### Added

- Present NixHostForge as a local Nix cache warmer and document an ncps cache proxy setup.
- Real-time UI updates via Server-Sent Events (SSE): build lists, host status, dashboard stats, and scheduler status now update automatically without reloading the page.
- Live connection indicator (green pulsing dot) in the navigation bar showing whether the SSE stream is active.
- Live duration counter on running builds ticks every second on all pages, including the build detail page and build tables.
- Auto-scroll build log to the bottom on the detail page when a running build updates.

### Fixed

- Pending (queued) host builds that are superseded by a newly detected commit are now automatically removed from the queue, preventing stale builds from running after the repository has moved on.

## 0.5.0

### Added

- Link the dashboard latest commit to GitHub when the configured repository is hosted on GitHub.
- Include build links and GitHub commit links in build result notifications.
- Add a Settings UI field for the public URL used in notification build links.
- Add search to the Hosts page.
- Add host sorting on the Hosts page, defaulting to enabled hosts first.
- Add remembered host grouping to the Builds page.
- Remember the selected host sorting on the Hosts page.
- Show queued upcoming builds on the Builds page.

### Fixed

- Update the Dashboard last check time after the `Check now` action completes.

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
