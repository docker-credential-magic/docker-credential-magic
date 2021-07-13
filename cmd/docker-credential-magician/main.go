package main

func main() {
	// Parse command line args/settings
	config := parseConfig()

	// Pull the remote image
	base := pullBaseImage(config.OrigRef)

	// Create a new tag, the original suffixed with ".magic"
	tag := createTag(config.NewRef)

	// Append credential helper binaries
	img := appendCredentialHelpers(base)

	// Save image locally
	pushImageToLocalDaemon(tag, img)
}
