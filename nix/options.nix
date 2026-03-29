{
  config,
  lib,
  pkgs,
  ...
}: let
  cfg = config.services.vexgo;
  settingsFormat = pkgs.formats.yaml {};
  configFile = settingsFormat.generate "vexgo-config.yml" cfg.settings;
in {
  options.services.vexgo = {
    enable = lib.mkEnableOption "VexGo blog CMS";
    package = lib.mkPackageOption pkgs "vexgo" {};

    openFirewall = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Open the firewall port for VexGo.";
    };

    dataDir = lib.mkOption {
      type = lib.types.str;
      default = "/var/lib/vexgo";
      description = "Data directory for storing SQLite database and uploaded media files.";
    };

    settings = lib.mkOption {
      type = settingsFormat.type;
      default = {};
      description = "VexGo configuration in YAML format. See https://github.com/vexgo-org/vexgo for all options.";
      example = lib.literalExpression ''
        {
          addr = "0.0.0.0";
          port = 3001;
          jwt_secret = "your-secret-key";
          db_type = "sqlite";
        }
      '';
    };

    environment = lib.mkOption {
      type = lib.types.attrsOf lib.types.str;
      default = {};
      description = "Environment variables to pass to the VexGo service.";
      example = lib.literalExpression ''
        {
          JWT_SECRET = "your-secret-key";
          S3_ENABLED = "true";
        }
      '';
    };

    environmentFile = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = null;
      description = ''
        Path to a file containing environment variables in KEY=VALUE format.
        Useful for passing secrets without putting them in the Nix store.
      '';
      example = "/run/secrets/vexgo.env";
    };
  };

  config = lib.mkIf cfg.enable {
    services.vexgo.settings = lib.mkDefault {
      addr = "0.0.0.0";
      port = 3001;
      data = cfg.dataDir;
      db_type = "sqlite";
      allow_local_login = true;
      behind_reverse_proxy = false;
      trusted_proxies = [];
      s3_enabled = false;
      oidc_enabled = false;
      oidc_scopes = "openid profile email";
      oidc_email_claim = "email";
      oidc_name_claim = "name";
      oidc_group_claim = "groups";
      oidc_auto_redirect = false;
      oidc_verify_email = false;
    };

    systemd.services.vexgo = {
      description = "VexGo Blog CMS";
      wantedBy = ["multi-user.target"];
      after = ["network.target"];
      serviceConfig = {
        Type = "simple";
        User = "vexgo";
        Group = "vexgo";
        Environment = lib.mapAttrsToList (k: v: "${k}=${v}") cfg.environment;
        EnvironmentFile = lib.mkIf (cfg.environmentFile != null) cfg.environmentFile;
        ExecStart = "${lib.getExe cfg.package} -c ${configFile}";
        Restart = "on-failure";
        RestartSec = "5s";
        StateDirectory = "vexgo";
        RuntimeDirectory = "vexgo";
        WorkingDirectory = cfg.dataDir;
      };
    };

    networking.firewall.allowedTCPPorts = lib.mkIf cfg.openFirewall [
      cfg.settings.port
    ];

    users.users.vexgo = {
      isSystemUser = true;
      group = "vexgo";
      home = cfg.dataDir;
      description = "VexGo service user";
    };

    users.groups.vexgo = {};
  };
}
