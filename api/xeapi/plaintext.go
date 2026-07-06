// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package xeapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type plaintextEnvelope struct {
	ContentType string `json:"contentType,omitempty"`
	Method      string `json:"method,omitempty"`
	QueryString string `json:"queryString,omitempty"`
	Body        string `json:"body,omitempty"`
}

func buildPlaintextEnvelope(req EncryptRequest) ([]byte, error) {
	contentType := strings.TrimSpace(req.ContentType)
	if contentType == "" {
		contentType = "application/x-www-form-urlencoded;charset=utf-8"
	}
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodPost
	}

	env := plaintextEnvelope{}
	if mediaType != "application/x-www-form-urlencoded" {
		env.ContentType = contentType
	}
	if method != http.MethodPost {
		env.Method = method
	}

	parsed, err := url.Parse(req.URI)
	if err != nil {
		return nil, fmt.Errorf("url.Parse: %w", err)
	}
	env.QueryString = strings.TrimPrefix(parsed.RawQuery, "?")
	if env.QueryString != "" {
		env.QueryString += "&e_r=true"
	} else {
		env.QueryString = "e_r=true"
	}

	if req.Data != nil {
		body, err := formBody(req.Data)
		if err != nil {
			return nil, err
		}
		if len(body) > 0 {
			env.Body = base64.StdEncoding.EncodeToString(body)
		}
	}

	data, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal plaintext envelope: %w", err)
	}
	return data, nil
}

func formBody(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal form body: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var fields map[string]interface{}
	if err := decoder.Decode(&fields); err != nil {
		return nil, fmt.Errorf("json.Decode form body: %w", err)
	}
	delete(fields, "e_r")

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	values := url.Values{}
	for _, key := range keys {
		value, ok, err := formValue(fields[key])
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", key, err)
		}
		if ok {
			values.Set(key, value)
		}
	}
	return []byte(values.Encode()), nil
}

func formValue(v interface{}) (string, bool, error) {
	switch value := v.(type) {
	case nil:
		return "", false, nil
	case string:
		return value, true, nil
	case bool:
		if value {
			return "true", true, nil
		}
		return "false", true, nil
	case json.Number:
		return value.String(), true, nil
	case float64:
		return fmt.Sprintf("%v", value), true, nil
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return "", false, err
		}
		return string(data), true, nil
	}
}
