package main

import (
	"log"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/jdolitsky/docker-credential-magic/pkg/magician"
)

// Version can be set via:
// -ldflags="-X 'github.com/jdolitsky/docker-credential-magic/cmd/docker-credential-magician.Version=$TAG'"
var Version string

func init() {
	if Version == "" {
		i, ok := debug.ReadBuildInfo()
		if !ok {
			return
		}
		Version = i.Main.Version
	}
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "docker-credential-magician",
		Short: "Augment images with various credential helpers (including magic)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			var opts []magician.MagicOption
			return magician.Abracadabra(ref, opts...)
		},
	}
	if err := rootCmd.Execute(); err != nil {
		log.Fatalln(err.Error())
		os.Exit(1)
	}
}
