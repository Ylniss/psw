{
  description = "Flake for psw and clipclean Go binaries";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
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
      lib = pkgs.lib;

      removeWhitespacesFunc = str: lib.strings.replaceStrings [" " "\t" "\n" "\r"] ["" "" "" ""] str;
      rawVersion = builtins.readFile ../VERSION;
      version = removeWhitespacesFunc rawVersion;

      vendorFile = ../gomod2nix.toml;
      vendorHash = lib.fakeHash;
      src = ../.;

      nativeDeps = with pkgs; [
        go_1_26
        gomod2nix.packages.${system}.default
      ];
    in {
  packages.psw = pkgs.buildGoModule {
        pname = "psw";
        inherit version;
        inherit src;
        modules = vendorFile;
        inherit vendorHash;
        nativeBuildInputs = nativeDeps;

        subPackages = [
          "cmd/psw"
          "cmd/clipclean"
        ];

        postInstall = ''
          mkdir -p $out/bin
          cp ${../pswcfg-template.toml} $out/bin/pswcfg.toml
        '';
      };

      devShells.default = pkgs.mkShell {
        buildInputs = nativeDeps;
      };
    });
}
