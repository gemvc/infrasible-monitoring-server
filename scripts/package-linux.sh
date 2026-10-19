#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p dist
VERSION="$(tr -d ' \t\r\n' <VERSION.md)"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Invalid VERSION.md" >&2; exit 1; }
GO_VER="$(sed -n 's/^const relayVersion = "\([^"]*\)"/\1/p' version.go)"
[[ "$VERSION" == "$GO_VER" ]] || { echo "VERSION.md ($VERSION) != version.go ($GO_VER)" >&2; exit 1; }
if [[ -n "${GITHUB_REF_NAME:-}" && "${GITHUB_REF_TYPE:-}" == "tag" ]]; then
  [[ "$GITHUB_REF_NAME" == "v$VERSION" ]] || { echo "Tag does not match relay version" >&2; exit 1; }
fi

SERVICE=packaging/systemd/infrasible-relay.service
CONFIG_EXAMPLE=packaging/config.example.json
ACL_EXAMPLE=packaging/acl.example.json
[[ -f "$SERVICE" ]] || { echo "missing $SERVICE" >&2; exit 1; }
[[ -f "$CONFIG_EXAMPLE" ]] || { echo "missing $CONFIG_EXAMPLE" >&2; exit 1; }
[[ -f "$ACL_EXAMPLE" ]] || { echo "missing $ACL_EXAMPLE" >&2; exit 1; }

for target in amd64 arm64 armv7 armv6; do
  case "$target" in
    amd64) arch=amd64; goarm=''; debarch=amd64 ;;
    arm64) arch=arm64; goarm=''; debarch=arm64 ;;
    armv7) arch=arm; goarm=7; debarch=armhf ;;
    armv6) arch=arm; goarm=6; debarch='' ;;
  esac
  binary="infrasible-relay-linux-$target"
  echo "Building $binary"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" GOARM="$goarm" GOTOOLCHAIN=local \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o "dist/$binary" ./cmd/infrasible-relay
  file "dist/$binary"
  case "$target" in
    amd64|arm64) [[ "$(file -b "dist/$binary")" == *"ELF 64-bit"* ]] || exit 1 ;;
    armv*) [[ "$(file -b "dist/$binary")" == *"ELF 32-bit"* ]] || exit 1 ;;
  esac
  dir="$(mktemp -d)"
  install -Dm755 "dist/$binary" "$dir/infrasible-relay"
  cp LICENSE "$dir/LICENSE"
  cp "$SERVICE" "$dir/infrasible-relay.service"
  cp "$CONFIG_EXAMPLE" "$dir/config.example.json"
  cp "$ACL_EXAMPLE" "$dir/acl.example.json"
  tar -C "$dir" -czf "dist/infrasible-relay_${VERSION}_linux_${target}.tar.gz" \
    infrasible-relay LICENSE infrasible-relay.service config.example.json acl.example.json
  rm -rf "$dir"
  if [[ -n "$debarch" ]] && command -v dpkg-deb >/dev/null 2>&1; then
    pkg="$(mktemp -d)"
    install -Dm755 "dist/$binary" "$pkg/usr/bin/infrasible-relay"
    mkdir -p "$pkg/DEBIAN" "$pkg/usr/share/doc/infrasible-relay"
    cp LICENSE "$pkg/usr/share/doc/infrasible-relay/copyright"
    install -Dm644 "$SERVICE" "$pkg/lib/systemd/system/infrasible-relay.service"
    install -Dm644 "$CONFIG_EXAMPLE" "$pkg/usr/share/infrasible-relay/config.example.json"
    install -Dm644 "$ACL_EXAMPLE" "$pkg/usr/share/infrasible-relay/acl.example.json"
    install -Dm755 packaging/debian/postinst "$pkg/DEBIAN/postinst"
    install -Dm755 packaging/debian/prerm "$pkg/DEBIAN/prerm"
    install -Dm755 packaging/debian/postrm "$pkg/DEBIAN/postrm"
    printf 'Package: infrasible-relay\nVersion: %s\nSection: admin\nPriority: optional\nArchitecture: %s\nMaintainer: Infrasible Maintainers <maintainers@users.noreply.github.com>\nDescription: Infrasible monitoring Live bridge (inbound mTLS)\n' \
      "$VERSION" "$debarch" > "$pkg/DEBIAN/control"
    dpkg-deb --build --root-owner-group "$pkg" "dist/infrasible-relay_${VERSION}_${debarch}.deb"
    rm -rf "$pkg"
  fi
done

# A generic, service-free RPM for x86_64. Other RPM distributions can use tar.gz.
if command -v rpmbuild >/dev/null 2>&1; then
  top="$(mktemp -d)"
  mkdir -p "$top"/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
  cp dist/infrasible-relay-linux-amd64 "$top/SOURCES/infrasible-relay"
  cat > "$top/SPECS/infrasible-relay.spec" <<EOF
Name: infrasible-relay
Version: $VERSION
Release: 1
Summary: Infrasible monitoring Live bridge
License: Apache-2.0
BuildArch: x86_64
%description
Infrasible monitoring Live bridge with inbound mTLS for Agents.
%prep
%build
%install
mkdir -p %{buildroot}/usr/bin
install -m 0755 %{_sourcedir}/infrasible-relay %{buildroot}/usr/bin/infrasible-relay
%files
/usr/bin/infrasible-relay
EOF
  rpmbuild -bb --define "_topdir $top" "$top/SPECS/infrasible-relay.spec"
  find "$top/RPMS" -type f -name '*.rpm' -exec cp {} dist/ \;
  rm -rf "$top"
fi

(
  cd dist
  shopt -s nullglob
  files=(
    infrasible-relay-linux-amd64
    infrasible-relay-linux-arm64
    infrasible-relay-linux-armv7
    infrasible-relay-linux-armv6
    *.tar.gz
    *.deb
    *.rpm
  )
  sha256sum "${files[@]}" | awk '!seen[$2]++' > SHA256SUMS
)
echo "Packaged relay $VERSION"
