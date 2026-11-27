# Install

Each install method puts two programs on your machine: `okfctl` and the
`okfctl-search` plugin. They're written in pure Go and run offline. Below is
the install script on a new Linux machine, then the check that it worked:

<!-- transcript: skip installs from the network and prints a release build's version string -->
```text
$ curl -sSL https://okfctl.dev/install.sh | sh
==> resolving latest release
==> installing okfctl v0.5.0 for linux/arm64
==> downloading okfctl_0.5.0_linux_arm64.tar.gz
==> downloading checksums.txt
==> checksum verified
==> installed okfctl and okfctl-search to /usr/local/bin
okfctl 0.5.0 (commit d9901295a4b5e66108697e40632b6888f6da93e8, built 2026-11-27T13:25:03Z)
$ okfctl plugin list
okfctl-search	/usr/local/bin/okfctl-search
```

The script picked the archive for the machine's OS and CPU and checked it
against the release's `checksums.txt`. Then it copied the two programs into
`/usr/local/bin` and ran `okfctl version` to print the last line you see.
`okfctl plugin list` shows that okfctl found the search plugin next to it.

The script works on macOS and Linux. If it can't write to `/usr/local/bin` it
uses `sudo`. With no `sudo`, it falls back to `~/.local/bin`. Set
`OKFCTL_BIN_DIR` to pick the directory yourself. To install a release other
than the latest, set `OKFCTL_VERSION=v0.5.0`. On a checksum mismatch the script
quits and installs nothing.

## Homebrew

```sh
brew install cwest/tap/okfctl
```

This works on macOS and on Linux with [Homebrew](https://brew.sh). It links
the two programs into Homebrew's `bin` directory.

## Go toolchain

You need Go 1.26 or later. Install each program by its module path:

<!-- transcript: skip installs from the network and prints a release build's version string -->
```text
$ go install github.com/cwest/okfctl@latest
$ go install github.com/cwest/okfctl/cmd/okfctl-search@latest
$ okfctl version
okfctl v0.5.0 (commit none, built unknown)
```

`go install` doesn't stamp a commit or build date. The output says
`none` and `unknown`. The version comes from the module tag. The programs land
in `$(go env GOPATH)/bin`. Put that directory on your `PATH`.

## Prebuilt archives

The [releases page](https://github.com/cwest/okfctl/releases) has amd64 and
arm64 archives for Windows (as a `.zip`) and for macOS and Linux (as a
`.tar.gz`). Each one has `okfctl` and `okfctl-search` in it, plus the
license and the README. Pull
out the two programs and move them onto your `PATH`:

```sh
tar -xzf okfctl_0.5.0_darwin_arm64.tar.gz okfctl okfctl-search
sudo mv okfctl okfctl-search /usr/local/bin/
```

## Debian and RPM packages

There are `.deb` and `.rpm` packages too, for Linux on amd64 and arm64. Either
one puts the two programs in `/usr/bin`.

```sh
sudo dpkg -i okfctl_0.5.0_linux_amd64.deb          # Debian, Ubuntu
sudo dnf install ./okfctl_0.5.0_linux_amd64.rpm    # Fedora, RHEL
```

The RPM has no GPG signature. On Fedora 45, `rpm -i` by itself refuses it
with `package okfctl-0.5.0-1.aarch64 does not verify: no signature`.
`dnf install` on the downloaded file works. You can still check the package
against the release checksums below.

## Checking a release

Each release has a `checksums.txt` with a SHA-256 for each archive and
package. [cosign](https://github.com/sigstore/cosign) signs it with Sigstore's
keyless signing. The signature lives in `checksums.txt.sigstore.json`.
Download the two files next to your archive. Check the signature, then check the
archive against the list:

```text
$ cosign verify-blob \
    --bundle checksums.txt.sigstore.json \
    --certificate-identity-regexp 'https://github.com/cwest/okfctl/.*' \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com \
    checksums.txt
Verified OK
$ shasum -a 256 --ignore-missing -c checksums.txt
okfctl_0.5.0_darwin_arm64.tar.gz: OK
```

`Verified OK` means this repository's release job on GitHub Actions signed the
checksums file. The second command checks only the files you downloaded. On
Linux, `sha256sum --ignore-missing -c checksums.txt` does the same. Each
archive also has a software bill of materials (SBOM) next to it, named after
the archive with `.sbom.json` on the end.

## Building from a checkout

```sh
git clone https://github.com/cwest/okfctl.git
cd okfctl
go build -o okfctl .
go build -o okfctl-search ./cmd/okfctl-search
```

Build from a checkout and `okfctl version` reports a Go pseudo-version like
`v0.5.1-0.20261127154954-6759558a79d1`. It reports `dev` if Go can't read any
version control information. Release builds set `CGO_ENABLED=0`. Do the same
if you want a matching static binary:

```sh
CGO_ENABLED=0 go build -o okfctl .
```
