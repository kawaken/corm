package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const cormfilename = "Cormfile"

var errCannotParseLine = errors.New("cannot parse line")

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

func mainCmd() int {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot get current directory")
		return 1
	}

	cormfile := filepath.Join(dir, cormfilename)
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

	vendorDir := filepath.Join(dir, "_vendor")
	gopath := os.Getenv("GOPATH")
	os.Setenv("GOPATH", fmt.Sprintf("%s:%s", vendorDir, gopath))

	for _, repo := range repos {
		err := goGet(repo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot exec go get: %s, %s\n", repo.Path, err)
		}
	}

	return 0
}

func main() {
	os.Exit(mainCmd())
}
