system:
let
  inherit (builtins) fromJSON readFile;
  inherit ((fromJSON (readFile ../flake.lock)).nodes) nixpkgs gomod2nix;
  fetchLocked =
    node:
    let
      inherit (node.locked)
        owner
        repo
        rev
        narHash
        ;
    in
    fetchTarball {
      url = "https://github.com/${owner}/${repo}/archive/${rev}.tar.gz";
      sha256 = narHash;
    };

  pkgs = import (fetchLocked nixpkgs) {
    inherit system;
    overlays = [
      (import "${fetchLocked gomod2nix}/overlay.nix")
    ];
  };
in
pkgs.buildGoApplication
