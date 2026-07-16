package fingerprint

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"unicode/utf8"
)

type Record struct {
	Path string
	Kind string
	Data []byte
}

func Files(root string, paths []string) (string, error) {
	records := make([]Record, 0, len(paths))
	for _, relative := range paths {
		clean := filepath.Clean(relative)
		full := filepath.Join(root, clean)
		info, err := os.Lstat(full)
		if err != nil {
			return "", err
		}
		var kind string
		var data []byte
		switch {
		case info.Mode().IsRegular():
			kind = "file"
			data, err = os.ReadFile(full)
		case info.Mode()&os.ModeSymlink != 0:
			kind = "symlink"
			var target string
			target, err = os.Readlink(full)
			data = []byte(filepath.ToSlash(target))
		default:
			err = errors.New("unsupported evidence file type")
		}
		if err != nil {
			return "", err
		}
		records = append(records, Record{
			Path: filepath.ToSlash(clean),
			Kind: kind,
			Data: data,
		})
	}
	return Records(records), nil
}

func Records(records []Record) string {
	ordered := append([]Record(nil), records...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Path == ordered[j].Path {
			return ordered[i].Kind < ordered[j].Kind
		}
		return ordered[i].Path < ordered[j].Path
	})
	hash := sha256.New()
	for _, record := range ordered {
		data := record.Data
		if utf8.Valid(data) && !bytes.ContainsRune(data, '\x00') {
			data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
		}
		fmt.Fprintf(hash, "%s\x00%s\x00%d\x00", record.Path, record.Kind, len(data))
		_, _ = hash.Write(data)
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}
