{
  description = "DMS-styled pinentry for GPG/RBW";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
  };

  outputs = inputs @ {self, ...}:
    inputs.flake-parts.lib.mkFlake {inherit inputs;} {
      systems = ["x86_64-linux" "aarch64-linux"];

      perSystem = {pkgs, ...}: {
        packages = {
          pinentry-dms = pkgs.buildGoModule {
            pname = "pinentry-dms";
            version = "0.1.0";
            src = ./.;
            vendorHash = null;
            subPackages = ["cmd/pinentry-dms"];

            meta = {
              description = "DMS-styled pinentry for GPG and RBW";
              mainProgram = "pinentry-dms";
            };
          };

          dms-plugin = pkgs.stdenvNoCC.mkDerivation {
            pname = "dms-plugin-dankPinentry";
            version = "0.1.0";
            src = ./plugin;

            preferLocalBuild = true;
            allowSubstitutes = false;

            installPhase = ''
              mkdir -p $out
              cp -r $src/* $out
            '';

            meta = {
              description = "GPG/SSH passphrase entry with native DMS modal";
              platforms = pkgs.lib.platforms.all;
            };
          };

          default = self.packages.${pkgs.system}.pinentry-dms;
        };
      };
    };
}
