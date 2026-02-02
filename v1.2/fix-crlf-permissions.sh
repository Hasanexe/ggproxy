#!/bin/bash

if [ -z "$1" ]; then
  echo "Usage: $0 <package-folder>"
  exit 1
fi

PKG="$1"

if [ ! -d "$PKG" ]; then
  echo "Error: Folder '$PKG' does not exist."
  exit 1
fi

echo "Fixing CRLF for text files only..."

find "$PKG" -type f \
  ! -path "$PKG/usr/local/bin/*" \
  -exec sed -i 's/\r$//' {} \;

echo "Setting directory permissions..."
find "$PKG" -type d -exec chmod 755 {} \;

echo "Setting file permissions..."

chmod 644 "$PKG/DEBIAN/control" 2>/dev/null
chmod 755 "$PKG/DEBIAN/postinst" 2>/dev/null
chmod 755 "$PKG/DEBIAN/prerm" 2>/dev/null
chmod 755 "$PKG/DEBIAN/preinst" 2>/dev/null
chmod 755 "$PKG/DEBIAN/postrm" 2>/dev/null

chmod 644 "$PKG/etc/"* 2>/dev/null
chmod 644 "$PKG/lib/systemd/system/"*.service 2>/dev/null
chmod 755 "$PKG/usr/local/bin/"* 2>/dev/null

echo "Done."

