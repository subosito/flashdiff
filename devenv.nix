{...}: {
  languages.go.enable = true;

  # Optional: shell-use (https://github.com/microsoft/shell-use) is a
  # PTY/terminal automation tool used to drive and snapshot-test the
  # flashdiff TUI. It is NOT required to build, run, or test the project
  # (`go build` / `go test` are enough) — it's purely a maintainer convenience
  # for terminal-level testing.
  #
  # It is not in nixpkgs or on crates.io, so the prebuilt binary is installed
  # into ~/.local/bin. We deliberately do NOT auto-install on shell entry;
  # run it explicitly if you want it:
  #
  #   devenv shell
  #   install-shell-use
  #
  # You can also just install it yourself per the upstream README.
  scripts.install-shell-use.exec = ''
    export SHELL_USE_INSTALL_DIR="''${SHELL_USE_INSTALL_DIR:-$HOME/.local/bin}"
    mkdir -p "$SHELL_USE_INSTALL_DIR"
    curl --proto '=https' --tlsv1.2 -LsSf \
      https://raw.githubusercontent.com/microsoft/shell-use/main/install/install.sh \
      | sh
  '';
}
