package term

import "fmt"

func Infof(format string, args ...interface{})    { fmt.Printf("  "+format+"\n", args...) }
func Successf(format string, args ...interface{}) { fmt.Printf("✓ "+format+"\n", args...) }
func Errorf(format string, args ...interface{})   { fmt.Printf("✗ "+format+"\n", args...) }
func Warnf(format string, args ...interface{})    { fmt.Printf("⚠ "+format+"\n", args...) }
func Printf(format string, args ...interface{})   { fmt.Printf(format+"\n", args...) }
