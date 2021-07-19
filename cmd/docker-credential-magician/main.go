package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/jdolitsky/docker-credential-magic/pkg/magician"
)

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
