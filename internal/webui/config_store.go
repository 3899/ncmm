package webui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/3899/ncmm/config"
	"github.com/3899/ncmm/internal/filelock"
	"github.com/3899/ncmm/internal/loginresult"
	"github.com/3899/ncmm/pkg/notify"
	"gopkg.in/yaml.v3"
)

var (
	errConfigRevisionRequired = errors.New("config revision is required")
	errConfigRevisionConflict = errors.New("config revision conflict")
)

const configLockTimeout = 10 * time.Second

type configDocument struct {
	Revision        string                    `json:"revision"`
	ModifiedAt      time.Time                 `json:"modifiedAt"`
	Raw             string                    `json:"raw"`
	Data            any                       `json:"data"`
	Descriptions    map[string]string         `json:"descriptions,omitempty"`
	ParseError      string                    `json:"parseError,omitempty"`
	Sections        map[string]string         `json:"sections,omitempty"`
	AccountNames    map[string]string         `json:"accountNames,omitempty"`
	AccountProfiles map[string]accountProfile `json:"accountProfiles,omitempty"`
}

type accountProfile struct {
	Nickname  string `json:"nickname,omitempty"`
	AvatarURL string `json:"avatarUrl,omitempty"`
}

func withYAMLSections(document configDocument) configDocument {
	if document.ParseError == "" {
		document.Sections = extractYAMLSections(document.Raw)
	}
	return document
}

func extractYAMLSections(raw string) map[string]string {
	var document yaml.Node
	if yaml.Unmarshal([]byte(raw), &document) != nil || len(document.Content) == 0 {
		return nil
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	sections := make(map[string]string, len(root.Content)/2)
	for i := 0; i+1 < len(root.Content); i += 2 {
		section := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{
			Kind: yaml.MappingNode, Tag: "!!map",
			Content: []*yaml.Node{cloneYAMLNode(root.Content[i]), cloneYAMLNode(root.Content[i+1])},
		}}}
		var output bytes.Buffer
		encoder := yaml.NewEncoder(&output)
		encoder.SetIndent(2)
		if encoder.Encode(section) == nil && encoder.Close() == nil {
			sections[root.Content[i].Value] = output.String()
		}
	}
	return sections
}

type configStore struct {
	mu       sync.Mutex
	path     string
	validate func(string) error
}

func newConfigStore(path string) *configStore {
	return newYAMLStore(path, validateConfigFile)
}

func newYAMLStore(path string, validate func(string) error) *configStore {
	return &configStore{path: path, validate: validate}
}

func newNotifyStore(path string) (*configStore, error) {
	store := newYAMLStore(path, validateNotifyFile)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	lock, err := acquireConfigLock(path)
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		data, marshalErr := yaml.Marshal(defaultNotifyConfig())
		if marshalErr != nil {
			return nil, marshalErr
		}
		if err := writeFileAtomic(path, data, 0600); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return store, nil
}

func (s *configStore) get() (configDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := acquireConfigLock(s.path)
	if err != nil {
		return configDocument{}, err
	}
	defer lock.Close()
	return s.getLocked()
}

func (s *configStore) getLocked() (configDocument, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return configDocument{}, err
	}
	document := configDocument{Revision: revision(data), Raw: string(data)}
	if info, statErr := os.Stat(s.path); statErr == nil {
		document.ModifiedAt = info.ModTime()
	}
	var parsed any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		document.ParseError = fmt.Errorf("parse config: %w", err).Error()
		return document, nil
	}
	var yamlDocument yaml.Node
	if err := yaml.Unmarshal(data, &yamlDocument); err != nil {
		document.ParseError = fmt.Errorf("parse config comments: %w", err).Error()
		return document, nil
	}
	document.Data = parsed
	document.Descriptions = collectYAMLDescriptions(&yamlDocument)
	document.AccountProfiles = collectAccountProfiles(&yamlDocument)
	document.AccountNames = make(map[string]string, len(document.AccountProfiles))
	for path, profile := range document.AccountProfiles {
		if profile.Nickname != "" {
			document.AccountNames[path] = profile.Nickname
		}
	}
	return document, nil
}

