package gitrepo

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Repo struct {
	Root string
}

func Discover(start string) (Repo, error) {
	cmd := exec.Command("git", "-C", start, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return Repo{}, errors.New("not inside a Git repository")
	}
	root, err := filepath.Abs(strings.TrimSpace(string(out)))
	if err != nil {
		return Repo{}, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return Repo{}, err
	}
	return Repo{Root: root}, nil
}

func (r Repo) Output(args ...string) (string, error) {
	return r.OutputInput(nil, args...)
}

func (r Repo) OutputInput(input []byte, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", r.Root}, args...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", errors.New(strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (r Repo) Head() (string, error) {
	head, err := r.Output("rev-parse", "--verify", "HEAD")
	if err == nil {
		return head, nil
	}
	return r.OutputInput(nil, "hash-object", "-t", "tree", "--stdin")
}

func (r Repo) WorktreeID() (string, error) {
	path, err := r.Output("rev-parse", "--git-path", ".")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.Root, path)
	}
	return filepath.Abs(path)
}

func (r Repo) ChangedPaths(base string) ([]string, error) {
	committedAndTracked, err := r.Output("diff", "--name-only", base, "--")
	if err != nil {
		return nil, err
	}
	untracked, err := r.Output("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	unique := map[string]struct{}{}
	for _, output := range []string{committedAndTracked, untracked} {
		for _, path := range strings.Split(output, "\n") {
			if path != "" {
				unique[filepath.ToSlash(path)] = struct{}{}
			}
		}
	}
	paths := make([]string, 0, len(unique))
	for path := range unique {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

func (r Repo) WorktreePatch(excludedPaths ...string) (string, error) {
	head, err := r.Head()
	if err != nil {
		return "", err
	}
	args := []string{
		"diff", "--binary", "--no-ext-diff", "--no-textconv", "--submodule=diff",
		head, "--", ".",
	}
	args = append(args, exclusionPathspecs(excludedPaths...)...)
	patch, err := r.Output(args...)
	if err != nil {
		return "", err
	}
	untracked, err := r.UntrackedPaths(excludedPaths...)
	if err != nil {
		return "", err
	}
	for _, relative := range untracked {
		full := filepath.Join(r.Root, filepath.FromSlash(relative))
		info, statErr := os.Lstat(full)
		if statErr != nil {
			return "", statErr
		}
		var data []byte
		switch {
		case info.Mode().IsRegular():
			data, statErr = os.ReadFile(full)
		case info.Mode()&os.ModeSymlink != 0:
			var target string
			target, statErr = os.Readlink(full)
			data = []byte(target)
		default:
			continue
		}
		if statErr != nil {
			return "", statErr
		}
		patch += fmt.Sprintf("\n--- untracked: %s ---\n%s", relative, data)
	}
	return patch, nil
}

func (r Repo) UntrackedPaths(excludedPaths ...string) ([]string, error) {
	args := []string{"ls-files", "--others", "--exclude-standard", "--", "."}
	args = append(args, exclusionPathspecs(excludedPaths...)...)
	out, err := r.Output(args...)
	if err != nil || out == "" {
		return nil, err
	}
	paths := strings.Split(out, "\n")
	sort.Strings(paths)
	return paths, nil
}

func exclusionPathspecs(paths ...string) []string {
	var result []string
	for _, path := range paths {
		slash := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(path)), "/")
		result = append(
			result,
			":(top,exclude,literal)"+slash,
			":(top,exclude,glob)"+slash+"/**",
		)
	}
	return result
}
