{
  description = "go shell";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    # nixpkgs.url = "github:NixOS/nixpkgs/da2ae6e41e5787f50b75ff2cf521057ab44d504e";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, flake-utils, nixpkgs }:
    flake-utils.lib.eachDefaultSystem (system: {
      devShell =
        let
          pkgs = import nixpkgs {
            inherit system;
          };

          inherit (pkgs) lib;

        in
        pkgs.mkShell {
          buildInputs = with pkgs; [
            go_1_20
            gopls
            sqlite
          ];
          hardeningDisable = [ "all" ];
        };
    });
}
