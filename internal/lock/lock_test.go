package lock

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestLeasePersistsUntilOwnerReleasesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wiki.lock")
	first, err := Acquire(context.Background(), path, "session-1", 100*time.Millisecond, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = Acquire(context.Background(), path, "session-2", 20*time.Millisecond, time.Minute); err == nil {
		t.Fatal("Acquire() error = nil, want timeout")
	}

	sameOwner, err := Acquire(context.Background(), path, "session-1", 20*time.Millisecond, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if sameOwner.Owner() != "session-1" {
		t.Fatalf("Owner() = %q", sameOwner.Owner())
	}

	if err := first.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(context.Background(), path, "session-2", 100*time.Millisecond, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Release(context.Background())
}

func TestExpiredLeaseCanBeReplaced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wiki.lock")
	if err := writeRecord(path, record{Owner: "old", ExpiresAt: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	lease, err := Acquire(context.Background(), path, "new", 100*time.Millisecond, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release(context.Background())
	owner, err := CurrentOwner(context.Background(), path)
	if err != nil || owner != "new" {
		t.Fatalf("CurrentOwner() = %q, %v", owner, err)
	}
}

func TestReleaseRefusesWrongOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wiki.lock")
	lease, err := Acquire(context.Background(), path, "session-1", 100*time.Millisecond, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	wrong := &Lease{path: path, owner: "session-2"}
	if err := wrong.Release(context.Background()); err == nil {
		t.Fatal("wrong-owner Release() error = nil")
	}
	if owner, err := CurrentOwner(context.Background(), path); err != nil || owner != "session-1" {
		t.Fatalf("lease changed after wrong-owner release: %q, %v", owner, err)
	}
	if err := lease.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
}
