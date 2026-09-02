package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/3899/ncmm/config"
	"gopkg.in/yaml.v3"
)

func TestConfigurationSchemaPathsAreUniqueAndDefaultsMatch(t *testing.T) {
	schema := configurationSchema()
	if schema.SchemaVersion <= 0 {
		t.Fatal("schema version must be positive")
	}
	var defaults any
	if err := yaml.Unmarshal(config.DefaultYAML(), &defaults); err != nil {
		t.Fatal(err)
	}

	for targetName, target := range schema.Targets {
		seen := make(map[string]struct{}, len(target.Fields))
		groups := make(map[string]struct{})
		for _, category := range target.Categories {
			for _, group := range category.Groups {
				groups[group.ID] = struct{}{}
			}
		}
		for _, field := range target.Fields {
			if _, exists := seen[field.Path]; exists {
				t.Fatalf("%s schema path %q is duplicated", targetName, field.Path)
			}
			seen[field.Path] = struct{}{}
			if _, exists := groups[field.Group]; !exists {
				t.Fatalf("%s schema field %q uses unknown group %q", targetName, field.Path, field.Group)
			}
			if targetName != "config" {
				continue
			}
			value, exists := schemaValueAt(defaults, field.Path)
			if !exists {
				continue // Optional fields may be absent from the compact default YAML.
			}
			if !reflect.DeepEqual(value, field.Default) {
				t.Fatalf("default for %q = %#v; want %#v", field.Path, field.Default, value)
			}
		}
	}
}

func TestMainConfigurationSchemaCoversDefaultFields(t *testing.T) {
	schema := configurationSchema().Targets["config"]
	if len(schema.Categories) == 0 || schema.Categories[0].ID != "task" {
		t.Fatalf("first config category = %q; want task", schema.Categories[0].ID)
	}
	registered := make(map[string]struct{}, len(schema.Fields))
	for _, field := range schema.Fields {
		registered[field.Path] = struct{}{}
	}
	var defaults any
	if err := yaml.Unmarshal(config.DefaultYAML(), &defaults); err != nil {
		t.Fatal(err)
	}
	var missing []string
	collectUnregisteredSchemaPaths(defaults, nil, registered, &missing)
	if len(missing) > 0 {
		t.Fatalf("default configuration fields missing from schema: %s", strings.Join(missing, ", "))
	}
}

func collectUnregisteredSchemaPaths(value any, path []string, registered map[string]struct{}, missing *[]string) {
	joined := strings.Join(path, ".")
	if _, ok := registered[joined]; ok {
		return
	}
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if len(path) == 0 && key == "version" {
				continue
			}
			collectUnregisteredSchemaPaths(child, append(path, key), registered, missing)
		}
	case []any:
		if len(path) > 0 {
			*missing = append(*missing, joined)
		}
	default:
		if len(path) > 0 {
			*missing = append(*missing, joined)
		}
	}
}

func TestConfigSchemaAPI(t *testing.T) {
	server := newAuthTestServer(t, t.TempDir())
	credentials, err := server.authManager.Setup(context.Background(), "Admin#123", requestClientInfo(httptest.NewRequest(http.MethodGet, "/", nil)))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/config/schema", nil)
	request.Host = "localhost:3899"
	request.AddCookie(&http.Cookie{Name: "ncmm_session", Value: credentials.Token})
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("schema status = %d, body = %s", response.Code, response.Body.String())
	}
	var got configSchemaView
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != configSchemaVersion || len(got.Targets["config"].Fields) == 0 || len(got.Targets["notify"].Fields) == 0 {
		t.Fatalf("unexpected schema response: %+v", got)
	}
}
