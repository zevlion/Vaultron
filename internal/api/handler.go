package api

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/zevlion/vaultron/internal/store"
)

// Handler serves the Vaultron HTTP API.
//
// Routes:
//
//	PUT  /objects/          — upload a new object; returns 201 + content hash
//	GET  /objects/{hash}    — stream an object by content hash
//	HEAD /objects/{hash}    — check existence without body
//	DELETE /objects/{hash}  — delete an object
type Handler struct {
	store *store.Store
}

// New returns an http.Handler backed by the given store.
func New(st *store.Store) http.Handler {
	h := &Handler{store: st}
	mux := http.NewServeMux()
	mux.HandleFunc("/objects/", h.route)
	return mux
}

func (h *Handler) route(w http.ResponseWriter, r *http.Request) {
	// Strip prefix to get the optional hash segment.
	hash := strings.TrimPrefix(r.URL.Path, "/objects/")

	switch r.Method {
	case http.MethodPut:
		h.put(w, r)
	case http.MethodGet:
		if hash == "" {
			http.Error(w, "missing content hash", http.StatusBadRequest)
			return
		}
		h.get(w, r, hash)
	case http.MethodHead:
		if hash == "" {
			http.Error(w, "missing content hash", http.StatusBadRequest)
			return
		}
		h.head(w, r, hash)
	case http.MethodDelete:
		if hash == "" {
			http.Error(w, "missing content hash", http.StatusBadRequest)
			return
		}
		h.delete(w, r, hash)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) put(w http.ResponseWriter, r *http.Request) {
	contentHash, err := h.store.Put(r.Body)
	if err != nil {
		log.Printf("PUT error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	io.WriteString(w, contentHash+"\n")
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request, contentHash string) {
	meta, err := h.store.Stat(contentHash)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Hash", contentHash)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", meta.Size))

	if err := h.store.Get(contentHash, w); err != nil {
		// Headers already sent; log and drop.
		log.Printf("GET stream error %s: %v", contentHash, err)
	}
}

func (h *Handler) head(w http.ResponseWriter, r *http.Request, contentHash string) {
	meta, err := h.store.Stat(contentHash)
	if errors.Is(err, store.ErrNotFound) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("X-Content-Hash", contentHash)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", meta.Size))
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request, contentHash string) {
	err := h.store.Delete(contentHash)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
