package handlers

import (
	"net/http"
	"path/filepath"
)

type ConversionsPageHandler struct {
	execDir string
}

func NewConversionsPageHandler(execDir string) *ConversionsPageHandler {
	return &ConversionsPageHandler{execDir: execDir}
}

func (h *ConversionsPageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(h.execDir, "templates", "conversions.html"))
}
