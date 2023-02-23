{ pkgs ? import <nixpkgs> {} }:
pkgs.mkShell {
  hardeningDisable = [ "all" ];
  nativeBuildInputs = with pkgs; [
    go
    gopls
    golangci-lint
    delve
  ];
}
