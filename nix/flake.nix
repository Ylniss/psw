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

      # Static version for the project
      version = "0.5";

      # Path to the generated gomod2nix.toml file
      vendorFile = ../gomod2nix.toml;

      # Replace these fake hashes with the actual hashes from failed builds
      vendorHash = "sha256-AQgMbLNm2wp1S0Yif7FnrDIVmVerAEQ+YIyelj9Emso=";

      # Common build dependencies
      nativeDeps = with pkgs; [
        go
        gopls
        gotools
        go-tools
        gomod2nix.packages.${system}.default
        xorg.libX11
        xorg.libX11.dev
        xorg.libXext
        xorg.xorgproto
        libxkbcommon
        pkg-config
        gcc
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

      # Development shell with Go tooling and gomod2nix
      devShell = pkgs.mkShell {
        buildInputs = nativeDeps;
      };
    });
}
