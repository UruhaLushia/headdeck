package identity

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	hc "github.com/UruhaLushia/headdeck/internal/headscale/config"
	"github.com/UruhaLushia/headdeck/internal/paths"
	"github.com/UruhaLushia/headdeck/internal/securefile"
)

const journalDirectory = paths.StateDir + "/headscale-login"
const journalName = "transaction.json"

type Journal struct {
	Configuration hc.OIDCChange `json:"configuration"`
	Bindings      []Binding     `json:"bindings"`
	UID           uint32        `json:"uid"`
	Unit          string        `json:"unit"`
	Committed     bool          `json:"committed"`
}

func Pending() error {
	_, err := os.Lstat(journalDirectory + "/" + journalName)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("unfinished Headscale login transaction; run hdkt headscale login recover")
}

func journalRoot() (*os.Root, error) {
	if err := securefile.PrepareDirectory(journalDirectory, 0o700); err != nil {
		return nil, err
	}
	info, err := os.Stat(journalDirectory)
	if err != nil {
		return nil, err
	}
	if info.Mode().Perm() != 0o700 {
		return nil, fmt.Errorf("login transaction directory must be private")
	}
	return os.OpenRoot(journalDirectory)
}

func (j *Journal) Save(initial bool) error {
	root, err := journalRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	if initial {
		if _, err := root.Lstat(journalName); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("recover the previous login transaction first")
		}
	}
	data, err := json.Marshal(j)
	if err != nil {
		return err
	}
	if len(data) > 16<<20 {
		return fmt.Errorf("login transaction exceeds the size limit")
	}
	const temporary = "transaction.tmp"
	if err := root.Remove(temporary); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := root.Rename(temporary, journalName); err != nil {
		return err
	}
	return syncDirectory(root)
}

func LoadJournal() (*Journal, error) {
	root, err := journalRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	info, err := root.Lstat(journalName)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || !securefile.Owned(info) || info.Size() > 16<<20 {
		return nil, fmt.Errorf("unsafe login transaction")
	}
	f, err := root.Open(journalName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var j Journal
	decoder := json.NewDecoder(io.LimitReader(f, 16<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&j); err != nil {
		return nil, err
	}
	if len(j.Configuration.Before) == 0 || len(j.Configuration.After) == 0 {
		return nil, fmt.Errorf("incomplete login transaction")
	}
	return &j, nil
}

func ClearJournal() error {
	root, err := journalRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.Remove(journalName); err != nil {
		return err
	}
	return syncDirectory(root)
}

func syncDirectory(root *os.Root) error {
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
