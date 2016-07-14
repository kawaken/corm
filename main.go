package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	cormFilename = "Cormfile"
	cormDir      = "_corm"
)

var (
	currentDir     string
	dirtyVendorDir string
)

var errCannotParseLine = errors.New("cannot parse line")

var vcsMetaDir = []string{".svn", ".git", ".hg"}

type repository struct {
	Path   string
	Commit string
}

func parse(s string) (*repository, error) {
	// TODO: Consider the format
	fields := strings.Fields(s)
	switch len(fields) {
	case 1:
		return &repository{Path: fields[0]}, nil
	case 2:
		return &repository{Path: fields[0], Commit: fields[1]}, nil
	default:
		return nil, errCannotParseLine
	}
}

func exists(filepath string) bool {
	_, err := os.Stat(filepath)
	return err == nil
}

func readCorm(filepath string) ([]*repository, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("cannot open Cormfile: %s", err)
	}
	defer f.Close()

	repos := make([]*repository, 0, 30)

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if line == "" {
			continue
		}

		repo, err := parse(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "SKIPPED: %s: %s\n", err, line)
			continue
		}
		repos = append(repos, repo)
	}

	return repos, nil
}

func goGet(repo *repository) error {
	// TODO: can handle options for `go get`
	fmt.Printf("go get %s\n", repo.Path)
	cmd := exec.Command("go", "get", repo.Path)
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	// Commit が指定されている場合の処理は後で実装する
	return err
}

func newCopyFileFun(srcBase string, destBase string) filepath.WalkFunc {

	return func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcBase, srcPath)
		if err != nil {
			return err
		}

		destPath := filepath.Join(destBase, relPath)

		if info.IsDir() {
			index := sort.SearchStrings(vcsMetaDir, info.Name())
			if index < len(vcsMetaDir) {
				return filepath.SkipDir
			}

			return os.MkdirAll(destPath, info.Mode())
		}

		if exists(destPath) {
			return nil
		}

		return os.Link(srcPath, destPath)
	}

}

func export(src string, dest string) error {
	copyFile := newCopyFileFun(src, dest)
	err := filepath.Walk(src, copyFile)
	return err
}

func init() {
	var err error
	currentDir, err = os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot get current directory")
		os.Exit(1)
	}

	dirtyVendorDir = filepath.Join(currentDir, cormDir)
}

func fakeGopath() {

	// if target package exists default GOPATH, then go get does nothing.
	// So, set GOPATH dirtyVendorDir only

	os.Setenv("GOPATH", dirtyVendorDir)

}

func mainCmd() int {
	cormfile := filepath.Join(currentDir, cormFilename)
	if !exists(cormfile) {
		fmt.Fprintf(os.Stderr, "%s does not exists\n", cormfile)
		return 1
	}

	repos, err := readCorm(cormfile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if len(repos) == 0 {
		fmt.Fprintln(os.Stderr, "no repositories in Cormfile")
		return 1
	}

	fakeGopath()
	for _, repo := range repos {
		err := goGet(repo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot exec go get: %s, %s\n", repo.Path, err)
		}
	}

	return 0
}

func exportCmd() int {
	dirtyVendorSrcDir := filepath.Join(dirtyVendorDir, "src")

	if !exists(dirtyVendorDir) {
		fmt.Fprintf(os.Stderr, "%s does not exists\n", dirtyVendorSrcDir)
		return 1
	}

	cleanVendorDir := filepath.Join(currentDir, "vendor")
	err := export(dirtyVendorSrcDir, cleanVendorDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "export error:", err)
		return 1
	}

	return 0
}

func execCmd(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "no exec target command")
		return 1
	}

	fakeGopath()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout

	// TODO: cant exec vim main.go
	err := cmd.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "exec error:", err)
		return 1
	}
	return 0
}

func usage() int {
	fmt.Println(`Usage: corm command
	install	:	install packages from Cormfile.
	export	:	export packages to vendor directory.`)

	return 1
}

func main() {
	sort.Strings(vcsMetaDir)

	if len(os.Args) == 1 {
		os.Exit(usage())
	}

	command := os.Args[1]

	switch command {
	case "install":
		os.Exit(mainCmd())
	case "export":
		os.Exit(exportCmd())
	case "exec":
		os.Exit(execCmd(os.Args[2:]))
	default:
		os.Exit(usage())
	}
}
