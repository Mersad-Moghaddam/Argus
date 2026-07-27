package openapi

import (
	"fmt"
	"sort"
	"strings"
)

var httpMethods = map[string]bool{
	"get": true, "post": true, "put": true, "patch": true,
	"delete": true, "head": true, "options": true, "trace": true,
}

// Parse parses raw JSON or YAML bytes as an OpenAPI 3.x or Swagger 2.0
// document and returns the flattened list of routes it defines. It performs
// no network I/O: any external/remote $ref is left unresolved with a
// warning rather than being fetched.
func Parse(data []byte) (*Result, error) {
	doc, err := decodeDocument(data)
	if err != nil {
		return nil, err
	}

	resolver := newRefResolver(doc)
	resolvedAny, err := resolver.Resolve(doc, 0)
	if err != nil {
		return nil, err
	}
	resolved, ok := resolvedAny.(map[string]any)
	if !ok {
		return nil, ErrUnsupportedSpec
	}

	format, err := detectFormat(resolved)
	if err != nil {
		return nil, err
	}

	result := &Result{Format: format}
	if info, ok := asMap(resolved["info"]); ok {
		result.Title = asString(info["title"])
		result.Version = asString(info["version"])
	}

	switch format {
	case FormatOpenAPI3:
		result.BaseURL = openapi3BaseURL(resolved)
	case FormatSwagger2:
		result.BaseURL = swagger2BaseURL(resolved)
	}

	pathsRaw, ok := resolved["paths"]
	if !ok {
		return nil, ErrNoPaths
	}
	paths, ok := asMap(pathsRaw)
	if !ok || len(paths) == 0 {
		return nil, ErrNoPaths
	}

	globalSecurity := resolved["security"]

	// Sort path keys for deterministic output (map iteration order is random).
	pathKeys := make([]string, 0, len(paths))
	for k := range paths {
		pathKeys = append(pathKeys, k)
	}
	sort.Strings(pathKeys)

	routes := make([]Route, 0, len(pathKeys))
	for _, path := range pathKeys {
		pathItem, ok := asMap(paths[path])
		if !ok {
			continue
		}
		commonParams, _ := asSlice(pathItem["parameters"])

		methodKeys := make([]string, 0, 8)
		for k := range pathItem {
			if httpMethods[strings.ToLower(k)] {
				methodKeys = append(methodKeys, k)
			}
		}
		sort.Strings(methodKeys)

		for _, methodKey := range methodKeys {
			op, ok := asMap(pathItem[methodKey])
			if !ok {
				continue
			}
			if len(routes) >= MaxOperations {
				return nil, ErrTooManyOperations
			}
			opParams, _ := asSlice(op["parameters"])
			route := Route{
				Method:      strings.ToUpper(methodKey),
				Path:        path,
				OperationID: asString(op["operationId"]),
				Summary:     asString(op["summary"]),
				Description: asString(op["description"]),
				Tags:        asStringSlice(op["tags"]),
				Deprecated:  asBool(op["deprecated"]),
				Parameters:  mergeParameters(commonParams, opParams),
				Responses:   op["responses"],
			}
			if sec, ok := op["security"]; ok {
				route.Security = sec
			} else {
				route.Security = globalSecurity
			}
			if format == FormatOpenAPI3 {
				route.RequestBody = op["requestBody"]
			} else {
				route.RequestBody = extractSwaggerBody(route.Parameters)
			}
			route.SpecHash = computeSpecHash(route)
			routes = append(routes, route)
		}
	}

	if len(routes) == 0 {
		return nil, ErrNoPaths
	}
	result.Routes = routes
	return result, nil
}

func detectFormat(doc map[string]any) (string, error) {
	if v := asString(doc["openapi"]); strings.HasPrefix(v, "3.") {
		return FormatOpenAPI3, nil
	}
	if v := asString(doc["swagger"]); v == "2.0" {
		return FormatSwagger2, nil
	}
	return "", ErrUnsupportedSpec
}

func openapi3BaseURL(doc map[string]any) string {
	servers, ok := asSlice(doc["servers"])
	if !ok || len(servers) == 0 {
		return ""
	}
	first, ok := asMap(servers[0])
	if !ok {
		return ""
	}
	return strings.TrimRight(asString(first["url"]), "/")
}

func swagger2BaseURL(doc map[string]any) string {
	host := asString(doc["host"])
	if host == "" {
		return ""
	}
	scheme := "https"
	if schemes := asStringSlice(doc["schemes"]); len(schemes) > 0 {
		scheme = schemes[0]
	}
	basePath := asString(doc["basePath"])
	return fmt.Sprintf("%s://%s%s", scheme, host, strings.TrimRight(basePath, "/"))
}

func mergeParameters(common, operation []any) []Parameter {
	byKey := map[string]Parameter{}
	order := []string{}
	add := func(list []any) {
		for _, raw := range list {
			m, ok := asMap(raw)
			if !ok {
				continue
			}
			p := Parameter{
				Name:        asString(m["name"]),
				In:          asString(m["in"]),
				Required:    asBool(m["required"]),
				Description: asString(m["description"]),
			}
			if schema, ok := asMap(m["schema"]); ok {
				p.Type = asString(schema["type"])
				p.Default = asScalarString(schema["default"])
				p.Example = asScalarString(schema["example"])
			} else {
				p.Type = asString(m["type"])
				p.Default = asScalarString(m["default"])
			}
			key := p.In + ":" + p.Name
			if _, exists := byKey[key]; !exists {
				order = append(order, key)
			}
			byKey[key] = p
		}
	}
	add(common)
	add(operation)
	out := make([]Parameter, 0, len(order))
	for _, k := range order {
		out = append(out, byKey[k])
	}
	return out
}

func extractSwaggerBody(params []Parameter) any {
	for _, p := range params {
		if p.In == "body" {
			return map[string]any{"in": "body", "name": p.Name, "type": p.Type}
		}
	}
	return nil
}
