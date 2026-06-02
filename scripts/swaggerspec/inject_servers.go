// Command inject_servers post-processes a swag/v2 OpenAPI 3.x spec into clean
// input for oapi-codegen and Swagger UI. Three fixes, applied in place (the
// swagger.json sibling is rewritten to match):
//
//  1. Adds the top-level `servers:` block from --server "url|description" flags
//     (URLs live in the Makefile). A {domain} in a url becomes a server variable.
//  2. Collapses swag/v2-rc's single-$ref `oneOf` request bodies to a bare $ref,
//     so oapi-codegen emits flat structs instead of union types.
//  3. Drops the bogus `requestBody` swag/v2-rc adds to `@Accept json` GETs.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// serverFlags collects repeated --server "url|description" values.
type serverFlags []string

func (s *serverFlags) String() string { return strings.Join(*s, ", ") }

func (s *serverFlags) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	specPath := flag.String("spec", "", "path to the swagger.yaml file to rewrite in place")
	domainDefault := flag.String("domain-default", "localhost:9000",
		"default value for the {domain} server variable, when a server url uses it")
	var serverArgs serverFlags
	flag.Var(&serverArgs, "server", `server entry as "url|description" (repeatable)`)
	flag.Parse()

	if *specPath == "" || len(serverArgs) == 0 {
		fmt.Fprintln(os.Stderr, "--spec and at least one --server are required")
		os.Exit(1)
	}

	srv, err := parseServers(serverArgs, *domainDefault)
	if err != nil {
		fmt.Fprintf(os.Stderr, "inject servers: %v\n", err)
		os.Exit(1)
	}

	if err := run(*specPath, srv); err != nil {
		fmt.Fprintf(os.Stderr, "inject servers: %v\n", err)
		os.Exit(1)
	}
}

// parseServers turns "url|description" flag values into OpenAPI server objects,
// attaching a {domain} variable block when a url references that template.
func parseServers(args serverFlags, domainDefault string) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(args))
	for _, a := range args {
		url, desc, ok := strings.Cut(a, "|")
		if !ok || url == "" {
			return nil, fmt.Errorf("invalid --server %q: want \"url|description\"", a)
		}
		entry := map[string]any{
			"url":          url,
			descriptionKey: desc,
		}
		if strings.Contains(url, "{domain}") {
			entry["variables"] = map[string]any{
				"domain": map[string]any{
					"default":      domainDefault,
					descriptionKey: "Hostname of the dev environment",
				},
			}
		}
		out = append(out, entry)
	}
	return out, nil
}

const (
	descriptionKey = "description"
	strTag         = "!!str"
)

func run(specPath string, srv []map[string]any) error {
	// specPath is a developer-supplied generator argument (a path to our own
	// freshly generated spec), not untrusted input.
	raw, err := os.ReadFile(specPath) //nolint:gosec // G304: dev tooling reads its own generated spec
	if err != nil {
		return fmt.Errorf("read spec: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("unexpected spec structure: root is not a mapping")
	}

	setServers(doc.Content[0], srv)
	collapseSingleOneOf(doc.Content[0])
	dropGetRequestBodies(doc.Content[0])

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}
	if err := os.WriteFile(specPath, out, 0o644); err != nil {
		return fmt.Errorf("write yaml: %w", err)
	}

	// Keep the JSON sibling in sync with the rewritten YAML.
	jsonPath := strings.TrimSuffix(specPath, filepath.Ext(specPath)) + ".json"
	if _, err := os.Stat(jsonPath); err == nil {
		var generic any
		if err := yaml.Unmarshal(out, &generic); err != nil {
			return fmt.Errorf("re-parse yaml for json: %w", err)
		}
		generic = normalizeForJSON(generic)
		buf, err := json.MarshalIndent(generic, "", "    ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		if err := os.WriteFile(jsonPath, append(buf, '\n'), 0o644); err != nil {
			return fmt.Errorf("write json: %w", err)
		}
	}

	return nil
}

