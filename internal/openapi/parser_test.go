package openapi

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestParseSwagger2(t *testing.T) {
	data, err := os.ReadFile("testdata/swagger2_petstore.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Format != FormatSwagger2 {
		t.Fatalf("expected swagger2 format, got %s", result.Format)
	}
	if result.BaseURL != "https://petstore.example.com/v2" {
		t.Fatalf("unexpected base url: %s", result.BaseURL)
	}
	if len(result.Routes) != 4 {
		t.Fatalf("expected 4 routes, got %d", len(result.Routes))
	}
	var getPet *Route
	for i := range result.Routes {
		if result.Routes[i].OperationID == "getPet" {
			getPet = &result.Routes[i]
		}
	}
	if getPet == nil {
		t.Fatal("expected getPet route to be present")
	}
	if getPet.Method != "GET" || getPet.Path != "/pets/{petId}" {
		t.Fatalf("unexpected getPet route: %+v", getPet)
	}
	// Path-level parameter must be merged in.
	foundPathParam := false
	for _, p := range getPet.Parameters {
		if p.Name == "petId" && p.In == "path" {
			foundPathParam = true
		}
	}
	if !foundPathParam {
		t.Fatalf("expected petId path parameter to be merged, got %+v", getPet.Parameters)
	}
	// $ref inside responses.schema must be resolved, not left dangling.
	responses, ok := getPet.Responses.(map[string]any)
	if !ok {
		t.Fatalf("expected responses to be a map, got %T", getPet.Responses)
	}
	ok200, ok := responses["200"].(map[string]any)
	if !ok {
		t.Fatalf("expected 200 response object")
	}
	schema, ok := ok200["schema"].(map[string]any)
	if !ok {
		t.Fatalf("expected schema to be resolved to a map, got %v", ok200["schema"])
	}
	if _, hasRef := schema["$ref"]; hasRef {
		t.Fatalf("expected $ref to be resolved, still present: %v", schema)
	}
	if _, hasProps := schema["properties"]; !hasProps {
		t.Fatalf("expected resolved schema to contain properties, got %v", schema)
	}
}

func TestParseOpenAPI3YAML(t *testing.T) {
	data, err := os.ReadFile("testdata/openapi3_petstore.yaml")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Format != FormatOpenAPI3 {
		t.Fatalf("expected openapi3 format, got %s", result.Format)
	}
	if result.BaseURL != "https://api.example.com/v3" {
		t.Fatalf("unexpected base url: %s", result.BaseURL)
	}
	if len(result.Routes) != 4 {
		t.Fatalf("expected 4 routes, got %d", len(result.Routes))
	}
	var deletePet *Route
	for i := range result.Routes {
		if result.Routes[i].OperationID == "deletePet" {
			deletePet = &result.Routes[i]
		}
	}
	if deletePet == nil || !deletePet.Deprecated {
		t.Fatalf("expected deletePet to be marked deprecated: %+v", deletePet)
	}
}

func TestParseRejectsMalformedInput(t *testing.T) {
	if _, err := Parse([]byte("{not valid json or yaml: [")); err == nil {
		t.Fatal("expected an error for malformed input")
	}
}

func TestParseRejectsUnsupportedSpec(t *testing.T) {
	_, err := Parse([]byte(`{"paths": {}}`))
	if err == nil {
		t.Fatal("expected an error for a document without version markers")
	}
}

func TestParseRejectsTooLarge(t *testing.T) {
	huge := bytes.Repeat([]byte("a"), MaxDocumentBytes+1)
	if _, err := Parse(huge); err != ErrTooLarge {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}
}

func TestParseHandlesCircularRefsWithoutHanging(t *testing.T) {
	spec := []byte(`{
		"openapi": "3.0.0",
		"info": {"title": "t", "version": "1"},
		"components": {"schemas": {"A": {"type": "object", "properties": {"self": {"$ref": "#/components/schemas/A"}}}}},
		"paths": {"/a": {"get": {"operationId": "getA", "responses": {"200": {"description": "ok", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/A"}}}}}}}}
	}`)
	result, err := Parse(spec)
	if err != nil {
		t.Fatalf("unexpected error resolving circular ref: %v", err)
	}
	if len(result.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(result.Routes))
	}
}

func TestParseGeneratesStableHashAndDetectsChanges(t *testing.T) {
	data, _ := os.ReadFile("testdata/swagger2_petstore.json")
	r1, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Routes[0].SpecHash != r2.Routes[0].SpecHash {
		t.Fatal("expected identical hash for identical spec parses")
	}

	modified := bytes.Replace(data, []byte("List all pets"), []byte("List every pet"), 1)
	r3, err := Parse(modified)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Routes[0].SpecHash == r3.Routes[0].SpecHash {
		t.Fatal("expected hash to change when the summary changes")
	}
}

// TestParseLargeSpecification builds and parses a specification with more
// than 500 operations, matching the acceptance criteria for bulk imports.
func TestParseLargeSpecification(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"openapi":"3.0.0","info":{"title":"Big API","version":"1.0"},"servers":[{"url":"https://big.example.com"}],"paths":{`)
	const resourceCount = 130 // 130 resources * 4 methods = 520 operations
	for i := 0; i < resourceCount; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `"/resource%d/{id}":{`, i)
		fmt.Fprintf(&b, `"get":{"operationId":"getResource%d","tags":["resource%d"],"responses":{"200":{"description":"ok"}}},`, i, i)
		fmt.Fprintf(&b, `"put":{"operationId":"putResource%d","tags":["resource%d"],"responses":{"200":{"description":"ok"}}},`, i, i)
		fmt.Fprintf(&b, `"delete":{"operationId":"deleteResource%d","tags":["resource%d"],"responses":{"204":{"description":"ok"}}},`, i, i)
		fmt.Fprintf(&b, `"patch":{"operationId":"patchResource%d","tags":["resource%d"],"responses":{"200":{"description":"ok"}}}`, i, i)
		b.WriteString("}")
	}
	b.WriteString("}}")

	result, err := Parse([]byte(b.String()))
	if err != nil {
		t.Fatalf("unexpected error parsing large spec: %v", err)
	}
	if len(result.Routes) != resourceCount*4 {
		t.Fatalf("expected %d routes, got %d", resourceCount*4, len(result.Routes))
	}
}

func TestParseRejectsTooManyOperations(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"openapi":"3.0.0","info":{"title":"Huge","version":"1.0"},"paths":{`)
	for i := 0; i < MaxOperations+10; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `"/r%d":{"get":{"operationId":"op%d","responses":{"200":{"description":"ok"}}}}`, i, i)
	}
	b.WriteString("}}")
	_, err := Parse([]byte(b.String()))
	if err != ErrTooManyOperations {
		t.Fatalf("expected ErrTooManyOperations, got %v", err)
	}
}
