package openapi

import (
	"bytes"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// decodeDocument safely decodes JSON or YAML input into a generic
// map[string]any tree. YAML anchors/aliases are supported by the decoder,
// but we cap the number of resolved nodes while walking the tree afterward
// (see refs.go) to prevent alias-expansion ("billion laughs") attacks, and
// we reject any node count blow-up by capping total decoded scalar/map/seq
// nodes.
func decodeDocument(data []byte) (map[string]any, error) {
	if len(data) == 0 {
		return nil, ErrUnsupportedInput
	}
	if len(data) > MaxDocumentBytes {
		return nil, ErrTooLarge
	}

	trimmed := bytes.TrimSpace(data)
	var doc map[string]any

	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		dec := json.NewDecoder(bytes.NewReader(trimmed))
		dec.UseNumber()
		if err := dec.Decode(&doc); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUnsupportedInput, err)
		}
		return doc, nil
	}

	// YAML decoding. gopkg.in/yaml.v3 limits alias expansion internally to a
	// bounded factor, but we still defensively count nodes while resolving
	// refs afterwards.
	if err := yaml.Unmarshal(trimmed, &doc); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnsupportedInput, err)
	}
	if doc == nil {
		return nil, ErrUnsupportedInput
	}
	return normalizeYAMLTypes(doc).(map[string]any), nil
}

// normalizeYAMLTypes recursively converts YAML-decoded structures into the
// same shapes produced by encoding/json (map[string]any / []any / scalars),
// since yaml.v3 may occasionally use map[string]interface{} keys already
// but nested numeric types differ slightly (int vs json.Number). We keep
// numbers as-is (int/float64) since only json.Number matters for byte-exact
// round-tripping, which we don't need here.
func normalizeYAMLTypes(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = normalizeYAMLTypes(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = normalizeYAMLTypes(val)
		}
		return out
	default:
		return v
	}
}
