# unzip-riscos

[![CI](https://github.com/david-ramsden/unzip-riscos/actions/workflows/ci.yml/badge.svg)](https://github.com/david-ramsden/unzip-riscos/actions/workflows/ci.yml)
[![Release](https://github.com/david-ramsden/unzip-riscos/actions/workflows/release.yml/badge.svg)](https://github.com/david-ramsden/unzip-riscos/actions/workflows/release.yml)

Extract ZIP files preserving RISC OS filetypes as NFS-encoded `,xxx` suffixes.

Uses Go's `archive/zip` (supports Stored and Deflate compression) and renames extracted files by appending `,xxx` suffixes based on RISC OS Info-ZIP extra fields (signature `0x4341` `'AC'`). Directories are detected by trailing slash and not renamed.

## Usage

```
unzip-riscos [-v] <zipfile> [<zipfile> ...] <destdir>
```

| Flag | Description |
|------|-------------|
| `-v` | Verbose output |

## Installation

Download a pre-built binary for your platform from the [Releases](https://github.com/david-ramsden/unzip-riscos/releases) page.

## Building from source

Requires Go 1.22 or later.

| Target | Description |
|--------|-------------|
| `make` | Build binary for the current platform |
| `make test` | Run tests |
| `make dist` | Cross-compile binaries for all platforms |
| `make clean` | Remove built binaries |

## License

See [LICENSE](LICENSE).
