// Package buildinfo reports where the running binary came from: its version,
// the commit it was built from, and when.
//
// Both binaries need this and neither may reach into the other, so it lives
// below them. internal/cli renders it as `upall version`, and internal/gui will
// show it in an about dialog.
//
// # Where the values come from
//
// Release builds inject them with linker flags:
//
//	go build -ldflags "-X github.com/daniel-kindl/upall/internal/buildinfo.version=1.2.3" ./cmd/...
//
// Nothing types that by hand, and nothing has to. Since Go 1.18 the toolchain
// stamps the commit, its timestamp, and whether the tree was dirty into every
// binary built inside a repository, and [Get] falls back to those stamps field
// by field. So a plain `go build` still reports an accurate commit and date,
// and only the version number is missing, which is the one thing a working tree
// genuinely does not know.
//
// A binary from `go install …@v1.2.3` reports v1.2.3, taken from the module
// version the toolchain recorded.
//
// The values are deliberately package-level variables rather than constants.
// Linker flags can only overwrite variables, and only variables of string type
// in the package being linked.
package buildinfo
