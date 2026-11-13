package httpapi

import (
	"bytes"
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

var projectPatterns = []string{"POST /v1/projects", "GET /v1/projects", "GET /v1/projects/{projectId}"}

// ProjectsHandler binds the transport guard and application operations to the same
// Auth and authoritative Project store. Inspection grants no execution authority.
func ProjectsHandler(auth *localauth.Auth, c *coordinator.Coordinator, store ports.ProjectCloneStore, inspector ports.ProjectInspector) (http.Handler, error) {
	if auth == nil || inspector == nil {
		return nil, domain.ErrInvalid
	}
	access, err := coordinator.NewProjectAccess(c, store, auth.Principal)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc(projectPatterns[0], func(w http.ResponseWriter, r *http.Request) {
		if _, err := auth.Principal(r.Context()); err != nil {
			projectError(w, err)
			return
		}
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			reject(w, 400, "invalid_request")
			return
		}
		data, readErr := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
		if _, err := auth.Principal(r.Context()); err != nil {
			projectError(w, err)
			return
		}
		root, err := parseAttach(data)
		if readErr != nil || err != nil {
			reject(w, 400, "invalid_request")
			return
		}
		value, _, err := access.AttachProject(r.Context(), root, inspector)
		if err != nil {
			projectError(w, err)
			return
		}
		respond(w, 200, projectResponse(value))
	})
	mux.HandleFunc(projectPatterns[1], func(w http.ResponseWriter, r *http.Request) {
		values, err := access.Projects(r.Context(), inspector)
		if err != nil {
			projectError(w, err)
			return
		}
		result := make([]projectJSON, 0, len(values))
		for _, value := range values {
			result = append(result, projectResponse(value))
		}
		respond(w, 200, result)
	})
	mux.HandleFunc(projectPatterns[2], func(w http.ResponseWriter, r *http.Request) {
		value, err := access.Project(r.Context(), domain.ProjectKey(r.PathValue("projectId")), inspector)
		if err != nil {
			projectError(w, err)
			return
		}
		respond(w, 200, projectResponse(value))
	})
	return auth.Handler(mux), nil
}

// Accept exactly one root field, including rejection of duplicate and case-folded keys.
func parseAttach(data []byte) (string, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	if token, err := d.Token(); err != nil || token != json.Delim('{') {
		return "", domain.ErrInvalid
	}
	if token, err := d.Token(); err != nil || token != "root" {
		return "", domain.ErrInvalid
	}
	var root string
	if err := d.Decode(&root); err != nil || root == "" {
		return "", domain.ErrInvalid
	}
	if token, err := d.Token(); err != nil || token != json.Delim('}') {
		return "", domain.ErrInvalid
	}
	if _, err := d.Token(); err != io.EOF {
		return "", domain.ErrInvalid
	}
	return root, nil
}

type projectJSON struct {
	ID     domain.ProjectKey `json:"id"`
	Name   string            `json:"name"`
	Root   string            `json:"root"`
	Config struct {
		Present bool   `json:"present"`
		Path    string `json:"path,omitempty"`
	} `json:"config"`
	Git struct {
		Present bool `json:"present"`
	} `json:"git"`
}

func projectResponse(value coordinator.InspectedProject) projectJSON {
	p := projectJSON{ID: value.Project.Key, Name: value.Project.Name, Root: value.Inspection.Root}
	p.Config.Present = value.Inspection.ConfigPresent
	if p.Config.Present {
		p.Config.Path = ".amaru/config.yaml"
	}
	p.Git.Present = true
	return p
}

func projectError(w http.ResponseWriter, err error) {
	if errors.Is(err, domain.ErrConflict) {
		reject(w, 409, "conflict")
		return
	}
	settingsError(w, err)
}
