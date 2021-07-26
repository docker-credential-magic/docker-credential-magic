package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/docker-credential-magic/docker-credential-magic/pkg/magician"
)

type settings struct {
	Tag string
}

func main() {
	var s settings

	rootCmd := &cobra.Command{
		Use:   "docker-credential-magician",
		Short: "Augment images with various credential helpers (including magic)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			var opts []magician.MagicOption
			if tag := s.Tag; tag != "" {
				opts = append(opts, magician.MagicOptWithTag(tag))
			}
			return magician.Abracadabra(ref, opts...)
		},
	}

	rootCmd.Flags().StringVarP(&s.Tag, "tag", "t", "", "push to custom location")

	if err := rootCmd.Execute(); err != nil {
		log.Fatalln(err.Error())
		os.Exit(1)
	}
}
