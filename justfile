import ".shared/common.just"

# By default, run the `--list` command
default:
    @just --list

# Build the frontend using npm
frontend-build:
    npm --prefix frontend run build

# Build the project using nix build
nix-build:
    nix build .#

# Run the project using nix run
nix-run:
    nix run .#

# Build the project and run the resulting binary
nix-build-run: nix-build
    ./result/bin/nixhostforge

# Open the web application in the default browser
open-browser:
    xdg-open http://localhost:9637

# Initialize the /var/lib/nixhostforge directory with the correct permissions
init-var-dir:
    sudo mkdir /var/lib/nixhostforge
    sudo chown $USER /var/lib/nixhostforge
