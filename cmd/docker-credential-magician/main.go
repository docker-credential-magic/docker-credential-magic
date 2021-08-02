package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/docker-credential-magic/docker-credential-magic/pkg/magician"
)

type mutateSettings struct {
	Tag            string
	HelpersDir     string
	MappingsDir    string
	IncludeHelpers []string
}

// Version can be set via:
// -ldflags="-X main.Version=$TAG"
var Version string

func main() {
	var mutate mutateSettings

	rootCmd := &cobra.Command{
		Use:   "docker-credential-magician",
		Short: "Augment images with various credential helpers (including magic)",
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version and exit",
		RunE: func(cmd *cobra.Command, args []string) error {
			if Version == "" {
				fmt.Println("could not determine build information")
			} else {
				fmt.Println(Version)
			}
			return nil
		},
	}
	rootCmd.AddCommand(versionCmd)

	mutateCmd := &cobra.Command{
		Use:   "mutate",
		Short: "Augment an image with one or more credential helpers",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			opts := []magician.MutateOption{
				magician.MutateOptWithWriter(os.Stdout),
				magician.MutateOptWithUserAgent(
					fmt.Sprintf("docker-credential-magician/%s", Version)),
			}
			if tag := mutate.Tag; tag != "" {
				opts = append(opts, magician.MutateOptWithTag(tag))
			}
			if helpersDir := mutate.HelpersDir; helpersDir != "" {
				opts = append(opts, magician.MutateOptWithHelpersDir(helpersDir))
			}
			if mappingsDir := mutate.MappingsDir; mappingsDir != "" {
				opts = append(opts, magician.MutateOptWithMappingsDir(mappingsDir))
			}
			if len(mutate.IncludeHelpers) > 0 {
				opts = append(opts, magician.MutateOptWithIncludeHelpers(mutate.IncludeHelpers))
			}
			return magician.Mutate(ref, opts...)
		},
	}
	mutateCmd.Flags().StringVarP(&mutate.Tag, "tag", "t", "", "push to custom location")
	mutateCmd.Flags().StringVarP(&mutate.HelpersDir, "helpers-dir", "", "",
		"path containing helpers")
	mutateCmd.Flags().StringVarP(&mutate.MappingsDir, "mappings-dir", "", "",
		"path containing mappings")
	mutateCmd.Flags().StringArrayVarP(&mutate.IncludeHelpers, "include", "i",
		[]string{}, "custom helpers to include")

	rootCmd.AddCommand(mutateCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatalln(err.Error())
		os.Exit(1)
	}
}
