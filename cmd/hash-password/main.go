// hash-password is a small helper that reads a password from stdin and prints
// the Argon2id hash suitable for use as ADMIN_PASSWORD_HASH in .env.
package main

import (
	"fmt"
	"os"

	"github.com/m4rcel-lol/cliverse/internal/auth"
	"golang.org/x/term"
)

func main() {
	fmt.Print("Enter password: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading password: %v\n", err)
		os.Exit(1)
	}
	if len(password) == 0 {
		fmt.Fprintln(os.Stderr, "error: password must not be empty")
		os.Exit(1)
	}

	hash, err := auth.HashPassword(string(password))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(hash)
}
