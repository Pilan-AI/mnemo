//go:build ignore

package mcp

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const ONBOARDING_COMPLETE_FLAG = "~/.mnemo/.onboarding-complete"

// Onboarding handles first-time user setup
type Onboarding struct{}

// RunOnboarding starts the interactive onboarding process
func (o *Onboarding) RunOnboarding() error {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║   Welcome to Mnemo!                                       ║")
	fmt.Println("║   Your AI conversation memory system                      ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("This onboarding will help you set up Mnemo context injection.")
	fmt.Println()

	mode, err := o.askModePreference()
	if err != nil {
		return err
	}

	if err := o.saveConfig(mode); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	fmt.Println("✓ Configuration saved!")
	fmt.Println()

	if err := o.markOnboardingComplete(); err != nil {
		return fmt.Errorf("failed to mark onboarding complete: %w", err)
	}

	fmt.Println("✓ Onboarding complete!")
	fmt.Println()
	fmt.Println("You can configure your injection mode later by running:")
	fmt.Println("  mnemo configure <mode>")
	fmt.Println()
	fmt.Println("Available modes:")
	fmt.Println("  - off      No auto-injection")
	fmt.Println("  - helper   Keyword-based (CODE/DEBUG only)")
	fmt.Println("  - assistant Always inject context")
	fmt.Println()

	return nil
}

func (o *Onboarding) askModePreference() (string, error) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("Choose your preferred injection mode:")
		fmt.Println("  1. helper (recommended) - Inject only for code/debug tasks")
		fmt.Println("  2. assistant - Inject context for every message")
		fmt.Println("  3. off - Disable auto-injection")
		fmt.Println()
		fmt.Print("Select mode (1-3): ")

		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "1", "helper":
			return "helper", nil
		case "2", "assistant":
			return "assistant", nil
		case "3", "off":
			return "off", nil
		default:
			fmt.Println()
			fmt.Println("Invalid selection. Please try again.")
			fmt.Println()
		}
	}
}

func (o *Onboarding) saveConfig(mode string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	mnemoDir := filepath.Join(homeDir, ".mnemo")
	if err := os.MkdirAll(mnemoDir, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(mnemoDir, "config.json")

	content := fmt.Sprintf(`{
  "injection_mode": "%s"
}`, mode)

	return os.WriteFile(configPath, []byte(content), 0644)
}

func (o *Onboarding) markOnboardingComplete() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	flagPath := filepath.Join(homeDir, ONBOARDING_COMPLETE_FLAG)

	return os.WriteFile(flagPath, []byte("completed"), 0644)
}

// CheckOnboardingCompleted checks if onboarding has been completed
func (o *Onboarding) CheckOnboardingCompleted() bool {
	if _, err := os.Stat(ONBOARDING_COMPLETE_FLAG); err == nil {
		return true
	}

	return false
}

// IsOnboardingRequired determines if onboarding should be shown
func (o *Onboarding) IsOnboardingRequired() bool {
	return !o.CheckOnboardingCompleted()
}

// GetOnboardingSummary returns a summary of the user's configuration
func (o *Onboarding) GetOnboardingSummary() (string, error) {
	mode := loadInjectionMode()

	var modeDescription string
	switch mode {
	case InjectionModeOff:
		modeDescription = "Off - No auto-injection"
	case InjectionModeHelper:
		modeDescription = "Helper - Code/debug only"
	case InjectionModeAssistant:
		modeDescription = "Assistant - Every message"
	default:
		modeDescription = "Auto (default)"
	}

	return fmt.Sprintf("Current mode: %s", modeDescription), nil
}

// ResetOnboarding clears the onboarding completion flag
func (o *Onboarding) ResetOnboarding() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	flagPath := filepath.Join(homeDir, ONBOARDING_COMPLETE_FLAG)

	if _, err := os.Stat(flagPath); err == nil {
		return os.Remove(flagPath)
	}

	return nil
}
