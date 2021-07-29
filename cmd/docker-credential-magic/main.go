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

	"gopkg.in/yaml.v2"

	"github.com/docker-credential-magic/docker-credential-magic/pkg/types"
)


const (
	anonymousTokenResponse = "{\"Username\":\"\",\"Secret\":\"\"}"
)

var (
	errorInvalidDomain = errors.New("supplied domain is invalid")

	// TODO: should use existing cred helper/docker config if no match
	errorHelperNotFound = errors.New("could not determine correct helper")

	// TODO: should allow XDG
	mappingRootDirNotFound = errors.New("DOCKER_CREDENTIAL_MAGIC_CONFIG not set")
)

func main() {
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
	mappingRootDir := os.Getenv("DOCKER_CREDENTIAL_MAGIC_CONFIG")
	if mappingRootDir == "" {
		// TODO: allow read from XDG
		return "", mappingRootDirNotFound
	}
	mappingRootDirAbsPath, err := filepath.Abs(mappingRootDir)
	if err != nil {
		return "", mappingRootDirNotFound
	}
	filepaths, err := ioutil.ReadDir(mappingRootDirAbsPath)
	if err != nil {
		return "", mappingRootDirNotFound
	}
	for _, filepath := range filepaths {
		filename := filepath.Name()
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
