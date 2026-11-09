package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const fileVersion = 1

// fileEnvelope is the on-disk shape. Version is required so a future
// format is migrated explicitly instead of being merged field by field.
type fileEnvelope struct {
	Version int             `json:"version"`
	Data    json.RawMessage `json:"data"`
}

// Load reads path. A missing file returns defaults with a nil error.
// A version-0 file is migrated. Any other version returns ErrUnknownVersion.
func Load[T any](path string, defaults func() T, migrateV0 func(json.RawMessage) (T, error)) (T, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaults(), nil
	}
	if err != nil {
		var zero T
		return zero, fmt.Errorf("config: read %s: %w", path, err)
	}
	return decode(raw, defaults, migrateV0)
}

func decode[T any](raw []byte, defaults func() T, migrateV0 func(json.RawMessage) (T, error)) (T, error) {
	var zero T
	var env fileEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return zero, fmt.Errorf("%w: %s", ErrInvalidSettings, err)
	}
	switch env.Version {
	case 0:
		if migrateV0 == nil {
			return zero, fmt.Errorf("%w: 0", ErrUnknownVersion)
		}
		value, err := migrateV0(raw)
		if err != nil {
			return zero, err
		}
		return value, nil
	case fileVersion:
		value := defaults()
		if len(env.Data) == 0 || string(env.Data) == "null" {
			return value, nil
		}
		if err := json.Unmarshal(env.Data, &value); err != nil {
			return zero, fmt.Errorf("%w: %s", ErrInvalidSettings, err)
		}
		return value, nil
	default:
		return zero, fmt.Errorf("%w: %d", ErrUnknownVersion, env.Version)
	}
}

// Save writes value as a versioned JSON file. The write goes to a temporary
// file in the same directory and is renamed into place, so a failure leaves
// the previous file unchanged.
func Save(path string, value any) error {
	body, err := json.MarshalIndent(fileEnvelope{
		Version: fileVersion,
		Data:    mustRaw(value),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("config: encode %s: %w", path, err)
	}
	body = append(body, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("config: mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("config: temp file for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("config: write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("config: close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("config: rename %s: %w", path, err)
	}
	cleanup = false
	return nil
}

func mustRaw(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		// Save reports the same class of error; a second marshal of a
		// value that just marshaled cannot fail for data reasons.
		panic(fmt.Sprintf("config: marshal settings: %v", err))
	}
	return raw
}
