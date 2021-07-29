package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v2"

	"github.com/docker-credential-magic/docker-credential-magic/pkg/types"
)

const (
	anonymousTokenResponse = "{\"Username\":\"\",\"Secret\":\"\"}"
)

var (
	// Version can be set via:
	// -ldflags="-X main.Version=$TAG"
	Version string

	errorInvalidDomain = errors.New("supplied domain is invalid")

	// TODO: should use existing cred helper/docker config if no match
	errorHelperNotFound = errors.New("could not determine correct helper")

	// TODO: should allow XDG
	mappingRootDirNotFound = errors.New("DOCKER_CREDENTIAL_MAGIC_CONFIG not set")
)

func main() {
	args := os.Args
	if len(args) < 2 {
		usage()
	}
	subcommand := args[1]
	if subcommand != "get" && subcommand != "version" {
		usage()
	}
	if subcommand == "version" {
		version()
	}

	// Assume subcommand is "get" here and read from stdin
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	rawInput := scanner.Text()

	domain, err := parseDomain(rawInput)
	if err != nil {
		//return err
		panic(err)
	}

	helperExe, err := getHelperExecutable(domain)
	if err != nil {
		if err == errorHelperNotFound {
			// Anonymous token
			fmt.Println(anonymousTokenResponse)
			return
		}
		panic(err)
	}

	cmd := exec.Command(helperExe, "get")
	cmd.Stdin = strings.NewReader(rawInput)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		panic(err)
	}
}

func usage() {
	fmt.Println("Usage: docker-credential-magic <get|version>")
	os.Exit(1)
}

func version() {
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
	var mappingRootDirAbsPath string
	mappingRootDir := os.Getenv("DOCKER_CREDENTIAL_MAGIC_CONFIG")
	if mappingRootDir == "" {
		mappingRootDir = filepath.Join(xdg.ConfigHome, "magic", "etc")
	}
	mappingRootDirAbsPath, err := filepath.Abs(mappingRootDir)
	if err != nil {
		return "", mappingRootDirNotFound
	}
	info, err := os.Stat(mappingRootDirAbsPath)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("mappings directory does not exist: %s", mappingRootDirAbsPath)
	}
	filepaths, err := ioutil.ReadDir(mappingRootDirAbsPath)
	if err != nil {
		return "", mappingRootDirNotFound
	}
	for _, fp := range filepaths {
		filename := filepath.Join(mappingRootDirAbsPath, fp.Name())
		b, err := ioutil.ReadFile(filename)
		if err != nil {
			return "", fmt.Errorf("unable to open %s: %v", filename, err)
		}
		var m types.HelperMapping
		err = yaml.Unmarshal(b, &m)
		if err != nil {
			return "", fmt.Errorf("parsing mappings for %s: %v", filename, err)
		}
		for _, d := range m.Domains {
			if d == domain {
				return fmt.Sprintf("docker-credential-%s", m.Helper), nil
			}
		}
	}
	return "", errorHelperNotFound
}
