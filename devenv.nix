{ pkgs, ... }:

{
  packages = [
    pkgs.go
    pkgs.gopls
    pkgs.gotools
    pkgs.golangci-lint
    pkgs.git
    pkgs.nix
    pkgs.sqlite
  ];

  scripts.test.exec = "go test ./...";
  scripts.run.exec = "go run ./cmd/nixhostforge --config ./config.toml";
  scripts.build.exec = "nix build .#";
}