// setServers replaces (or inserts) the top-level `servers` key in the root
// mapping node, placing it right after `openapi` for readability.
func setServers(root *yaml.Node, srv []map[string]any) {
	var serversNode yaml.Node
	_ = serversNode.Encode(srv)

	// Drop any existing servers key.
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "servers" {
			root.Content = append(root.Content[:i], root.Content[i+2:]...)
			break
		}
	}

	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: strTag, Value: "servers"}

	// Insert after the `openapi` key if present, else append.
	insertAt := len(root.Content)
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "openapi" {
			insertAt = i + 2
			break
		}
	}
	tail := append([]*yaml.Node{keyNode, &serversNode}, root.Content[insertAt:]...)
	root.Content = append(root.Content[:insertAt], tail...)
}

// collapseSingleOneOf walks the document and rewrites every mapping node that
// has a `oneOf` with exactly one member whose first key is `$ref` into a plain
// `{$ref: ...}` node. This undoes swag/v2-rc's habit of wrapping request-body
// refs in a one-member oneOf (with stray description/summary siblings), which
// otherwise makes oapi-codegen emit union types instead of flat structs.
func collapseSingleOneOf(n *yaml.Node) {
	if n == nil {
		return
	}

	if n.Kind == yaml.MappingNode {
		if ref := collapsibleOneOfRef(n); ref != "" {
			// Replace the whole mapping node with a bare {$ref: ...}.
			n.Content = []*yaml.Node{
				{Kind: yaml.ScalarNode, Tag: strTag, Value: "$ref"},
				{Kind: yaml.ScalarNode, Tag: strTag, Value: ref},
			}
			return
		}
	}

	for _, c := range n.Content {
		collapseSingleOneOf(c)
	}
}

// collapsibleOneOfRef returns the lone `$ref` of a mapping node's `oneOf` when
// that oneOf consists of exactly one $ref member plus only empty `{type:
// object}` placeholders (swag/v2-rc's wrapping). Otherwise it returns "".
func collapsibleOneOfRef(n *yaml.Node) string {
	seq := mappingValue(n, "oneOf")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return ""
	}

	ref := ""
	for _, member := range seq.Content {
		if member.Kind != yaml.MappingNode {
			return ""
		}
		if r := refValue(member); r != "" {
			if ref != "" {
				return "" // more than one $ref — a genuine union
			}
			ref = r
			continue
		}
		if !isEmptyObjectSchema(member) {
			return ""
		}
	}
	return ref
}

// dropGetRequestBodies removes the `requestBody` from every GET operation.
// swag/v2-rc attaches one whenever a GET handler is annotated with `@Accept
// json`, which is meaningless for GET and forces oapi-codegen to add a useless
// body parameter to the generated GET client methods.
func dropGetRequestBodies(root *yaml.Node) {
	paths := mappingValue(root, "paths")
	if paths == nil {
		return
	}
	for i := 0; i+1 < len(paths.Content); i += 2 {
		item := paths.Content[i+1] // path item mapping
		if item.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(item.Content); j += 2 {
			if item.Content[j].Value != "get" {
				continue
			}
			op := item.Content[j+1]
			if op.Kind != yaml.MappingNode {
				continue
			}
			for k := 0; k+1 < len(op.Content); k += 2 {
				if op.Content[k].Value == "requestBody" {
					op.Content = append(op.Content[:k], op.Content[k+2:]...)
					break
				}
			}
		}
	}
}

// mappingValue returns the value node for key in a mapping node, or nil.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// isEmptyObjectSchema reports whether a mapping node is just `{type: object}`
// (the stray placeholder swag/v2-rc adds inside oneOf).
func isEmptyObjectSchema(m *yaml.Node) bool {
	if len(m.Content) != 2 {
		return false
	}
	return m.Content[0].Value == "type" && m.Content[1].Value == "object"
}

// refValue returns the value of the `$ref` key in a mapping node, or "".
func refValue(m *yaml.Node) string {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == "$ref" {
			return m.Content[i+1].Value
		}
	}
	return ""
}

// normalizeForJSON converts map[interface{}]interface{} (produced by yaml.v3 in
// some cases) into map[string]interface{} so encoding/json accepts it.
func normalizeForJSON(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			t[k] = normalizeForJSON(val)
		}
		return t
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[fmt.Sprintf("%v", k)] = normalizeForJSON(val)
		}
		return m
	case []any:
		for i, val := range t {
			t[i] = normalizeForJSON(val)
		}
		return t
	default:
		return v
	}
}