func (s *configStore) saveRaw(expectedRevision, raw string) (configDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := acquireConfigLock(s.path)
	if err != nil {
		return configDocument{}, err
	}
	defer lock.Close()
	if _, err := s.checkRevisionLocked(expectedRevision); err != nil {
		return configDocument{}, err
	}
	data := []byte(raw)
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	if err := s.validateAndWriteLocked(data); err != nil {
		return configDocument{}, err
	}
	return s.getLocked()
}

func (s *configStore) saveSectionRaw(expectedRevision, section, raw string) (configDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := acquireConfigLock(s.path)
	if err != nil {
		return configDocument{}, err
	}
	defer lock.Close()
	existing, err := s.checkRevisionLocked(expectedRevision)
	if err != nil {
		return configDocument{}, err
	}
	var source yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &source); err != nil {
		return configDocument{}, fmt.Errorf("parse section YAML: %w", err)
	}
	if len(source.Content) == 0 || source.Content[0].Kind != yaml.MappingNode || len(source.Content[0].Content) != 2 || source.Content[0].Content[0].Value != section {
		return configDocument{}, fmt.Errorf("section YAML must contain only the %q top-level key", section)
	}
	var destination yaml.Node
	if err := yaml.Unmarshal(existing, &destination); err != nil {
		return configDocument{}, err
	}
	if len(destination.Content) == 0 || destination.Content[0].Kind != yaml.MappingNode {
		return configDocument{}, fmt.Errorf("YAML root must be a mapping")
	}
	root := destination.Content[0]
	replaced := false
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == section {
			root.Content[i+1] = cloneYAMLNode(source.Content[0].Content[1])
			replaced = true
			break
		}
	}
	if !replaced {
		root.Content = append(root.Content, cloneYAMLNode(source.Content[0].Content[0]), cloneYAMLNode(source.Content[0].Content[1]))
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(&destination); err != nil {
		return configDocument{}, err
	}
	if err := encoder.Close(); err != nil {
		return configDocument{}, err
	}
	if err := s.validateAndWriteLocked(output.Bytes()); err != nil {
		return configDocument{}, err
	}
	return s.getLocked()
}

func collectAccountNicknames(document *yaml.Node) map[string]string {
	names := make(map[string]string)
	for path, profile := range collectAccountProfiles(document) {
		if profile.Nickname != "" {
			names[path] = profile.Nickname
		}
	}
	return names
}

func collectAccountProfiles(document *yaml.Node) map[string]accountProfile {
	profiles := make(map[string]accountProfile)
	if document == nil || document.Kind != yaml.DocumentNode || len(document.Content) == 0 {
		return profiles
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return profiles
	}
	var accounts *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "accounts" {
			accounts = root.Content[i+1]
			break
		}
	}
	if accounts == nil || accounts.Kind != yaml.MappingNode {
		return profiles
	}
	for i := 0; i+1 < len(accounts.Content); i += 2 {
		key, value := accounts.Content[i].Value, accounts.Content[i+1]
		switch key {
		case "main", "primary":
			if value.Kind == yaml.ScalarNode {
				if profile := accountProfileFromNode(value); profile.Nickname != "" || profile.AvatarURL != "" {
					profiles[value.Value] = profile
				}
			}
		case "secondary":
			if value.Kind != yaml.SequenceNode {
				continue
			}
			for _, item := range value.Content {
				if item.Kind == yaml.ScalarNode {
					if profile := accountProfileFromNode(item); profile.Nickname != "" || profile.AvatarURL != "" {
						profiles[item.Value] = profile
					}
				}
			}
		}
	}
	return profiles
}

func accountNickname(node *yaml.Node) string {
	return accountProfileFromNode(node).Nickname
}

func accountProfileFromNode(node *yaml.Node) accountProfile {
	var profile accountProfile
	for _, comment := range []string{node.LineComment, node.HeadComment} {
		comment = normalizeYAMLComment(comment)
		for _, line := range strings.Split(comment, "\n") {
			for _, part := range strings.Split(line, "|") {
				part = strings.TrimSpace(part)
				for _, separator := range []string{":", "："} {
					label, value, ok := strings.Cut(part, separator)
					if !ok {
						continue
					}
					switch strings.TrimSpace(label) {
					case "昵称":
						profile.Nickname = strings.TrimSpace(value)
					case "头像":
						profile.AvatarURL = strings.TrimSpace(value)
					}
				}
			}
		}
	}
	return profile
}

