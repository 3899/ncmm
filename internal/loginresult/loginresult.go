package loginresult

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	Version = 1
	Marker  = "NCMM_LOGIN_RESULT "
)

type Result struct {
	Version     int    `json:"version"`
	UID         int64  `json:"uid"`
	Nickname    string `json:"nickname"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
	CookiePath  string `json:"cookiePath"`
	AccountPath string `json:"accountPath"`
	Main        bool   `json:"main"`
}

func Write(w io.Writer, result Result) error {
	result.Version = Version
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s%s\n", Marker, data)
	return err
}

func Parse(output string) (Result, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	buffer := make([]byte, 0, 64*1024)
	scanner.Buffer(buffer, 1<<20)
	var encoded string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, Marker) {
			encoded = strings.TrimSpace(strings.TrimPrefix(line, Marker))
		}
	}
	if err := scanner.Err(); err != nil {
		return Result{}, err
	}
	if encoded == "" {
		return Result{}, fmt.Errorf("login process did not return a structured result")
	}
	var result Result
	if err := json.Unmarshal([]byte(encoded), &result); err != nil {
		return Result{}, fmt.Errorf("decode login result: %w", err)
	}
	if result.Version != Version {
		return Result{}, fmt.Errorf("unsupported login result version %d", result.Version)
	}
	if result.UID <= 0 || strings.TrimSpace(result.CookiePath) == "" || strings.TrimSpace(result.AccountPath) == "" {
		return Result{}, fmt.Errorf("login process returned an incomplete result")
	}
	return result, nil
}
