{
  lib,
  buildGoModule,
  stdenv,
  bun,
  fetchFromGitHub,
  makeWrapper,
  version ? "0.5.0",
}:
let
  src = fetchFromGitHub {
    owner = "vexgo-org";
    repo = "vexgo";
    rev = "v${version}";
    hash = "sha256-4V1f6g/rnVVmvoqE5bL7V/1uiu0T+aLvf/Xgm04ylyg=";
  };

  # The frontend is built with `bun install --frozen-lockfile` at build time,
  # which requires network access during the build (the Nix sandbox must allow
  # it, e.g. `sandbox = false` on non-NixOS). This matches how the Docker
  # image and CI build the frontend.
  vexgoFrontend = stdenv.mkDerivation {
    pname = "vexgo-frontend";
    inherit version src;
    sourceRoot = "${src.name}/frontend";

    nativeBuildInputs = [ bun ];

    buildPhase = ''
      runHook preBuild
      chmod -R u+w $NIX_BUILD_TOP/source/backend
      mkdir -p $NIX_BUILD_TOP/source/backend/public/dist
      bun install --frozen-lockfile
      bun run build
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

  ldflags = [
    "-s"
    "-w"
    "-X main.Version=${version}"
  ];
  preBuild = ''
    mkdir -p backend/public/dist
    cp -r ${vexgoFrontend}/. backend/public/dist/
  '';
  postInstall = ''
    mv $out/bin/backend $out/bin/vexgo
  '';
  nativeBuildInputs = [ makeWrapper ];

  meta = with lib; {
    description = "A blog CMS built on React, Go, Gin, JWT, and SQLite";
    homepage = "https://github.com/vexgo-org/vexgo";
    license = licenses.agpl3Only;
    mainProgram = "vexgo";
    platforms = platforms.linux ++ platforms.darwin;
    maintainers = [ antipeth ];
  };
}
