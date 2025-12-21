{
  description = "hetzner dev shell";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
  };

  outputs =
    { nixpkgs, ... }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
    in
    {
      devShells.${system}.default = pkgs.mkShell {
        packages = [
          pkgs.clickhouse-lts
          pkgs.protobuf_33
          pkgs.libpcap
          pkgs.entr
          pkgs.delve
          pkgs.changie
        ];
      };
    };
}