func (s *configStore) saveData(expectedRevision string, value any) (configDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := acquireConfigLock(s.path)
	if err != nil {
		return configDocument{}, err
	}
	defer lock.Close()
	existing, err := s.checkRevisionLocked(expectedRevision)
	if err != nil {
		return configDocument{}, err
	}
	var dst yaml.Node
	if err := yaml.Unmarshal(existing, &dst); err != nil {
		return configDocument{}, err
	}
	jsonData, err := json.Marshal(value)
	if err != nil {
		return configDocument{}, err
	}
	var generic any
	if err := json.Unmarshal(jsonData, &generic); err != nil {
		return configDocument{}, err
	}
	srcData, err := yaml.Marshal(generic)
	if err != nil {
		return configDocument{}, err
	}
	var src yaml.Node
	if err := yaml.Unmarshal(srcData, &src); err != nil {
		return configDocument{}, err
	}
	mergeYAMLNode(&dst, &src)
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&dst); err != nil {
		return configDocument{}, err
	}
	if err := enc.Close(); err != nil {
		return configDocument{}, err
	}
	if err := s.validateAndWriteLocked(out.Bytes()); err != nil {
		return configDocument{}, err
	}
	return s.getLocked()
}

func (s *configStore) checkRevisionLocked(expected string) ([]byte, error) {
	if strings.TrimSpace(expected) == "" {
		return nil, errConfigRevisionRequired
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	if revision(data) != expected {
		return nil, fmt.Errorf("%w: config changed since it was loaded; refresh and try again", errConfigRevisionConflict)
	}
	return data, nil
}

func (s *configStore) validateAndWriteLocked(data []byte) error {
	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".ncmm-config-*.yaml")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if s.validate != nil {
		if err := s.validate(tmpPath); err != nil {
			return err
		}
	}
	if current, err := os.ReadFile(s.path); err == nil {
		if err := writeFileAtomic(s.path+".bak", current, 0600); err != nil {
			return fmt.Errorf("write config backup: %w", err)
		}
	}
	return writeFileAtomic(s.path, data, 0600)
}

func (s *configStore) updateAccount(expectedRevision string, result loginresult.Result) (configDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := acquireConfigLock(s.path)
	if err != nil {
		return configDocument{}, err
	}
	defer lock.Close()
	existing, err := s.checkRevisionLocked(expectedRevision)
	if err != nil {
		return configDocument{}, err
	}
	var document yaml.Node
	if err := yaml.Unmarshal(existing, &document); err != nil {
		return configDocument{}, fmt.Errorf("parse config: %w", err)
	}
	if err := config.ApplyAccountProfileUpdate(&document, result.AccountPath, result.Nickname, result.AvatarURL, result.Main); err != nil {
		return configDocument{}, err
	}
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&document); err != nil {
		return configDocument{}, err
	}
	if err := encoder.Close(); err != nil {
		return configDocument{}, err
	}
	if err := s.validateAndWriteLocked(out.Bytes()); err != nil {
		return configDocument{}, err
	}
	return s.getLocked()
}

func acquireConfigLock(path string) (*filelock.Lock, error) {
	ctx, cancel := context.WithTimeout(context.Background(), configLockTimeout)
	defer cancel()
	lock, err := filelock.Acquire(ctx, path+".lock")
	if err != nil {
		return nil, fmt.Errorf("acquire config lock: %w", err)
	}
	return lock, nil
}

func validateConfigFile(path string) error {
	if _, err := config.New(path); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}
	return nil
}

func validateNotifyFile(path string) error {
	if _, _, err := notify.LoadChannels(path); err != nil {
		return fmt.Errorf("notify validation failed: %w", err)
	}
	return nil
}

func defaultNotifyConfig() notify.ChannelsConfig {
	return notify.ChannelsConfig{
		Webhook:  notify.WebhookConfig{Method: "POST", Headers: map[string]string{}},
		CoolPush: notify.CoolPushConfig{Mode: "send"},
		WeComApp: notify.WeComAppConfig{ToUser: "@all"},
	}
}

