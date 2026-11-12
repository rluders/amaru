package domain

import "encoding/json"

// ProjectSettings is a Platform-owned discovery preference, never execution authority.
type ProjectSettings struct {
	ImportClaudeSkills bool `json:"importClaudeSkills"`
}

// UpdateSettings preserves fields omitted by a partial update.
type UpdateSettings struct {
	ImportClaudeSkills *bool `json:"importClaudeSkills,omitempty"`
}

func (p UpdateSettings) Validate() error {
	if p.ImportClaudeSkills == nil {
		return ErrInvalid
	}
	return nil
}

// ParseUpdateSettings enforces the exact existing UpdateSettings wire shape.
func ParseUpdateSettings(data []byte) (UpdateSettings, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || len(fields) != 1 {
		return UpdateSettings{}, ErrInvalid
	}
	raw, ok := fields["importClaudeSkills"]
	if !ok {
		return UpdateSettings{}, ErrInvalid
	}
	var value *bool
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return UpdateSettings{}, ErrInvalid
	}
	return UpdateSettings{ImportClaudeSkills: value}, nil
}
