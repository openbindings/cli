package app

import "github.com/openbindings/cli/internal/delegates"

// Error is the shared structured error type used throughout the app layer.
// It is an alias for delegates.Error so that a single definition is used across
// the app and delegate packages.
type Error = delegates.Error
