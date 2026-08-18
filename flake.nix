{
  description = "Diverge Bank Demo — preview environments for Kubernetes";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go
            go
            air

            # Kubernetes
            kubectl

            # Tools
            gh
            jq
            python3

            # Node (for frontend builds)
            nodejs_22
          ];

          shellHook = ''
            export GOPATH="$HOME/go"
            export PATH="$GOPATH/bin:$PATH"
            echo ""
            echo "🏦 Diverge Bank Demo shell"
            echo "   Go:      $(go version | cut -d' ' -f3)"
            echo "   Air:     $(air -v 2>/dev/null | head -1 || echo 'available')"
            echo "   kubectl: $(kubectl version --client -o json 2>/dev/null | jq -r '.clientVersion.gitVersion' 2>/dev/null || echo 'n/a')"
            echo "   Node:    $(node --version)"
            echo ""
          '';
        };
      }
    );
}
