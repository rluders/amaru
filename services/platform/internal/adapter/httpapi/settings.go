// Package httpapi maps public contracts onto authorized Platform operations.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/rluders/amaru/services/platform/internal/adapter/localauth"
	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

var settingsPatterns = []string{"GET /v1/projects/{projectId}/settings", "PATCH /v1/projects/{projectId}/settings"}

// SettingsHandler includes the compact Host/Origin/session/CSRF guard. access and
// store must use this Auth instance and the same authoritative Instance stores.
func SettingsHandler(auth *localauth.Auth, access *coordinator.ProjectAccess, store ports.SettingsStore) (http.Handler, error) {
	if auth == nil || access == nil || store == nil {
		return nil, domain.ErrInvalid
	}
	mux := http.NewServeMux()
	handle := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPatch {
			reject(w, http.StatusMethodNotAllowed, "invalid_request")
			return
		}
		project := domain.ProjectKey(r.PathValue("projectId"))
		// The authorized read checks attachment before decoding any client body.
		value, err := access.Settings(r.Context(), project, store)
		if err != nil {
			settingsError(w, err)
			return
		}
		if r.Method == http.MethodPatch {
			media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || media != "application/json" {
				reject(w, 400, "invalid_request")
				return
			}
			data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
			if err != nil {
				reject(w, 400, "invalid_request")
				return
			}
			patch, err := domain.ParseUpdateSettings(data)
			if err != nil {
				settingsError(w, err)
				return
			}
			// Recheck live identity after body IO and before the atomic update.
			value, err = access.UpdateSettings(r.Context(), project, patch, store)
			if err != nil {
				settingsError(w, err)
				return
			}
		}
		respond(w, http.StatusOK, value)
	}
	for _, pattern := range settingsPatterns {
		mux.HandleFunc(pattern, handle)
	}
	return auth.Handler(mux), nil
}

func settingsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotPermitted):
		reject(w, 403, "forbidden")
	case errors.Is(err, domain.ErrNotFound):
		reject(w, 404, "not_found")
	case errors.Is(err, domain.ErrInvalid):
		reject(w, 400, "invalid_request")
	default:
		reject(w, 500, "internal")
	}
}

func reject(w http.ResponseWriter, status int, code string) {
	respond(w, status, map[string]any{"error": map[string]string{"code": code, "message": "Request rejected."}})
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
