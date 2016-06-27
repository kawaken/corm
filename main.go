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

var errCannotParseLine = errors.New("cannot parse line")

var vcsMetaDir = []string{".svn", ".git", ".hg"}

type repository struct {
	Path   string
	Commit string
}

func parse(s string) (*repository, error) {
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
	fmt.Printf("go get %s\n", repo.Path)
	err := exec.Command("go", "get", repo.Path).Run()
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

func paths() (string, string, error) {
	cur, err := os.Getwd()
	if err != nil {
		return "", "", err
	}

	dirty := filepath.Join(cur, cormDir)
	return cur, dirty, err
}

func mainCmd() int {
	curdir, dirtyVendorDir, err := paths()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot get directory")
		return 1
	}

	cormfile := filepath.Join(curdir, cormFilename)
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

	gopath := os.Getenv("GOPATH")
	os.Setenv("GOPATH", fmt.Sprintf("%s:%s", dirtyVendorDir, gopath))

	for _, repo := range repos {
		err := goGet(repo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot exec go get: %s, %s\n", repo.Path, err)
		}
	}

	return 0
}

func exportCmd() int {
	curdir, dirtyVendorDir, err := paths()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot get directory")
		return 1
	}

	dirtyVendorSrcDir := filepath.Join(dirtyVendorDir, "src")
	cleanVendorDir := filepath.Join(curdir, "vendor")
	err = export(dirtyVendorSrcDir, cleanVendorDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "export error:", err)
	}

	return 0
}

func main() {
	sort.Strings(vcsMetaDir)
	os.Exit(mainCmd())
}
