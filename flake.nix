{
  description = "NixHostForge - prebuild and verify NixOS host configurations";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    devenv.url = "github:cachix/devenv";
  };

  outputs = inputs@{ self, nixpkgs, flake-utils, devenv }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        nixhostforge = pkgs.buildGoModule {
          pname = "nixhostforge";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-Rg1jVL6Uq0iQmj/uWox78eXAdRWGPEVAktMOjL6uygI=";
          nativeBuildInputs = [ pkgs.makeWrapper ];
          postInstall = ''
            wrapProgram $out/bin/nixhostforge \
              --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.git pkgs.nix pkgs.openssh pkgs.cacert ]} \
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
            ({ ... }: { devenv.root = "${./.}"; })
            ./devenv.nix
          ];
        };
      }) // {
        nixosModules.default = import ./nix/module.nix self;
      };
}
