package main

import (
	"fmt"
	"os"

	qt "github.com/mappu/miqt/qt6"

	"github.com/LeiSureLyYrsc/mclauncher-go/internal/ui"
)

func main() {
	qt.NewQApplication(os.Args)
	if err := ui.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
