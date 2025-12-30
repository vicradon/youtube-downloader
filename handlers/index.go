package handlers

import (
	"net/http"
	"path/filepath"
)

type IndexHandler struct {
	execDir string
}

func NewIndexHandler(execDir string) *IndexHandler {
	return &IndexHandler{execDir: execDir}
}

func (h *IndexHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(h.execDir, "templates", "index.html"))
}
