{
  lib,
  buildGoModule,
  stdenv,
  nodejs,
  pnpm,
  fetchPnpmDeps,
  pnpmConfigHook,
  fetchFromGitHub,
  makeWrapper,
  version ? "0.4.0",
}: let
  src = fetchFromGitHub {
    owner = "weimm16";
    repo = "vexgo";
    rev = "v${version}";
    hash = "sha256-4V1f6g/rnVVmvoqE5bL7V/1uiu0T+aLvf/Xgm04ylyg=";
  };

  vexgoFrontendDeps = fetchPnpmDeps {
    pname = "vexgo-frontend";
    inherit version src;
    sourceRoot = "${src.name}/frontend";
    fetcherVersion = 1;
    hash = "sha256-4SuyZcMhDYChxsvkHl2lxyO8U6tR39ibOK0mtFpD9UE=";
  };

  vexgoFrontend = stdenv.mkDerivation {
    pname = "vexgo-frontend";
    inherit version src;
    sourceRoot = "${src.name}/frontend";

    nativeBuildInputs = [nodejs pnpm pnpmConfigHook];
    pnpmDeps = vexgoFrontendDeps;

    preBuild = ''
      chmod -R u+w $NIX_BUILD_TOP/source/backend
      mkdir -p $NIX_BUILD_TOP/source/backend/public/dist
    '';

    buildPhase = ''
      runHook preBuild
      mkdir -p $NIX_BUILD_TOP/source/backend/public/dist
      pnpm run build
      runHook postBuild
    '';

    installPhase = ''
      runHook preInstall
      cp -r $NIX_BUILD_TOP/source/backend/public/dist $out
      runHook postInstall
    '';
  };
in
  buildGoModule {
    pname = "vexgo";
    inherit version src;

    vendorHash = "sha256-Ea+Zh21mkmjv2jmAHwiqxa4/bLWMLwcgzU2Tz3gvUIA=";

    ldflags = ["-s" "-w" "-X main.Version=${version}"];
    preBuild = ''
      mkdir -p backend/public/dist
      cp -r ${vexgoFrontend}/. backend/public/dist/
    '';
    postInstall = ''
      mv $out/bin/backend $out/bin/vexgo
    '';
    nativeBuildInputs = [makeWrapper];

    meta = with lib; {
      description = "A blog CMS built on React, Go, Gin, JWT, and SQLite";
      homepage = "https://github.com/weimm16/vexgo";
      license = licenses.agpl3Only;
      mainProgram = "vexgo";
      platforms = platforms.linux ++ platforms.darwin;
      maintainers = [antipeth];
    };
  }
