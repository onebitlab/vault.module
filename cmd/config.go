// File: cmd/config.go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"vault.module/internal/colors"
	"vault.module/internal/errors"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Shows the contents of the configuration file.",
	Long: `Shows the contents of the configuration file.

This command displays the raw contents of config.json file.
If jq or python3 is available, it will use them for better formatting.

Examples:
  vault.module config
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.WrapCommand(func() error {
			// Try to use external formatter first
			if externalOutput := tryExternalFormatter(); externalOutput != "" {
				fmt.Println(colors.SafeColor("Configuration file contents:", colors.Bold))
				fmt.Println(externalOutput)
				return nil
			}

			// Read the config.json file
			configData, err := os.ReadFile("config.json")
			if err != nil {
				return errors.NewFileSystemError("read", "config.json", err)
			}

			// Parse JSON for pretty printing
			var jsonData interface{}
			if err := json.Unmarshal(configData, &jsonData); err != nil {
				// If JSON is invalid, just print raw content
				fmt.Println(colors.SafeColor("Configuration file contents:", colors.Bold))
				fmt.Println(string(configData))
				return nil
			}

			// Pretty print JSON
			prettyJSON, err := json.MarshalIndent(jsonData, "", "  ")
			if err != nil {
				return errors.New(errors.ErrCodeInternal, "failed to format JSON").WithContext("marshal_error", err.Error())
			}

			fmt.Println(colors.SafeColor("Configuration file contents:", colors.Bold))
			fmt.Println(string(prettyJSON))

			return nil
		})
	},
}

// tryExternalFormatter attempts to format JSON using external tools
func tryExternalFormatter() string {
	// Try jq first
	if jqOutput := tryJq(); jqOutput != "" {
		return jqOutput
	}

	// Try Python as fallback
	if pythonOutput := tryPython(); pythonOutput != "" {
		return pythonOutput
	}

	return ""
}

// tryJq attempts to format JSON using jq
func tryJq() string {
	// Check if jq is available
	if _, err := exec.LookPath("jq"); err != nil {
		return ""
	}

	// Try to format with jq
	cmd := exec.Command("jq", ".", "config.json")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return string(output)
}

// tryPython attempts to format JSON using Python
func tryPython() string {
	// Check if python3 is available
	if _, err := exec.LookPath("python3"); err != nil {
		return ""
	}

	// Try to format with Python
	cmd := exec.Command("python3", "-m", "json.tool", "config.json")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return string(output)
}
