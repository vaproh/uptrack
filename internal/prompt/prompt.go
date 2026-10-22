// Package prompt holds tiny stdin helpers. Used only for confirmations
// and disambiguation; everything else stays non-interactive.
package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// IsTTY reports whether stdin is a terminal.
func IsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func readLine() (string, error) {
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// Confirm asks a yes/no question, defaulting to no on empty input
// or when stdin is not a terminal.
func Confirm(question string) bool {
	if !IsTTY() {
		return false
	}
	fmt.Printf("%s [y/N] ", question)
	line, err := readLine()
	if err != nil {
		return false
	}
	line = strings.ToLower(line)
	return line == "y" || line == "yes"
}

// Choose presents numbered options and returns the chosen index.
// It returns an error on invalid input or non-terminal stdin.
func Choose(question string, options []string) (int, error) {
	if !IsTTY() {
		return -1, fmt.Errorf("no terminal: cannot choose interactively")
	}
	fmt.Println(question)
	for i, o := range options {
		fmt.Printf("  %d) %s\n", i+1, o)
	}
	fmt.Print("choice [1]: ")
	line, err := readLine()
	if err != nil {
		return -1, err
	}
	if line == "" {
		line = "1"
	}
	var n int
	_, err = fmt.Sscanf(line, "%d", &n)
	if err != nil || n < 1 || n > len(options) {
		return -1, fmt.Errorf("invalid choice %q", line)
	}
	return n - 1, nil
}
