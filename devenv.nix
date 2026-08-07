{...}: {
  languages.go.enable = true;

  # shell-use (https://github.com/microsoft/shell-use): PTY/terminal
  # automation used to drive and test the flashdiff TUI. It is not in
  # nixpkgs or on crates.io, so the prebuilt binary is installed on first
  # shell entry via the upstream install script.
  enterShell = ''
    if ! command -v shell-use >/dev/null 2>&1; then
      echo "installing shell-use (one-time)…"
      export SHELL_USE_INSTALL_DIR="$HOME/.local/bin"
      mkdir -p "$SHELL_USE_INSTALL_DIR"
      curl --proto '=https' --tlsv1.2 -LsSf \
        https://raw.githubusercontent.com/microsoft/shell-use/main/install/install.sh \
        | sh >/dev/null 2>&1 || echo "note: shell-use install failed; install manually"
    fi
  '';
}
