package main

import "os"

func main() {
	if len(os.Args) < 2 {
		panic("usage: docker-credential-magician <ref>")
	}
	ref := os.Args[1]

	err := Abracadabra(ref)
	if err != nil {
		panic(err)
	}
}
