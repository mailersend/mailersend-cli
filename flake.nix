{
  description = "MailerSend CLI - command-line interface for the MailerSend API";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };
  };

  outputs = { self, nixpkgs, flake-utils, gomod2nix }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ gomod2nix.overlays.default ];
        };
      in
      {
        packages = {
          mailersend = pkgs.buildGoApplication {
            pname = "mailersend";
            version = "1.0.3";

            src = ./.;
            modules = ./gomod2nix.toml;

            ldflags = [
              "-s" "-w"
              "-X github.com/mailersend/mailersend-cli/cmd.version=1.0.3"
            ];

            postInstall = ''
              mv $out/bin/mailersend-cli $out/bin/mailersend
            '';

            meta = with pkgs.lib; {
              description = "Command-line interface for the MailerSend API";
              homepage = "https://github.com/mailersend/mailersend-cli";
              license = licenses.mit;
              mainProgram = "mailersend";
            };
          };

          default = self.packages.${system}.mailersend;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            golangci-lint
            lefthook
            gomod2nix.packages.${system}.default
          ];
        };
      }
    );
}
