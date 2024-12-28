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
      version = "0.6";
      vendorFile = ../gomod2nix.toml;
      vendorHash = "sha256-6/O2NGJQue4w2DLAGEZ1PZt2dCx9yrqLWZ9Ld7+BwFk=";

      # Common build dependencies
      nativeDeps = with pkgs; [
        go
        gomod2nix.packages.${system}.default
      ];

      # Runtime dependencies
      runtimeDeps = with pkgs; [
        xclip
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
        propagatedBuildInputs = runtimeDeps;
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
        propagatedBuildInputs = runtimeDeps;
      };

      devShells.default = pkgs.mkShell {
        buildInputs = nativeDeps ++ runtimeDeps;
      };
    });
}
