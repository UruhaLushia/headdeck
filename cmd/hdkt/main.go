package main

import (
	"os"

	"github.com/UruhaLushia/headdeck/internal/command"
)

func main() {
	os.Exit(command.Execute())
}
