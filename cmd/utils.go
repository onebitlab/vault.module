// File: cmd/utils.go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// askForConfirmation prompts the user for a yes/no confirmation.
func askForConfirmation(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [y/n]: ", prompt)
		response, err := reader.ReadString('\n')
		if err != nil {
			return false
		}
		response = strings.ToLower(strings.TrimSpace(response))

		// FIX: Replaced if/else if chain with a switch statement for better readability.
		switch response {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
	}
}

// askForInput prompts the user for data input.
func askForInput(prompt string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s: ", prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

// askForSecretInput prompts the user for secret input (without echoing to the screen).
func askForSecretInput(prompt string) (string, error) {
	fmt.Printf("%s: ", prompt)
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	fmt.Println() // Add a newline after the input
	return strings.TrimSpace(string(bytePassword)), nil
}
