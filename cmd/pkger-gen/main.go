package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/markbates/pkger"
	"github.com/markbates/pkger/cmd/pkger/cmds"
	"github.com/markbates/pkger/parser"
)

func main() {
	info, err := pkger.Current()
	if err != nil {
		panic(err)
	}

	info.Module.Path = filepath.Join(info.Module.Path, "pkg/magician")
	info.Module.Dir = filepath.Join(info.Module.Dir, "pkg/magician")

	fp := filepath.Join(info.Dir, "pkg/magician/pkged.go")
	os.RemoveAll(fp)

	decls, err := parser.Parse(info, "/credential-helpers")
	if err != nil {
		panic(err)
	}

	if err := cmds.Package(info, fp, decls); err != nil {
		panic(err)
	}

	files, err := decls.Files()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Total number of files packed: %d\n", len(files))
	for _, f := range files {
		fmt.Printf(" - %s\n", f.Path.String())
	}
}
