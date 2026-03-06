// Package resolver maps logical asset paths to storage backends.
//
// It provides a Strategy-pattern based resolver that routes asset
// requests to the appropriate storage provider based on path prefixes,
// media types, or custom rules.
//
// Design pattern: Strategy (storage backend selection).
package resolver

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
)

// Backend represents a storage backend that can read and write assets.
type Backend interface {
	Name() string
	Read(ctx context.Context, path string) (io.ReadCloser, error)
	Write(ctx context.Context, path string, data io.Reader) error
	Exists(ctx context.Context, path string) (bool, error)
	Delete(ctx context.Context, path string) error
}

// Rule defines how paths are mapped to backends.
type Rule struct {
	Prefix  string
	Backend string
}

// Resolver maps logical asset paths to storage backends.
type Resolver struct {
	mu       sync.RWMutex
	backends map[string]Backend
	rules    []Rule
	fallback string
}

// New creates a new Resolver.
func New() *Resolver {
	return &Resolver{
		backends: make(map[string]Backend),
	}
}

// RegisterBackend adds a storage backend.
func (r *Resolver) RegisterBackend(b Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.backends[b.Name()] = b
}

// AddRule adds a routing rule mapping a path prefix to a backend name.
func (r *Resolver) AddRule(prefix, backendName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules = append(r.rules, Rule{
		Prefix:  prefix,
		Backend: backendName,
	})
}

// SetFallback sets the default backend name when no rule matches.
func (r *Resolver) SetFallback(backendName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fallback = backendName
}

// Resolve returns the backend for the given path.
func (r *Resolver) Resolve(path string) (Backend, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, rule := range r.rules {
		if strings.HasPrefix(path, rule.Prefix) {
			b, ok := r.backends[rule.Backend]
			if !ok {
				return nil, fmt.Errorf(
					"backend %q referenced by rule for prefix %q not found",
					rule.Backend, rule.Prefix,
				)
			}
			return b, nil
		}
	}

	if r.fallback != "" {
		b, ok := r.backends[r.fallback]
		if !ok {
			return nil, fmt.Errorf(
				"fallback backend %q not found", r.fallback,
			)
		}
		return b, nil
	}

	return nil, fmt.Errorf("no backend matched path %q and no fallback configured", path)
}

// Read resolves and reads an asset.
func (r *Resolver) Read(ctx context.Context, path string) (io.ReadCloser, error) {
	b, err := r.Resolve(path)
	if err != nil {
		return nil, fmt.Errorf("resolve for read: %w", err)
	}
	return b.Read(ctx, path)
}

// Write resolves and writes an asset.
func (r *Resolver) Write(ctx context.Context, path string, data io.Reader) error {
	b, err := r.Resolve(path)
	if err != nil {
		return fmt.Errorf("resolve for write: %w", err)
	}
	return b.Write(ctx, path, data)
}

// Exists checks if an asset exists.
func (r *Resolver) Exists(ctx context.Context, path string) (bool, error) {
	b, err := r.Resolve(path)
	if err != nil {
		return false, fmt.Errorf("resolve for exists: %w", err)
	}
	return b.Exists(ctx, path)
}

// Delete removes an asset.
func (r *Resolver) Delete(ctx context.Context, path string) error {
	b, err := r.Resolve(path)
	if err != nil {
		return fmt.Errorf("resolve for delete: %w", err)
	}
	return b.Delete(ctx, path)
}
