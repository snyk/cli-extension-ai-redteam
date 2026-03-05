package main

import (
	"log"
	"os"

	"github.com/rs/zerolog"
	"github.com/snyk/go-application-framework/pkg/devtools"

	"github.com/snyk/cli-extension-ai-redteam/pkg/redteam"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	for _, arg := range os.Args[1:] {
		if arg == "--debug" || arg == "-d" {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
			break
		}
	}

	cmd, err := devtools.Cmd(redteam.Init)
	if err != nil {
		log.Fatal(err)
	}
	cmd.SilenceUsage = true
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
