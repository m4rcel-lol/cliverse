// hash-password is a small helper that reads a password from stdin and prints
// the Argon2id hash suitable for use as ADMIN_PASSWORD_HASH in .env.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/m4rcel-lol/cliverse/internal/auth"
)

func main() {
	fmt.Print("Enter password: ")
	password, err := readPassword()
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading password: %v\n", err)
		os.Exit(1)
	}
	if len(password) == 0 {
		fmt.Fprintln(os.Stderr, "error: password must not be empty")
		os.Exit(1)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(hash)
}

// readPassword reads a line from stdin. It disables terminal echo via stty
// when stdin is a terminal so the password is not displayed. If stty is
// unavailable the password is read in plain text.
func readPassword() (string, error) {
	// Attempt to hide input by disabling terminal echo.
	sttyOff := exec.Command("stty", "-echo")
	sttyOff.Stdin = os.Stdin
	echoDisabled := sttyOff.Run() == nil
	if echoDisabled {
		defer func() {
			sttyOn := exec.Command("stty", "echo")
			sttyOn.Stdin = os.Stdin
			sttyOn.Run()
		}()
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimRight(scanner.Text(), "\r\n"), scanner.Err()
}
