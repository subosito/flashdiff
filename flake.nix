{
  description = "flashdiff — watch file diffs live in your terminal";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };

        # Prefer a Go toolchain that satisfies go.mod (1.25+).
        go = pkgs.go_1_25 or pkgs.go;
        buildGo = pkgs.buildGoModule.override { inherit go; };

        flashdiff = buildGo {
          pname = "flashdiff";
          version =
            let
              rev = self.shortRev or self.dirtyShortRev or "dev";
            in
            "0.1.0+${rev}";

          src = pkgs.lib.cleanSourceWith {
            src = ./.;
            filter =
              path: type:
              let
                base = baseNameOf path;
              in
              # Drop local binaries and nix/devenv noise from the build source.
              !(builtins.elem base [
                "flashdiff"
                ".git"
                ".devenv"
                ".direnv"
                "result"
                "result-bin"
              ]);
          };

          # Bump when go.mod / go.sum change:
          #   nix build .#flashdiff
          # and copy the "got:" hash from the error (or use pkgs.lib.fakeHash first).
          vendorHash = "sha256-8RliwyVMXMGxXEmcZgKtzbX7mXPjdNaYXs2gXEPcykg=";

          ldflags = [
            "-s"
            "-w"
            "-X main.version=${if self ? rev then "git-${self.shortRev}" else "dev"}"
            "-X main.commit=${self.rev or "unknown"}"
            "-X main.date=unknown"
          ];

          meta = with pkgs.lib; {
            description = "Watch file diffs live in your terminal";
            homepage = "https://github.com/subosito/flashdiff";
            license = licenses.mit;
            mainProgram = "flashdiff";
            platforms = platforms.unix;
          };
        };
      in
      {
        packages = {
          default = flashdiff;
          flashdiff = flashdiff;
        };

        apps.default = flake-utils.lib.mkApp {
          drv = flashdiff;
          name = "flashdiff";
        };
      }
    );
}
