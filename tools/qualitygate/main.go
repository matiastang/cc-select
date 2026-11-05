package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "qualitygate: 需要子命令（filelines | filter | ratchet | coverage）")
	os.Exit(2)
}
