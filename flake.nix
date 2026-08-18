{
  description = "MailerSend CLI - command-line interface for the MailerSend API";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      version = "2.0.0";

      # Map nix system to Rust target triples (cargo-dist artifact naming)
      systemMap = {
        "x86_64-linux" = "x86_64-unknown-linux-gnu";
        "aarch64-linux" = "aarch64-unknown-linux-gnu";
        "x86_64-darwin" = "x86_64-apple-darwin";
        "aarch64-darwin" = "aarch64-apple-darwin";
      };

      # SHA256 hashes for each platform (updated by CI on release)
      hashes = {
        "x86_64-linux" = "sha256-Sh5tdOFrmiRGwkL/WsQq4vjSyYXgnZnj+53hF0cUkWw=";
        "aarch64-linux" = "sha256-uif2wvP3lCHXo17kZRFzxQzy8UN71zDA8kcrDPdfeHk=";
        "x86_64-darwin" = "sha256-EHOeqfEEF1ZgA+q7AhSK+buphUgeLnaziWfdlvL9Rew=";
        "aarch64-darwin" = "sha256-1wNs99InY++Uu7TnNl34WxYup7YhiPeqyi9UuEoWL5U=";
      };
    in
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        target = systemMap.${system} or (throw "Unsupported system: ${system}");

        mailersend = pkgs.stdenv.mkDerivation {
          pname = "mailersend";
          inherit version;

          src = pkgs.fetchurl {
            url = "https://github.com/mailersend/mailersend-cli/releases/download/v${version}/mailersend-${target}.tar.gz";
            sha256 = hashes.${system};
          };

          # cargo-dist tarballs unpack to a directory named after the archive
          sourceRoot = "mailersend-${target}";

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
            cargo
            rustc
            rustfmt
            clippy
            lefthook
          ];
        };
      }
    );
}
