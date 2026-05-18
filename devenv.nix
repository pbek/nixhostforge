{ pkgs, ... }:

{
  packages = [
    pkgs.nix
    pkgs.sqlite
  ];

  enterShell = ''
    echo "🛠️ NixHostForge dev shell"
  '';

  scripts.test.exec = "go test ./...";
  scripts.run.exec = "go run ./cmd/nixhostforge --config ./config.toml";
  scripts.build.exec = "nix build .#";
}
