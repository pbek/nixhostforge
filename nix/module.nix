self:
{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.services.nixhostforge;
  inherit (cfg) package;
  configFile = pkgs.writeText "nixhostforge-config.toml" ''
    repository = "${cfg.repository}"
    branch = "${cfg.branch}"
    listen_address = "${cfg.listenAddress}"
    port = ${toString cfg.port}
    ${lib.optionalString (cfg.publicUrl != "") ''public_url = "${cfg.publicUrl}"''}
    state_dir = "${cfg.stateDir}"
    ${lib.optionalString (cfg.interval != null) ''interval = "${cfg.interval}"''}
    ${lib.optionalString (cfg.concurrency != null) "concurrency = ${toString cfg.concurrency}"}
  '';
in
{
  options.services.nixhostforge = {
    enable = lib.mkEnableOption "NixHostForge host configuration prebuilder";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
      description = "NixHostForge package to run.";
    };

    repository = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = "Git repository URL containing the Nix flake to check. Leave empty to configure it in the web UI.";
    };

    branch = lib.mkOption {
      type = lib.types.str;
      default = "main";
      description = "Git branch to watch.";
    };

    interval = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      example = "15m";
      description = "Polling interval as a Go duration string. Leave null to configure it in the web UI.";
    };

    listenAddress = lib.mkOption {
      type = lib.types.str;
      default = "0.0.0.0";
      description = "Address for the web interface to listen on.";
    };

    port = lib.mkOption {
      type = lib.types.port;
      default = 9637;
      description = "Port for the web interface.";
    };

    publicUrl = lib.mkOption {
      type = lib.types.str;
      default = "";
      example = "https://nixhostforge.example.com";
      description = "Public base URL used to create absolute build links in notifications.";
    };

    stateDir = lib.mkOption {
      type = lib.types.path;
      default = "/var/lib/nixhostforge";
      description = "State directory for repository checkout, SQLite DB, and build state.";
    };

    concurrency = lib.mkOption {
      type = lib.types.nullOr lib.types.ints.positive;
      default = null;
      example = 1;
      description = "Maximum number of concurrent host builds. Leave null to configure it in the web UI.";
    };

    openFirewall = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Open the web interface port in the firewall.";
    };
  };

  config = lib.mkIf cfg.enable {
    users.users.nixhostforge = {
      isSystemUser = true;
      group = "nixhostforge";
      home = cfg.stateDir;
      createHome = true;
    };

    users.groups.nixhostforge = { };

    systemd.tmpfiles.rules = [
      "d ${cfg.stateDir} 0750 nixhostforge nixhostforge -"
    ];

    systemd.services.nixhostforge = {
      description = "NixHostForge NixOS host configuration prebuilder";
      wantedBy = [ "multi-user.target" ];
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];
      path = [
        pkgs.git
        pkgs.nix
        pkgs.openssh
        pkgs.cacert
      ];
      environment = {
        SSL_CERT_FILE = "${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt";
        NIX_SSL_CERT_FILE = "${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt";
        NIX_CONFIG = "experimental-features = nix-command flakes";
      };
      serviceConfig = {
        ExecStart = "${package}/bin/nixhostforge --config ${configFile}";
        User = "nixhostforge";
        Group = "nixhostforge";
        Restart = "on-failure";
        RestartSec = "10s";
      };
    };

    networking.firewall.allowedTCPPorts = lib.mkIf cfg.openFirewall [ cfg.port ];
  };
}
