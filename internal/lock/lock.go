package lock

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/merdandt/LLM-wiki-dev/internal/atomicfile"
)

type record struct {
	Owner      string    `json:"owner"`
	AcquiredAt time.Time `json:"acquired_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type Lease struct {
	path  string
	owner string
}

func Acquire(ctx context.Context, path, owner string, wait, ttl time.Duration) (*Lease, error) {
	if owner == "" {
		return nil, errors.New("lease owner is required")
	}
	if ttl <= 0 {
		return nil, errors.New("lease ttl must be positive")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		mutex, err := acquireMutex(ctx, path+".mutex", time.Until(deadline))
		if err != nil {
			return nil, err
		}
		now := time.Now().UTC()
		current, readErr := readRecord(path)
		switch {
		case errors.Is(readErr, os.ErrNotExist):
		case readErr != nil:
			_ = mutex.Unlock()
			return nil, readErr
		case current.Owner == owner:
			current.ExpiresAt = now.Add(ttl)
			if err := writeRecord(path, current); err != nil {
				_ = mutex.Unlock()
				return nil, err
			}
			_ = mutex.Unlock()
			return &Lease{path: path, owner: owner}, nil
		case current.ExpiresAt.After(now):
			_ = mutex.Unlock()
			if time.Now().After(deadline) {
				return nil, errors.New("wiki synchronization lease timeout")
			}
			if err := waitForRetry(ctx, 25*time.Millisecond); err != nil {
				return nil, err
			}
			continue
		}
		next := record{Owner: owner, AcquiredAt: now, ExpiresAt: now.Add(ttl)}
		if err := writeRecord(path, next); err != nil {
			_ = mutex.Unlock()
			return nil, err
		}
		_ = mutex.Unlock()
		return &Lease{path: path, owner: owner}, nil
	}
}

func (l *Lease) Owner() string {
	return l.owner
}

func CurrentOwner(ctx context.Context, path string) (string, error) {
	mutex, err := acquireMutex(ctx, path+".mutex", 250*time.Millisecond)
	if err != nil {
		return "", err
	}
	defer mutex.Unlock()
	current, err := readRecord(path)
	if err != nil {
		return "", err
	}
	if !current.ExpiresAt.After(time.Now().UTC()) {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		return "", os.ErrNotExist
	}
	return current.Owner, nil
}

func (l *Lease) Release(ctx context.Context) error {
	mutex, err := acquireMutex(ctx, l.path+".mutex", 5*time.Second)
	if err != nil {
		return err
	}
	defer mutex.Unlock()
	current, err := readRecord(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if current.Owner != l.owner {
		return errors.New("wiki synchronization lease owner changed")
	}
	return os.Remove(l.path)
}

func acquireMutex(ctx context.Context, path string, wait time.Duration) (*flock.Flock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if wait < 0 {
		wait = 0
	}
	deadline := time.Now().Add(wait)
	mutex := flock.New(path)
	for {
		locked, err := mutex.TryLock()
		if err != nil {
			return nil, err
		}
		if locked {
			return mutex, nil
		}
		if time.Now().After(deadline) {
			return nil, errors.New("wiki lease mutex timeout")
		}
		if err := waitForRetry(ctx, 10*time.Millisecond); err != nil {
			return nil, err
		}
	}
}

func waitForRetry(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func readRecord(path string) (record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return record{}, err
	}
	var value record
	err = json.Unmarshal(data, &value)
	return value, err
}

func writeRecord(path string, value record) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return atomicfile.Write(path, append(data, '\n'), 0o600)
}
