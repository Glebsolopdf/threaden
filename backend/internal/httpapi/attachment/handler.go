package attachment

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"

	"voice-rooms/internal/model"
	"voice-rooms/internal/store"
)

func Download(st *store.Store, currentUser func(*http.Request) model.User, writeError func(http.ResponseWriter, *http.Request, int, string, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, err := st.Attachment(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
			return
		}
		user := currentUser(r)
		member, err := st.IsGroupMember(r.Context(), item.GroupID, user.ID)
		if err != nil || !member {
			writeError(w, r, http.StatusForbidden, "forbidden", "operation is not allowed")
			return
		}
		if !item.ExpiresAt.After(time.Now()) {
			writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
			return
		}
		file, err := os.Open(item.Path)
		if err != nil {
			writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			writeError(w, r, http.StatusNotFound, "not_found", "resource not found")
			return
		}
		w.Header().Set("Content-Type", item.Mime)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		contentDisposition := "attachment"
		if item.Kind == "audio" {
			contentDisposition = "inline"
		}
		w.Header().Set("Content-Disposition", contentDisposition+"; filename*=UTF-8''"+urlEscapeName(filepath.Base(item.Name)))
		http.ServeContent(w, r, filepath.Base(item.Name), item.CreatedAt, file)
	}
}

func urlEscapeName(name string) string {
	return url.PathEscape(name)
}
