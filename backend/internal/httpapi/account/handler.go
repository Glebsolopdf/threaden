package account

import (
	"errors"
	"net/http"
	"time"

	"voice-rooms/internal/attachments/account"
	"voice-rooms/internal/model"
)

type Hooks struct {
	WriteJSON   func(http.ResponseWriter, int, any)
	WriteError  func(http.ResponseWriter, *http.Request, int, string, string)
	CurrentUser func(*http.Request) model.User
}

type Handler struct {
	service *account.Service
	hooks   Hooks
}

type QuotaResponse struct {
	Usage         UsageResponse                  `json:"usage"`
	Limits        LimitsResponse                 `json:"limits"`
	PendingDelete *model.AttachmentDeleteRequest `json:"pending_delete,omitempty"`
}

type UsageResponse struct {
	StoredBytes int64 `json:"stored_bytes"`
	DailyBytes  int64 `json:"daily_bytes"`
}

type LimitsResponse struct {
	MaxInputMediaBytes  uint64 `json:"max_input_media_bytes"`
	MaxArchiveBytes     uint64 `json:"max_archive_bytes"`
	MaxOutputMediaBytes uint64 `json:"max_output_media_bytes"`
	MaxFilesPerMessage  int    `json:"max_files_per_message"`
	MaxUserStoredBytes  uint64 `json:"max_user_stored_bytes"`
	MaxUserDailyBytes   uint64 `json:"max_user_daily_bytes"`
	MaxTotalBytes       uint64 `json:"max_total_bytes"`
	MinFreeBytes        uint64 `json:"min_free_bytes"`
	RetentionSeconds    int64  `json:"retention_seconds"`
}

func New(service *account.Service, hooks Hooks) *Handler {
	return &Handler{service: service, hooks: hooks}
}

func (h *Handler) Quotas(w http.ResponseWriter, r *http.Request) {
	quota, err := h.service.Quotas(r.Context(), h.hooks.CurrentUser(r).ID)
	if err != nil {
		h.hooks.WriteError(w, r, http.StatusInternalServerError, "quota_unavailable", "quotas are temporarily unavailable")
		return
	}
	h.hooks.WriteJSON(w, http.StatusOK, quotaResponse(quota))
}

func (h *Handler) ScheduleDelete(w http.ResponseWriter, r *http.Request) {
	request, err := h.service.ScheduleDeleteAll(r.Context(), h.hooks.CurrentUser(r).ID, time.Now().UTC())
	if err != nil {
		h.hooks.WriteError(w, r, http.StatusInternalServerError, "attachment_delete_unavailable", "attachments cannot be scheduled for deletion")
		return
	}
	h.hooks.WriteJSON(w, http.StatusAccepted, request)
}

func (h *Handler) CancelDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.CancelDeleteAll(r.Context(), h.hooks.CurrentUser(r).ID); err != nil {
		if errors.Is(err, account.ErrDeleteRequestNotFound) {
			h.hooks.WriteError(w, r, http.StatusConflict, "attachment_delete_not_pending", "there is no pending attachment deletion")
			return
		}
		h.hooks.WriteError(w, r, http.StatusInternalServerError, "attachment_delete_unavailable", "attachments cannot be restored")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func quotaResponse(quota account.QuotaSnapshot) QuotaResponse {
	return QuotaResponse{Usage: UsageResponse{StoredBytes: quota.StoredBytes, DailyBytes: quota.DailyBytes}, Limits: LimitsResponse{MaxInputMediaBytes: quota.MaxInputMedia, MaxArchiveBytes: quota.MaxArchive, MaxOutputMediaBytes: quota.MaxOutputMedia, MaxFilesPerMessage: quota.MaxFiles, MaxUserStoredBytes: quota.MaxUserStored, MaxUserDailyBytes: quota.MaxUserDaily, MaxTotalBytes: quota.MaxTotal, MinFreeBytes: quota.MinFree, RetentionSeconds: int64(quota.Retention.Seconds())}, PendingDelete: quota.PendingDelete}
}
