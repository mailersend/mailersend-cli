package prompt

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
)

func IsInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func Input(label, placeholder string) (string, error) {
	var value string
	err := huh.NewInput().
		Title(label).
		Placeholder(placeholder).
		Value(&value).
		Run()
	return strings.TrimSpace(value), err
}

func Confirm(label string) (bool, error) {
	var value bool
	err := huh.NewConfirm().
		Title(label).
		Value(&value).
		Run()
	return value, err
}

func Select(label string, options []string) (string, error) {
	var value string
	opts := make([]huh.Option[string], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, o)
	}
	err := huh.NewSelect[string]().
		Title(label).
		Options(opts...).
		Value(&value).
		Run()
	return value, err
}

func RequireArg(value, flag, label string) (string, error) {
	if value != "" {
		return value, nil
	}
	if !IsInteractive() {
		return "", fmt.Errorf("--%s is required", flag)
	}
	return Input(label, "")
}
