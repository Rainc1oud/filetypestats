{
  description = "go devShell";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    flake-utils,
    nixpkgs,
  }:
    flake-utils.lib.eachDefaultSystem (system: {
      devShell = let
        pkgs = import nixpkgs {
          inherit system;
        };
        # inherit (pkgs) lib;
      in
        pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            sqlite
          ];
          hardeningDisable = ["all"];
        };
    });
}
