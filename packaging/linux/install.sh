#!/bin/sh
# Installs FM NewGen Faces for the current user (no root needed):
#   binary  -> ~/.local/bin/fm-newgen-faces
#   launcher-> ~/.local/share/applications/fm-newgen-faces.desktop
#   icon    -> ~/.local/share/icons/hicolor/256x256/apps/fm-newgen-faces.png
# Run ./install.sh --uninstall to remove them again.
set -e
here=$(cd "$(dirname "$0")" && pwd)
bin="$HOME/.local/bin"
apps="$HOME/.local/share/applications"
icons="$HOME/.local/share/icons/hicolor/256x256/apps"

if [ "$1" = "--uninstall" ]; then
  rm -f "$bin/fm-newgen-faces" "$apps/fm-newgen-faces.desktop" "$icons/fm-newgen-faces.png"
  echo "FM NewGen Faces removed."
  exit 0
fi

mkdir -p "$bin" "$apps" "$icons"
install -m 755 "$here/fm-newgen-faces" "$bin/fm-newgen-faces"
install -m 644 "$here/fm-newgen-faces.desktop" "$apps/fm-newgen-faces.desktop"
install -m 644 "$here/fm-newgen-faces.png" "$icons/fm-newgen-faces.png"
command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$apps" || true
command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -q "$HOME/.local/share/icons/hicolor" || true
echo "Installed. Start 'FM NewGen Faces' from your app menu or run: $bin/fm-newgen-faces"
case ":$PATH:" in *":$bin:"*) ;; *) echo "Note: $bin is not on your PATH." ;; esac
