package openapi

import (
	"fmt"
	"strconv"
	"strings"
)

// refResolver resolves local "#/a/b/c" JSON pointer references within a
// single decoded document. Remote/external refs (URLs or file paths) are
// intentionally rejected: following them would mean fetching untrusted
// user-supplied URLs from the server process, which is an SSRF vector.
type refResolver struct {
	root     map[string]any
	resolved int
	inflight map[string]bool
}

func newRefResolver(root map[string]any) *refResolver {
	return &refResolver{root: root, inflight: map[string]bool{}}
}

// Resolve walks a node, replacing any {"$ref": "#/..."} object with a deep,
// fully-resolved copy of the referenced node. It rejects external refs and
// enforces a global resolution budget and max depth to prevent resource
// exhaustion from adversarial documents.
func (r *refResolver) Resolve(node any, depth int) (any, error) {
	if depth > MaxRefDepth {
		return nil, ErrRefBudgetExceeded
	}
	r.resolved++
	if r.resolved > MaxRefResolutions {
		return nil, ErrRefBudgetExceeded
	}

	switch t := node.(type) {
	case map[string]any:
		if refVal, ok := t["$ref"]; ok {
			refStr, _ := refVal.(string)
			return r.resolveRef(refStr, depth)
		}
		out := make(map[string]any, len(t))
		for k, v := range t {
			resolved, err := r.Resolve(v, depth+1)
			if err != nil {
				return nil, err
			}
			out[k] = resolved
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, v := range t {
			resolved, err := r.Resolve(v, depth+1)
			if err != nil {
				return nil, err
			}
			out[i] = resolved
		}
		return out, nil
	default:
		return t, nil
	}
}

func (r *refResolver) resolveRef(ref string, depth int) (any, error) {
	if !strings.HasPrefix(ref, "#/") {
		// External/remote refs are rejected outright (SSRF hardening: never
		// fetch user-supplied URLs or filesystem paths from the server).
		return map[string]any{"_unresolvedRef": ref}, nil
	}
	if r.inflight[ref] {
		// Circular reference: stop expanding, return a marker instead of
		// recursing forever.
		return map[string]any{"_circularRef": ref}, nil
	}
	target, err := lookupPointer(r.root, ref)
	if err != nil {
		return map[string]any{"_unresolvedRef": ref}, nil
	}
	r.inflight[ref] = true
	defer delete(r.inflight, ref)
	return r.Resolve(target, depth+1)
}

func lookupPointer(root map[string]any, ref string) (any, error) {
	path := strings.TrimPrefix(ref, "#/")
	if path == "" {
		return root, nil
	}
	parts := strings.Split(path, "/")
	var cur any = root
	for _, part := range parts {
		part = jsonPointerUnescape(part)
		switch node := cur.(type) {
		case map[string]any:
			val, ok := node[part]
			if !ok {
				return nil, fmt.Errorf("ref segment %q not found", part)
			}
			cur = val
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil, fmt.Errorf("ref index %q not found", part)
			}
			cur = node[idx]
		default:
			return nil, fmt.Errorf("cannot descend into ref segment %q", part)
		}
	}
	return cur, nil
}

func jsonPointerUnescape(s string) string {
	s = strings.ReplaceAll(s, "~1", "/")
	s = strings.ReplaceAll(s, "~0", "~")
	return s
}
