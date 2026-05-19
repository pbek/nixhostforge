{
  description = "NixHostForge - prebuild and verify NixOS host configurations";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-darwin"
        "x86_64-linux"
      ];
      forEachSystem = nixpkgs.lib.genAttrs systems;
    in
    {
      packages = forEachSystem (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          settingsFrontend = pkgs.buildNpmPackage {
            pname = "nixhostforge-settings";
            version = "0.2.0";
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
        in
        {
          default = pkgs.buildGoModule {
            pname = "nixhostforge";
            version = "0.2.0";
            src = ./.;
            vendorHash = "sha256-Rg1jVL6Uq0iQmj/uWox78eXAdRWGPEVAktMOjL6uygI=";
            tags = [ "embed_settings" ];
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
        }
      );

      apps = forEachSystem (system: {
        default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/nixhostforge";
        };
      });

      nixosModules.default = import ./nix/module.nix self;
    };
}
