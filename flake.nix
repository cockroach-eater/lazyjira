{
  description = "Terminal UI for Jira";
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };
  outputs = { self, nixpkgs, flake-utils, gomod2nix }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        buildGoApplication = gomod2nix.legacyPackages.${system}.buildGoApplication;
      in {
        packages = rec {
          lazyjira = buildGoApplication {
            pname = "lazyjira";
            version = self.shortRev or self.dirtyShortRev or "dev";
            src = self;
            modules = ./gomod2nix.toml;
            ldflags = [ "-s" "-w" "-X main.version=${self.shortRev or "dev"}" ];
            subPackages = [ "cmd/lazyjira" ];
            meta = with pkgs.lib; {
              description = "Terminal UI for Jira";
              homepage = "https://github.com/textfuel/lazyjira";
              license = licenses.mit;
              mainProgram = "lazyjira";
            };
          };
          default = lazyjira;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.go
            gomod2nix.packages.${system}.default
          ];
        };
      }
    );
}
