package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)


const (
	helperACREnv   = "acr-env"
	helperECRLogin = "ecr-login"
	helperGCR      = "gcr"

	anonymousTokenResponse = "{\"Username\":\"\",\"Secret\":\"\"}"
)

var (
	// Root domains mapped to helpers that support auth for them
	domainHelperMap = map[string]string{
		"amazonaws.com": helperECRLogin,
		"azurecr.io":    helperACREnv,
		"gcr.io":        helperGCR,
		"pkg.dev":       helperGCR,
	}

	errorInvalidDomain = errors.New("supplied domain is invalid")

	// TODO: should use existing cred helper/docker config if no match
	errorHelperNotFound = errors.New("could not determine correct helper")
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
	helper, ok := domainHelperMap[domain]
	if !ok {
		return "", errorHelperNotFound
	}
	return fmt.Sprintf("docker-credential-%s", helper), nil
}
