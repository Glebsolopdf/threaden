package groupchat

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"voice-rooms/internal/attachments"
	"voice-rooms/internal/attachments/storage"
	appgroups "voice-rooms/internal/groups"
	"voice-rooms/internal/publicview"
)

func (h *Handler) sendMultipart(w http.ResponseWriter, r *http.Request) {
	if h.hooks.Attachments == nil {
		h.hooks.WriteError(w, r, http.StatusServiceUnavailable, "attachments_unavailable", "attachments are temporarily unavailable")
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		h.hooks.WriteError(w, r, http.StatusBadRequest, "invalid_multipart", "invalid multipart request")
		return
	}
	files := r.MultipartForm.File["files[]"]
	if len(files) == 0 {
		files = r.MultipartForm.File["files"]
	}
	if len(files) == 0 || len(files) > 3 {
		h.hooks.WriteError(w, r, http.StatusBadRequest, "attachment_count", "message must contain between 1 and 3 files")
		return
	}
	inputs := make([]storage.Input, 0, len(files))
	for _, header := range files {
		fileHeader := header
		inputs = append(inputs, storage.Input{Name: fileHeader.Filename, Size: fileHeader.Size, Open: func() (io.ReadCloser, error) { return fileHeader.Open() }})
	}
	batch, err := h.hooks.Attachments.Prepare(r.Context(), h.hooks.CurrentUser(r).ID, inputs)
	if err != nil {
		writeAttachmentError(h.hooks.WriteError, w, r, err)
		return
	}
	body := formValue(r, "body")
	replyToID := formValue(r, "reply_to_id")
	m, err := h.service.SendWithAttachments(r.Context(), chi.URLParam(r, "id"), h.hooks.CurrentUser(r), body, replyToID, batch, r.Header.Get("Idempotency-Key"))
	if err != nil {
		writeAttachmentError(h.hooks.WriteError, w, r, err)
		return
	}
	h.hooks.WriteJSON(w, http.StatusCreated, publicview.MessageView(m))
}

func formValue(r *http.Request, key string) string {
	values := r.MultipartForm.Value[key]
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func writeAttachmentError(writeError func(http.ResponseWriter, *http.Request, int, string, string), w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := http.StatusBadRequest, "attachment_invalid", "attachment could not be processed"
	switch {
	case errors.Is(err, attachments.ErrTooLarge):
		code, message = "attachment_too_large", "attachment exceeds the allowed size"
	case errors.Is(err, attachments.ErrTooMany):
		code, message = "attachment_count", "message contains too many attachments"
	case errors.Is(err, attachments.ErrQuotaExceeded):
		status, code, message = http.StatusRequestEntityTooLarge, "attachment_quota", "attachment quota exceeded"
	case errors.Is(err, attachments.ErrLowDisk):
		status, code, message = http.StatusInsufficientStorage, "attachment_storage_full", "attachment storage is temporarily unavailable"
	case errors.Is(err, attachments.ErrUnsupportedFormat):
		code, message = "attachment_unsupported_format", "attachment format is not supported"
	case errors.Is(err, appgroups.ErrForbidden):
		status, code, message = http.StatusForbidden, "forbidden", "operation is not allowed"
	}
	writeError(w, r, status, code, message)
}
