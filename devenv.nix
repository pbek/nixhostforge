{ pkgs, ... }:

{
  packages = [
    pkgs.nix
    pkgs.sqlite
  ];

  enterShell = ''
    echo "🛠️ NixHostForge dev shell"
  '';

  scripts = {
    test.exec = "go test ./...";
    run.exec = "go run ./cmd/nixhostforge --config ./config.toml";
    build.exec = "nix build .#";
  };
}
