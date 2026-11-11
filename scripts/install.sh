#!/usr/bin/env bash
# Install gclone with `go install` and make sure Go's bin folder is on your PATH.
#
#   scripts/install.sh            # latest
#   scripts/install.sh v0.1.0     # a specific version
#
# Needs Go 1.24 or newer (https://go.dev/dl/). If the bin folder is not on your PATH, a line
# is appended to your shell's startup file (~/.zshrc, ~/.bashrc or ~/.profile).
set -euo pipefail

VERSION="${1:-latest}"

command -v go >/dev/null 2>&1 || {
  echo "Go is not installed or not on PATH. Install it from https://go.dev/dl/ and run this again." >&2
  exit 1
}

echo "Installing github.com/rauni5/gclone@${VERSION} ..."
go install "github.com/rauni5/gclone@${VERSION}"

BIN="$(go env GOBIN)"
if [ -z "$BIN" ]; then
  GOPATH_FIRST="$(go env GOPATH)"
  GOPATH_FIRST="${GOPATH_FIRST%%:*}"
  BIN="$GOPATH_FIRST/bin"
fi

case ":$PATH:" in
  *":$BIN:"*)
    echo "$BIN is already on your PATH."
    ;;
  *)
    case "$(basename "${SHELL:-}")" in
      zsh)  RC="$HOME/.zshrc" ;;
      bash) if [ -f "$HOME/.bashrc" ]; then RC="$HOME/.bashrc"; else RC="$HOME/.bash_profile"; fi ;;
      *)    RC="$HOME/.profile" ;;
    esac
    LINE="export PATH=\"\$PATH:$BIN\""
    if [ -f "$RC" ] && grep -qsF "$BIN" "$RC"; then
      echo "$RC already mentions $BIN."
    else
      printf '\n# Added by gclone install script\n%s\n' "$LINE" >> "$RC"
      echo "Added $BIN to PATH in $RC."
    fi
    echo "Open a new terminal, or run:  source $RC"
    ;;
esac

echo "Done. Try:  gclone --help"
