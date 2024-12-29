{
  description = "Flake for psw and clipclean Go binaries";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs";
    flake-utils.url = "github:numtide/flake-utils";
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
    gomod2nix,
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = import nixpkgs {inherit system;};
      version = builtins.readFile ../VERSION;
      vendorFile = ../gomod2nix.toml;
      vendorHash = "sha256-neC5tZA4/9KrfhV9T83IiDF0PbQ+ZSWED6Ql4j1G07Y=";

      nativeDeps = with pkgs; [
        go
        gomod2nix.packages.${system}.default
      ];
    in {
      # Package for the main application
      packages.psw = pkgs.buildGoModule {
        pname = "psw";
        inherit version;
        src = ../.;
        modules = vendorFile;
        inherit vendorHash;
        nativeBuildInputs = nativeDeps;

        postInstall = ''
          ln -sf ${../pswcfg.toml} $out/bin/pswcfg.toml
        '';
      };

      # Package for clipclean binary
      packages.clipclean = pkgs.buildGoModule {
        pname = "clipclean";
        inherit version;
        src = ../.;
        modules = vendorFile;
        subPackages = ["clipclean"];
        inherit vendorHash;
        nativeBuildInputs = nativeDeps;
      };

      devShells.default = pkgs.mkShell {
        buildInputs = nativeDeps;
      };
    });
}
