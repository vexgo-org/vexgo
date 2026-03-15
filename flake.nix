{
  description = "VexGo — blog CMS built on React, Go, Gin, JWT, and SQLite";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs = {
    self,
    nixpkgs,
  }: let
    supportedSystems = ["x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin"];

    forEachSupportedSystem = f:
      nixpkgs.lib.genAttrs supportedSystems (system:
        f {
          pkgs = import nixpkgs {inherit system;};
        });
  in {
    overlays.default = import ./nix/overlay.nix;
    nixosModules.default = import ./nix/options.nix;
    packages = forEachSupportedSystem ({pkgs}: {
      vexgo = pkgs.callPackage ./nix/package.nix {};
      default = pkgs.callPackage ./nix/package.nix {};
    });

    devShells = forEachSupportedSystem ({pkgs}: {
      default = pkgs.mkShell {
        packages = with pkgs; [pnpm nodejs go];
      };
    });
  };
}
