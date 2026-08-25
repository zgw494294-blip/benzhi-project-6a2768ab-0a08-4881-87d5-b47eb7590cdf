package httpapi

import (
	"errors"
	"log"
	"net/http"
	"seed-vault-admission/internal/admission"
)

type admissionError struct {
	status        int
	code, message string
}

func (e *admissionError) Error() string { return e.message }

func writeError(w http.ResponseWriter, err error) {
	var transport *admissionError
	if errors.As(err, &transport) {
		writeJSON(w, transport.status, map[string]any{"error": map[string]string{"code": transport.code, "message": transport.message}})
		return
	}
	code := admission.ErrorCode(err)
	status := http.StatusBadRequest
	switch code {
	case "not_found":
		status = http.StatusNotFound
	case "version_conflict", "state_conflict", "idempotency_conflict":
		status = http.StatusConflict
	case "internal_error":
		status = http.StatusInternalServerError
		log.Printf("http internal error: %v", err)
	}
	message := err.Error()
	if status == http.StatusInternalServerError {
		message = "服务器内部错误"
	}
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
