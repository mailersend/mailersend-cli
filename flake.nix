{
  description = "MailerSend CLI - command-line interface for the MailerSend API";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      version = "1.0.1";
      
      # Map nix system to goreleaser naming
      systemMap = {
        "x86_64-linux" = { os = "linux"; arch = "amd64"; };
        "aarch64-linux" = { os = "linux"; arch = "arm64"; };
        "x86_64-darwin" = { os = "darwin"; arch = "amd64"; };
        "aarch64-darwin" = { os = "darwin"; arch = "arm64"; };
      };

      # SHA256 hashes for each platform (updated by CI on release)
      hashes = {
        "x86_64-linux" = "sha256-0WqAEW11HOMM402mnrG9v54oroZ8htx3LKRW93UMsJc=";
        "aarch64-linux" = "sha256-yQboeLbjARPz+nnacQZ/u12dltX8hj23getCOA0zgC4=";
        "x86_64-darwin" = "sha256-215wajyFbUSPtKeDijJ9lqqohHDpYyOR4B9nUUBIyXs=";
        "aarch64-darwin" = "sha256-kojCxh5BxI2rzuxQFprGm4wcPhepTCAk0KJu4LRwbl4=";
      };
    in
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        platformInfo = systemMap.${system} or (throw "Unsupported system: ${system}");
        
        mailersend = pkgs.stdenv.mkDerivation {
          pname = "mailersend";
          inherit version;

          src = pkgs.fetchurl {
            url = "https://github.com/mailersend/mailersend-cli/releases/download/v${version}/mailersend-cli_${version}_${platformInfo.os}_${platformInfo.arch}.tar.gz";
            sha256 = hashes.${system};
          };

          sourceRoot = ".";

          installPhase = ''
            install -Dm755 mailersend $out/bin/mailersend
          '';

          meta = with pkgs.lib; {
            description = "Command-line interface for the MailerSend API";
            homepage = "https://github.com/mailersend/mailersend-cli";
            license = licenses.mit;
            mainProgram = "mailersend";
            platforms = builtins.attrNames systemMap;
          };
        };
      in
      {
        packages = {
          inherit mailersend;
          default = mailersend;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            golangci-lint
            lefthook
          ];
        };
      }
    );
}
