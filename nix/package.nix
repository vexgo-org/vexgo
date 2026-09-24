{
  lib,
  buildGoModule,
  stdenv,
  bun,
  fetchFromGitHub,
  version ? "0.9.1",
  themeVersion ? "0.1.1",
}:
let
  src = fetchFromGitHub {
    owner = "vexgo-org";
    repo = "vexgo";
    rev = "v${version}";
    hash = "sha256-3/Ng0E/HdslsRLKrSv3sIA4gfzhq2N6Kegc2E95+f3Y=";
  };

  # Both front ends are built with `bun install --frozen-lockfile` at build
  # time, which requires network access during the build (the Nix sandbox must
  # allow it, e.g. `sandbox = false` on non-NixOS). This matches how the Docker
  # image and CI build them.
  #
  # The admin SPA, built into backend/internal/public/dist and embedded with
  # //go:embed dist/**/*.
  vexgoFrontend = stdenv.mkDerivation {
    pname = "vexgo-frontend";
    inherit version src;
    sourceRoot = "${src.name}/frontend";

    nativeBuildInputs = [ bun ];

    buildPhase = ''
      runHook preBuild
      chmod -R u+w $NIX_BUILD_TOP/source/backend
      mkdir -p $NIX_BUILD_TOP/source/backend/internal/public/dist
      bun install --frozen-lockfile
      bun run build
      runHook postBuild
    '';

    installPhase = ''
      runHook preInstall
      cp -r $NIX_BUILD_TOP/source/backend/internal/public/dist $out
      runHook postInstall
    '';
  };

  # The built-in public theme lives in its own repository
  # (https://github.com/vexgo-org/vexgo-default-theme) and is built into
  # backend/internal/public/default-theme, which the backend embeds with
  # //go:embed default-theme.
  vexgoDefaultTheme = stdenv.mkDerivation {
    pname = "vexgo-default-theme";
    version = themeVersion;
    src = fetchFromGitHub {
      owner = "vexgo-org";
      repo = "vexgo-default-theme";
      rev = "v${themeVersion}";
      hash = "sha256-Afvvwj4o6trJwAkDMRkx8fQJsWSKb0uJX7ME5Y9ZJ2s=";
    };

    nativeBuildInputs = [ bun ];

    buildPhase = ''
      runHook preBuild
      bun install --frozen-lockfile
      bun run build
      runHook postBuild
    '';

    installPhase = ''
      runHook preInstall
      cp -r dist $out
      runHook postInstall
    '';
  };
in
buildGoModule {
  pname = "vexgo";
  inherit version src;

  vendorHash = "sha256-cszhKB9IkSkG3CJjO/mPpsNmMIvmJhCFHIIZB/oOAt4=";

  ldflags = [
    "-s"
    "-w"
    "-X main.Version=${version}"
  ];
  # Drop gin's MessagePack binding and its ugorji/go codec dependency, which
  # VexGo never uses (~6 MB smaller binary).
  tags = [ "nomsgpack" ];
  subPackages = [ "backend/cmd/vexgo" ];
  preBuild = ''
    mkdir -p backend/internal/public/dist
    cp -r ${vexgoFrontend}/. backend/internal/public/dist/
    cp -r ${vexgoDefaultTheme}/. backend/internal/public/default-theme/
  '';

  meta = with lib; {
    description = "A blog CMS built on React, Go, Gin, JWT, and SQLite";
    homepage = "https://github.com/vexgo-org/vexgo";
    license = licenses.agpl3Only;
    mainProgram = "vexgo";
    platforms = platforms.linux ++ platforms.darwin;
    maintainers = [ atp-gh ];
  };
}
