package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/merdandt/LLM-wiki-dev/internal/atomicfile"
)

type Layout struct {
	Root string
}

func NewLayout(root string) Layout {
	return Layout{Root: root}
}

func (l Layout) WriteReceipt(receipt Receipt) error {
	return writeJSON(l.receiptPath(receipt.Fingerprint), receipt)
}

func (l Layout) ReadReceipt(fp Fingerprint) (Receipt, error) {
	var receipt Receipt
	err := readJSON(l.receiptPath(fp), &receipt)
	return receipt, err
}

func (l Layout) WriteSession(session Session) error {
	return writeJSON(l.sessionPath(session.ID), session)
}

func (l Layout) ReadSession(id string) (Session, error) {
	var session Session
	err := readJSON(l.sessionPath(id), &session)
	return session, err
}

func (l Layout) WriteObservation(observation Observation) error {
	return writeJSON(l.observationPath(observation.WorktreeID), observation)
}

func (l Layout) ReadObservation(worktreeID string) (Observation, error) {
	var observation Observation
	err := readJSON(l.observationPath(worktreeID), &observation)
	return observation, err
}

func (l Layout) LatestSession(worktreeID string) (Session, error) {
	entries, err := os.ReadDir(filepath.Join(l.Root, "sessions"))
	if err != nil {
		return Session{}, err
	}
	var latest Session
	found := false
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var candidate Session
		if err := readJSON(filepath.Join(l.Root, "sessions", entry.Name()), &candidate); err != nil {
			return Session{}, err
		}
		if candidate.WorktreeID == worktreeID &&
			(!found || candidate.StartedAt.After(latest.StartedAt)) {
			latest = candidate
			found = true
		}
	}
	if !found {
		return Session{}, os.ErrNotExist
	}
	return latest, nil
}

func (l Layout) LatestReceipt() (Receipt, error) {
	entries, err := os.ReadDir(filepath.Join(l.Root, "receipts"))
	if err != nil {
		return Receipt{}, err
	}
	var latest Receipt
	found := false
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var candidate Receipt
		if err := readJSON(filepath.Join(l.Root, "receipts", entry.Name()), &candidate); err != nil {
			return Receipt{}, err
		}
		if !found || candidate.CreatedAt.After(latest.CreatedAt) {
			latest = candidate
			found = true
		}
	}
	if !found {
		return Receipt{}, os.ErrNotExist
	}
	return latest, nil
}

func (l Layout) receiptPath(fp Fingerprint) string {
	data, _ := json.Marshal(fp)
	sum := sha256.Sum256(data)
	return filepath.Join(l.Root, "receipts", hex.EncodeToString(sum[:])+".json")
}

func (l Layout) observationPath(worktreeID string) string {
	sum := sha256.Sum256([]byte(worktreeID))
	return filepath.Join(l.Root, "observations", hex.EncodeToString(sum[:])+".json")
}

func (l Layout) LockPath(worktreeID string) string {
	sum := sha256.Sum256([]byte(worktreeID))
	return filepath.Join(l.Root, "locks", hex.EncodeToString(sum[:])+".lock")
}

func (l Layout) sessionPath(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return filepath.Join(l.Root, "sessions", hex.EncodeToString(sum[:])+".json")
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(path, append(data, '\n'), 0o600)
}

func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.ErrNotExist
		}
		return err
	}
	return json.Unmarshal(data, value)
}
