frontend-build:
    npm --prefix frontend run build

nix-build:
    nix build .#

nix-run:
    nix run .#

nix-build-run: nix-build
    ./result/bin/nixhostforge

open:
    xdg-open http://localhost:9637
