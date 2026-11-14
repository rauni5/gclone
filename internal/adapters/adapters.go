// Package adapters is the list of ecosystems gclone knows. Adding an ecosystem means
// adding it here.
package adapters

import (
	"github.com/rauni5/gclone/internal/adapter"
	"github.com/rauni5/gclone/internal/adapter/golang"
	"github.com/rauni5/gclone/internal/adapter/java"
	"github.com/rauni5/gclone/internal/adapter/node"
)

// All returns the adapters in the order they are analyzed and shown.
func All() []adapter.Adapter {
	return []adapter.Adapter{node.Adapter{}, golang.Adapter{}, java.Adapter{}}
}
