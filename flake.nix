{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };
  outputs = {
    nixpkgs,
    flake-utils,
    ...
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        pkgs = nixpkgs.legacyPackages.${system};
      in {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # generic
            git
            git-lfs
            gnumake
            gnused
            gnutar
            gzip

            # frontend
            nodejs_20

            # backend
            gofumpt
            sqlite
            go
            gopls
          ];
        };
      }
    );
}
