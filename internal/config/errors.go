package config

import "errors"

// ErrUnknownVersion means a settings file declares a version this build
// does not know how to read or migrate.
var ErrUnknownVersion = errors.New("config: unknown settings version")

// ErrInvalidSettings means a settings file is not valid JSON of a known version.
var ErrInvalidSettings = errors.New("config: invalid settings file")
