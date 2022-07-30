# qrc

[![Build Status](https://cloud.drone.io/api/badges/pgaskin/qrc/status.svg)](https://cloud.drone.io/pgaskin/qrc) [![PkgGoDev](https://pkg.go.dev/badge/github.com/pgaskin/qrc)](https://pkg.go.dev/github.com/pgaskin/qrc) ![GitHub Tag](https://img.shields.io/github/v/tag/pgaskin/qrc)

Go library and command-line tool to extract Qt resources from RCC files and executables.

This package supports resource formats 1-3 and includes language/country code information from Qt 5.13. Resources can be compressed using zlib or zstd.

See [pkg.go.dev/github.com/pgaskin/qrc](https://pkg.go.dev/github.com/pgaskin/qrc) for the Go library documentation.

The command-line tool, [qrc2zip](./qrc2zip), can be installed with `GO111MODULE=on go get github.com/pgaskin/qrc/cmd/qrc2zip`.

To automatically find offsets for ARM binaries with Qt resources embedded by rcc, use `scripts/armqrc.py`.

```
Usage: qrc2zip [options] rcc_file
       qrc2zip [options] executable format_version tree_offset data_offset names_offset

Options:
  -o, --output string         Output filename (default "resources.zip")
  -f, --force                 Ignore errors during extraction if possible
  -r, --recursive             Expand nested RCC files
  -e, --exclude stringArray   Exclude files matching this glob (can be specified multiple times)
  -v, --verbose               Show information about the files being extracted
  -h, --help                  Show this help text

Executable offsets:
  To find executable offsets and format version, look for calls to qRegisterResourceData. These
  are usually within entry points or qInitResource* functions. qRegisterResourceData takes four
  arguments: format, tree, names, data.

Qt support:
  Format versions 1-3 are supported, along with locale/country codes from Qt 5.13. Resources
  can be compressed with zlib or zstd.

Output:
  The extracted resources are written to a zip file. The directory structure is preserved and
  separated with forward slashes on all platforms. If the file has language/country constraints,
  they are added to the filename before the extension in the format '[language!LanguageName]'
  and [country!CountryName]. If the Qt resource format is >= 2, the modification time is also
  written for each file.

github.com/pgaskin/qrc
```
