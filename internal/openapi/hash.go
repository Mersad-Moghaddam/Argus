package openapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// computeSpecHash produces a stable hash of the parts of an operation that
// matter for change detection during re-import. It deliberately excludes
// user-editable monitoring configuration (interval, timeout, enabled, ...)
// so those settings survive re-imports untouched.
func computeSpecHash(r Route) string {
	tags := append([]string(nil), r.Tags...)
	sort.Strings(tags)
	payload := struct {
		Method      string      `json:"method"`
		Path        string      `json:"path"`
		Summary     string      `json:"summary"`
		Description string      `json:"description"`
		Tags        []string    `json:"tags"`
		Deprecated  bool        `json:"deprecated"`
		Parameters  []Parameter `json:"parameters"`
		RequestBody any         `json:"requestBody"`
		Responses   any         `json:"responses"`
		Security    any         `json:"security"`
	}{r.Method, r.Path, r.Summary, r.Description, tags, r.Deprecated, r.Parameters, r.RequestBody, r.Responses, r.Security}
	b, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
