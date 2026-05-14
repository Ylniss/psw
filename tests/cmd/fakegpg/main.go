// fakegpg replaces a real gpg for `git commit -S` in tests: drains stdin,
// writes git's required SIG_CREATED status to stderr, writes a dummy PGP
// signature to stdout, exits 0. Args ignored.
package main

import (
	"io"
	"os"
)

func main() {
	io.Copy(io.Discard, os.Stdin)
	os.Stderr.WriteString("[GNUPG:] SIG_CREATED B 1 10 00 1234567890 0000000000000000\n")
	os.Stdout.WriteString("-----BEGIN PGP SIGNATURE-----\n\nfakesig\n-----END PGP SIGNATURE-----\n")
}
