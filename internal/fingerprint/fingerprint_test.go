package fingerprint

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestFilesIsOrderIndependent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := Files(root, []string{"a.txt", "b.txt"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Files(root, []string{"b.txt", "a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("fingerprints differ: %q != %q", first, second)
	}
}

func TestRecordsNormalizesTextButPreservesBinary(t *testing.T) {
	lf := Records([]Record{{Path: "file.txt", Kind: "file", Data: []byte("a\nb\n")}})
	crlf := Records([]Record{{Path: "file.txt", Kind: "file", Data: []byte("a\r\nb\r\n")}})
	if lf != crlf {
		t.Fatalf("line endings changed fingerprint: %q != %q", lf, crlf)
	}
	binaryLF := Records([]Record{{Path: "file.bin", Kind: "file", Data: []byte{'a', 0, '\n'}}})
	binaryCRLF := Records([]Record{{Path: "file.bin", Kind: "file", Data: []byte{'a', 0, '\r', '\n'}}})
	if binaryLF == binaryCRLF {
		t.Fatal("binary records with NUL should remain byte-sensitive")
	}
}

func TestFilesHashesSymlinkTargetWithoutFollowingIt(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink("missing-target", link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	got, err := Files(root, []string{"link"})
	if err != nil {
		t.Fatal(err)
	}
	want := Records([]Record{{Path: "link", Kind: "symlink", Data: []byte("missing-target")}})
	if got != want {
		t.Fatalf("symlink fingerprint = %q, want %q", got, want)
	}
}

func TestRecordsDistinguishesMissingAndGitlink(t *testing.T) {
	missing := Records([]Record{{Path: "module", Kind: "missing"}})
	gitlinkA := Records([]Record{{Path: "module", Kind: "gitlink", Data: []byte("a")}})
	gitlinkB := Records([]Record{{Path: "module", Kind: "gitlink", Data: []byte("b")}})
	if missing == gitlinkA || gitlinkA == gitlinkB {
		t.Fatal("record kinds or gitlink commits must affect fingerprints")
	}
	if bytes.Equal([]byte(gitlinkA), []byte(gitlinkB)) {
		t.Fatal("gitlink fingerprints unexpectedly equal")
	}
}
