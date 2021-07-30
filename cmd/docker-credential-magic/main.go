package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v2"

	"github.com/docker-credential-magic/docker-credential-magic/internal/constants"
	"github.com/docker-credential-magic/docker-credential-magic/internal/embedded/mappings"
	"github.com/docker-credential-magic/docker-credential-magic/internal/types"
)

var (
	// Version can be set via:
	// -ldflags="-X main.Version=$TAG"
	Version string

	errorInvalidDomain = errors.New("supplied domain is invalid")

	// TODO: should use existing cred helper/docker config if no match
	errorHelperNotFound = errors.New("could not determine correct helper")

	validHelper = regexp.MustCompile(`^[a-z0-9_-].*?$`)
)

func main() {
	args := os.Args
	if len(args) < 2 {
		usage()
	}
	subcommand := args[1]
	switch subcommand {
	case constants.HelperSubcommandGet:
		subcommandGet()
	case "env":
		subcommandEnv()
	case "init":
		subcommandInit()
	case "version":
		subcommandVersion()
	}
	usage()
}

func usage() {
	fmt.Printf("Usage: docker-credential-magic <%s|env|init|version>\n",
		constants.HelperSubcommandGet)
	os.Exit(1)
}

func subcommandGet() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	rawInput := scanner.Text()
	domain, err := parseDomain(rawInput)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	helperExe, err := getHelperExecutable(domain)
	if err != nil {
		if err == errorHelperNotFound {
			fmt.Print(constants.AnonymousTokenResponse)
			os.Exit(0)
		}
		fmt.Println(err.Error())
		os.Exit(1)
	}
	cmd := exec.Command(helperExe, constants.HelperSubcommandGet)
	cmd.Stdin = strings.NewReader(rawInput)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}

func subcommandEnv() {
	dockerCredentialMagicConfig := getDockerCredentialMagicConfig()
	fmt.Printf("%s=\"%s\"\n",
		constants.EnvVarDockerCredentialMagicConfig,
		dockerCredentialMagicConfig)
	os.Exit(0)
}

func subcommandInit() {
	dockerCredentialMagicConfig := getDockerCredentialMagicConfig()
	parentDir := filepath.Join(dockerCredentialMagicConfig, constants.MappingsSubdir)
	parentDirAbs, err := filepath.Abs(parentDir)
	if err != nil {
		fmt.Printf("Error: '%s' is not a valid directory\n", dockerCredentialMagicConfig)
		os.Exit(1)
	}
	if info, err := os.Stat(parentDirAbs); err == nil && info.IsDir() {
		fmt.Printf("Directory '%s' already exists. Skipping.\n", parentDirAbs)
	} else {
		fmt.Printf("Creating directory '%s' ...\n", parentDirAbs)
		if err := os.MkdirAll(parentDirAbs, 0755); err != nil {
			fmt.Printf("Error creating directory: %s\n", err.Error())
			os.Exit(1)
		}
	}
	items, err := mappings.Embedded.ReadDir(constants.EmbeddedParentDir)
	if err != nil {
		fmt.Printf("Error reading embedded directory: %s\n", err.Error())
		os.Exit(1)
	}
	for _, item := range items {
		filename := filepath.Join(parentDirAbs, item.Name())
		if _, err := os.Stat(filename); err == nil {
			fmt.Printf("File '%s' already exists. Skipping.\n", filename)
			continue
		}
		fmt.Printf("Creating mapping file '%s' ...\n", filename)
		embeddedName := filepath.Join(constants.EmbeddedParentDir, item.Name())
		file, err := mappings.Embedded.Open(embeddedName)
		if err != nil {
			fmt.Printf("Error loading embedded file %s: %s\n", embeddedName, err.Error())
			os.Exit(1)
		}
		defer file.Close()
		b, err := ioutil.ReadAll(file)
		if err != nil {
			fmt.Printf("Error reading embedded file %s: %s\n", embeddedName, err.Error())
			os.Exit(1)
		}
		if err = ioutil.WriteFile(filename, b, 0644); err != nil {
			fmt.Printf("Error writing embedded file %s: %s\n", embeddedName, err.Error())
			os.Exit(1)
		}
	}
	os.Exit(0)
}

func subcommandVersion() {
	fmt.Println(Version)
	os.Exit(0)
}

func parseDomain(s string) (string, error) {
	parts := strings.Split(s, ".")
	numParts := len(parts)
	if numParts < 2 {
		return "", errorInvalidDomain
	}
	root := parts[numParts-2]
	ext := parts[numParts-1]
	if root == "" || ext == "" {
		return "", errorInvalidDomain
	}
	domain := strings.Join([]string{root, ext}, ".")
	return domain, nil
}

func getHelperExecutable(domain string) (string, error) {
	dockerCredentialMagicConfig := getDockerCredentialMagicConfig()
	parentDir := filepath.Join(dockerCredentialMagicConfig, constants.MappingsSubdir)
	parentDirAbs, err := filepath.Abs(parentDir)
	if err != nil {
		return "", fmt.Errorf("'%s' is not a valid directory", dockerCredentialMagicConfig)
	}
	notExistsErr := fmt.Errorf(
		"Directory '%s' does not exist.\nHint: Try running \"docker-credential-magic init\"",
		parentDirAbs)
	if info, err := os.Stat(parentDirAbs); err != nil || !info.IsDir() {
		return "", notExistsErr
	}
	items, err := ioutil.ReadDir(parentDirAbs)
	if err != nil {
		return "", notExistsErr
	}
	for _, item := range items {
		filename := filepath.Join(parentDirAbs, item.Name())
		b, err := ioutil.ReadFile(filename)
		if err != nil {
			return "", fmt.Errorf("unable to open '%s': %v", filename, err)
		}
		var m types.HelperMapping
		err = yaml.Unmarshal(b, &m)
		if err != nil {
			return "", fmt.Errorf("parsing mappings for '%s': %v", filename, err)
		}
		if !validHelper.MatchString(m.Helper) {
			return "", fmt.Errorf("helper '%s' is invalid", m.Helper)
		}
		for _, d := range m.Domains {
			if d == domain {
				return fmt.Sprintf("docker-credential-%s", m.Helper), nil
			}
		}
	}
	return "", errorHelperNotFound
}

func getDockerCredentialMagicConfig() string {
	if d := os.Getenv(constants.EnvVarDockerCredentialMagicConfig); d != "" {
		return d
	}
	return filepath.Join(xdg.ConfigHome, constants.XDGConfigSubdir)
}
