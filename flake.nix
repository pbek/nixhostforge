{
  description = "NixHostForge - prebuild and verify NixOS host configurations";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    devenv.url = "github:cachix/devenv";
  };

  outputs =
    inputs@{
      self,
      nixpkgs,
      flake-utils,
      devenv,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
        settingsFrontend = pkgs.buildNpmPackage {
          pname = "nixhostforge-settings";
          version = "0.1.0";
          src = ./frontend;
          npmDepsHash = "sha256-dPcSqDufxIGokVzq//P0dcPucZBIzdKWRVPpd4u39RE=";
          VITE_OUT_DIR = "dist";
          installPhase = ''
            runHook preInstall
            mkdir -p $out
            cp -r dist/* $out/
            runHook postInstall
          '';
        };
        nixhostforge = pkgs.buildGoModule {
          pname = "nixhostforge";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-Rg1jVL6Uq0iQmj/uWox78eXAdRWGPEVAktMOjL6uygI=";
          nativeBuildInputs = [ pkgs.makeWrapper ];
          postPatch = ''
            rm -rf internal/app/settings/dist
            mkdir -p internal/app/settings/dist
            cp -r ${settingsFrontend}/* internal/app/settings/dist/
          '';
          postInstall = ''
            wrapProgram $out/bin/nixhostforge \
              --prefix PATH : ${
                pkgs.lib.makeBinPath [
                  pkgs.git
                  pkgs.nix
                  pkgs.openssh
                  pkgs.cacert
                ]
              } \
              --set SSL_CERT_FILE ${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt
          '';
          meta = {
            description = "Prebuild and verify NixOS host configurations from a flake";
            homepage = "https://github.com/omega/nixhostforge";
            license = pkgs.lib.licenses.agpl3Plus;
            mainProgram = "nixhostforge";
          };
        };
      in
      {
        packages.default = nixhostforge;
        apps.default = {
          type = "app";
          program = "${nixhostforge}/bin/nixhostforge";
        };
        devShells.default = devenv.lib.mkShell {
          inherit inputs pkgs;
          modules = [
            (_: {
              devenv.root = "${./.}";
            })
            ./devenv.nix
          ];
        };
      }
    )
    // {
      nixosModules.default = import ./nix/module.nix self;
    };
}
