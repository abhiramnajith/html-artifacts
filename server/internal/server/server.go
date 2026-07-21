// Package server wires the HTTP handlers (view, list, annotations) onto a
// storage.Store. It never inspects or branches on which agent produced or
// consumes an artifact — the contract is files and HTTP only.
//
// Phase 0 ships the package so the module compiles; the handlers arrive in
// Phase 2.
package server

import "github.com/abhiramnajith/html-artifacts/server/internal/storage"

// Server holds the dependencies shared by the HTTP handlers.
type Server struct {
	store *storage.Store
}

// New returns a Server backed by the given store.
func New(store *storage.Store) *Server {
	return &Server{store: store}
}
