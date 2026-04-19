package util

import "fmt"

// Successf prints a success message with green checkmark.
func Successf(format string, args ...any) {
	fmt.Printf("\033[32m✓\033[0m "+format+"\n", args...)
}

// Infof prints an info message with blue arrow.
func Infof(format string, args ...any) {
	fmt.Printf("\033[34m→\033[0m "+format+"\n", args...)
}

// Warnf prints a warning message with yellow exclamation.
func Warnf(format string, args ...any) {
	fmt.Printf("\033[33m!\033[0m "+format+"\n", args...)
}

// Errorf prints an error message with red X.
func Errorf(format string, args ...any) {
	fmt.Printf("\033[31m✗\033[0m "+format+"\n", args...)
}
