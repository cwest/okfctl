# Install

Three paths get you running fast, and they're in the
[README](../README.md#install): Homebrew, the `install.sh` one-liner, and
`go install`. This page covers everything else — prebuilt archives, system
packages, signature verification, and building from source.

## Prebuilt binaries

Download the archive for your platform from the
[releases page](https://github.com/cwest/okfctl/releases): macOS, Linux, and
Windows, on amd64 and arm64. Each archive bundles both `okfctl` and the
`okfctl-search` plugin — extract them onto your `PATH`:

```sh
tar -xzf okfctl_<version>_<os>_<arch>.tar.gz     # or unzip the .zip on Windows
sudo mv okfctl okfctl-search /usr/local/bin/
```

## System packages (Linux)

```sh
sudo dpkg -i okfctl_<version>_linux_<arch>.deb    # Debian/Ubuntu
sudo rpm -i  okfctl_<version>_linux_<arch>.rpm    # Fedora/RHEL
```

## Verifying a release

Every release ships an SBOM (syft) and is signed with cosign (keyless via
Sigstore). Verify the checksums file, which covers every artifact:

```sh
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp 'https://github.com/cwest/okfctl/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

## Confirming the install

```sh
okfctl version        # e.g. okfctl v0.2.0 (commit <sha>, built <date>)
okfctl --version      # same string
```

## Building from source

```sh
go build -o okfctl .                    # dynamic build; version reports "dev"
CGO_ENABLED=0 go build -o okfctl .      # static, no cgo
```