func revision(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func mergeYAMLNode(dst, src *yaml.Node) {
	if dst.Kind == yaml.DocumentNode && src.Kind == yaml.DocumentNode && len(dst.Content) > 0 && len(src.Content) > 0 {
		mergeYAMLNode(dst.Content[0], src.Content[0])
		return
	}
	if dst.Kind == yaml.MappingNode && src.Kind == yaml.MappingNode {
		source := make(map[string][2]*yaml.Node, len(src.Content)/2)
		for i := 0; i+1 < len(src.Content); i += 2 {
			source[src.Content[i].Value] = [2]*yaml.Node{src.Content[i], src.Content[i+1]}
		}
		merged := make([]*yaml.Node, 0, len(src.Content))
		seen := make(map[string]bool, len(source))
		for i := 0; i+1 < len(dst.Content); i += 2 {
			key := dst.Content[i].Value
			pair, ok := source[key]
			if !ok {
				continue
			}
			mergeYAMLNode(dst.Content[i+1], pair[1])
			merged = append(merged, dst.Content[i], dst.Content[i+1])
			seen[key] = true
		}
		for i := 0; i+1 < len(src.Content); i += 2 {
			key := src.Content[i].Value
			if seen[key] {
				continue
			}
			merged = append(merged, cloneYAMLNode(src.Content[i]), cloneYAMLNode(src.Content[i+1]))
		}
		dst.Content = merged
		return
	}
	comments := [3]string{dst.HeadComment, dst.LineComment, dst.FootComment}
	replacement := cloneYAMLNode(src)
	*dst = *replacement
	dst.HeadComment, dst.LineComment, dst.FootComment = comments[0], comments[1], comments[2]
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	copyNode := *node
	copyNode.Content = make([]*yaml.Node, len(node.Content))
	for i, child := range node.Content {
		copyNode.Content[i] = cloneYAMLNode(child)
	}
	return &copyNode
}

func collectYAMLDescriptions(document *yaml.Node) map[string]string {
	descriptions := make(map[string]string)
	if document == nil {
		return descriptions
	}
	collectYAMLDescriptionsNode(document, nil, descriptions)
	return descriptions
}

func collectYAMLDescriptionsNode(node *yaml.Node, path []string, descriptions map[string]string) {
	if node == nil {
		return
	}
	if node.Kind == yaml.DocumentNode {
		for _, child := range node.Content {
			collectYAMLDescriptionsNode(child, path, descriptions)
		}
		return
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			childPath := appendYAMLPath(path, key.Value)
			comment := yamlNodeComment(key, value)
			if i > 0 {
				comment = joinYAMLComments(
					normalizeYAMLComment(node.Content[i-2].FootComment),
					normalizeYAMLComment(node.Content[i-1].FootComment),
					comment,
				)
			}
			if comment != "" {
				descriptions[strings.Join(childPath, ".")] = comment
			}
			collectYAMLDescriptionsNode(value, childPath, descriptions)
		}
		return
	}
	if node.Kind == yaml.SequenceNode {
		for _, child := range node.Content {
			collectYAMLDescriptionsNode(child, path, descriptions)
		}
	}
}

func joinYAMLComments(comments ...string) string {
	seen := make(map[string]bool)
	lines := make([]string, 0, len(comments))
	for _, comment := range comments {
		for _, line := range strings.Split(comment, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !seen[line] {
				seen[line] = true
				lines = append(lines, line)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func appendYAMLPath(path []string, key string) []string {
	result := make([]string, len(path), len(path)+1)
	copy(result, path)
	return append(result, key)
}

func yamlNodeComment(nodes ...*yaml.Node) string {
	parts := make([]string, 0, len(nodes)*2)
	for index, node := range nodes {
		if node == nil {
			continue
		}
		comments := []string{node.HeadComment, node.LineComment}
		if index == 1 {
			comments = []string{node.LineComment}
		}
		for _, comment := range comments {
			if comment = normalizeYAMLComment(comment); comment != "" {
				parts = append(parts, comment)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func normalizeYAMLComment(comment string) string {
	lines := strings.Split(comment, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}
