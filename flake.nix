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

        # Single source of truth: ./VERSION (same file Go embeds). Bump VERSION
        # when cutting a release tag (vX.Y.Z → VERSION contains X.Y.Z).
        releaseVersion = pkgs.lib.removeSuffix "\n" (builtins.readFile ./VERSION);

        # Unique Nix drv version: release + short rev when available.
        packageVersion =
          if self ? shortRev then
            "${releaseVersion}+${self.shortRev}"
          else if self ? dirtyShortRev then
            "${releaseVersion}+${self.dirtyShortRev}"
          else
            "${releaseVersion}-dev";

        # What `flashdiff --version` prints (matches goreleaser-style v-prefixed tags).
        versionString =
          if self ? shortRev then
            "v${releaseVersion}+${self.shortRev}"
          else if self ? dirtyShortRev then
            "v${releaseVersion}+${self.dirtyShortRev}"
          else
            "v${releaseVersion}-dev";

        commitString = self.rev or self.dirtyRev or "unknown";

        flashdiff = buildGo {
          pname = "flashdiff";
          version = packageVersion;

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
            "-X main.version=${versionString}"
            "-X main.commit=${commitString}"
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
