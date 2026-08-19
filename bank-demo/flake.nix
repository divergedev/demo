{
  description = "bank-demo dev environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            # Go
            go

            # Node / frontend
            nodejs_22
            typescript

            # General
            git
            jq
            curl
          ];

          shellHook = ''
            echo "bank-demo dev shell loaded — Go $(go version | cut -d' ' -f3), Node $(node --version)"
          '';
        };
      });
}
