// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package config

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/3899/ncmm/api"
	"github.com/3899/ncmm/pkg/database"
	"github.com/3899/ncmm/pkg/log"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var HomeDir string

var (
	//go:embed config.yaml
	defaultConfigByte []byte
	defaultConfig     *Config
)

func init() {
	var err error
	if os.Getenv("NCMM_DOCKER_OFFICIAL") == "1" {
		HomeDir = "/data"
	} else {
		var exePath string
		exePath, err = os.Executable()
		if err == nil {
			HomeDir = filepath.Dir(exePath)
		} else {
			HomeDir = "."
		}
	}
	if err := yaml.Unmarshal(defaultConfigByte, &defaultConfig); err != nil {
		panic(fmt.Sprintf("defaultConfig.Unmarshal: %s", err))
	}
	defaultConfig.syncMainCookie()
	// defaultConfig.ReplaceMagicVariables("HOME", HomeDir)
	if err := defaultConfig.Validate(); err != nil {
		panic(fmt.Sprintf("defaultConfig.Validate: %s", err))
	}
}

func decodeB64(s string) string {
	b, _ := base64.StdEncoding.DecodeString(s)
	return string(b)
}

type AccountsConf struct {
	Main             string            `json:"main" yaml:"main"`
	Primary          string            `json:"primary" yaml:"primary"` // 兼容旧版
	Secondary        []string          `json:"secondary" yaml:"secondary"`
	AntiCheatTokens  map[string]string `json:"antiCheatTokens" yaml:"antiCheatTokens"`
}

// AntiCheatTokenFor returns the antiCheatToken for the given cookie file path.
// It tries both the original path and the absolute path for matching.
func (a *AccountsConf) AntiCheatTokenFor(cookiePath string) string {
	if a == nil || a.AntiCheatTokens == nil {
		return ""
	}
	if v, ok := a.AntiCheatTokens[cookiePath]; ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	abs, err := filepath.Abs(cookiePath)
	if err == nil && abs != cookiePath {
		if v, ok := a.AntiCheatTokens[abs]; ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

type YunbeiTaskConf struct {
	EnableViewVipCenter      bool `json:"enableViewVipCenter" yaml:"enableViewVipCenter"`
	EnableLikeComment        bool `json:"enableLikeComment" yaml:"enableLikeComment"`
	EnableListenIndie        bool `json:"enableListenIndie" yaml:"enableListenIndie"`
	EnableReserve            bool `json:"enableReserve" yaml:"enableReserve"`
	EnableFollowArtist       bool `json:"enableFollowArtist" yaml:"enableFollowArtist"`
	EnableLikeSong           bool `json:"enableLikeSong" yaml:"enableLikeSong"`
	EnableCollectSong        bool `json:"enableCollectSong" yaml:"enableCollectSong"`
	EnablePublishNote        bool `json:"enablePublishNote" yaml:"enablePublishNote"`
	EnablePlayDailyRecommend bool `json:"enablePlayDailyRecommend" yaml:"enablePlayDailyRecommend"`
}

type SignConf struct {
	EnableMain        bool            `json:"enableMain" yaml:"enableMain"`
	EnablePrimary     bool            `json:"enablePrimary" yaml:"enablePrimary"` // 兼容旧版
	EnableSecondaries bool            `json:"enableSecondaries" yaml:"enableSecondaries"`
	YunbeiTask        *YunbeiTaskConf `json:"yunbeiTask" yaml:"yunbeiTask"`
	Automatic         bool            `json:"automatic" yaml:"automatic"`
	EnableVipTask     *bool           `json:"enableVipTask" yaml:"enableVipTask"`
}

type TaskConf struct {
	Sign           bool     `json:"sign" yaml:"sign"`
	PlayIds        bool     `json:"playids" yaml:"playids"`
	MusicianSign   bool     `json:"musician-sign" yaml:"musician-sign"`
	MusicianVip    bool     `json:"musician-vip" yaml:"musician-vip"`
	Note           bool     `json:"note" yaml:"note"`
	FansGroup      bool     `json:"fansgroup" yaml:"fansgroup"`
	DailySongShare bool     `json:"daily-song-share" yaml:"daily-song-share"`
	VipMemberGift  bool     `json:"vip-member-gift" yaml:"vip-member-gift"`
	Mode           string   `json:"mode" yaml:"mode"`
	FastTasks      []string `json:"fast_tasks" yaml:"fast_tasks"`
	SlowTasks      []string `json:"slow_tasks" yaml:"slow_tasks"`
}

// FansGroupConf 乐迷团任务配置
type FansGroupConf struct {
	EnableMain        bool          `json:"enableMain" yaml:"enableMain"`
	EnableSecondaries bool          `json:"enableSecondaries" yaml:"enableSecondaries"`
	GroupIDs          StringOrSlice `json:"groupIds" yaml:"groupIds"`
	AutoDeleteNote    *bool         `json:"autoDeleteNote" yaml:"autoDeleteNote"`
}

type MixPlayConf struct {
	Enabled             bool    `json:"enabled" yaml:"enabled"`
	DailyRecommendRatio float64 `json:"dailyRecommendRatio" yaml:"dailyRecommendRatio"`
	CountTarget         bool    `json:"countTarget" yaml:"countTarget"`
}

// StringOrSlice supports unmarshaling from either a single string or an array of strings.
type StringOrSlice []string

// UnmarshalYAML implements yaml.Unmarshaler
func (s *StringOrSlice) UnmarshalYAML(value *yaml.Node) error {
	var str string
	if err := value.Decode(&str); err == nil {
		*s = []string{str}
		return nil
	}
	var slice []string
	if err := value.Decode(&slice); err == nil {
		*s = slice
		return nil
	}
	return fmt.Errorf("failed to unmarshal StringOrSlice at line %d", value.Line)
}

type PlayIdsConfig struct {
	DailyMin          int64         `json:"daily_min" yaml:"daily_min"`
	DailyMax          int64         `json:"daily_max" yaml:"daily_max"`
	RunMin            int64         `json:"run_min" yaml:"run_min"`
	RunMax            int64         `json:"run_max" yaml:"run_max"`
	GapMin            int64         `json:"gap_min" yaml:"gap_min"`
	GapMax            int64         `json:"gap_max" yaml:"gap_max"`
	IDs               string        `json:"ids" yaml:"ids"`
	IDsFile           StringOrSlice `json:"idsFile" yaml:"idsFile"`
	EnableMain        bool          `json:"enableMain" yaml:"enableMain"`
	EnablePrimary     bool          `json:"enablePrimary" yaml:"enablePrimary"` // 兼容旧版
	EnableSecondaries bool          `json:"enableSecondaries" yaml:"enableSecondaries"`
}

type UpdaterConf struct {
	Check      *bool `json:"check" yaml:"check"`
	AutoUpdate *bool `json:"auto_update" yaml:"auto_update"`
}

type Config struct {
	v              *viper.Viper
	Version        string              `json:"version" yaml:"version"`
	Accounts       *AccountsConf       `json:"accounts" yaml:"accounts"`
	Log            *log.Config         `json:"log" yaml:"log"`
	Network        *api.Config         `json:"network" yaml:"network"`
	Database       *database.Config    `json:"database" yaml:"database"`
	PlayIds        *PlayIdsConfig      `json:"playids" yaml:"playids"`
	Sign           *SignConf           `json:"sign" yaml:"sign"`
	MixPlay        *MixPlayConf        `json:"mixPlay" yaml:"mixPlay"`
	Note           *NoteConf           `json:"note" yaml:"note"`
	Musician       *MusicianConf       `json:"musician" yaml:"musician"`
	FansGroup      *FansGroupConf      `json:"fansgroup" yaml:"fansgroup"`
	DailySongShare *DailySongShareConf `json:"dailySongShare" yaml:"dailySongShare"`
	VipMemberGift  *VipMemberGiftConf  `json:"vipMemberGift" yaml:"vipMemberGift"`
	Task           *TaskConf           `json:"task" yaml:"task"`
	Updater        *UpdaterConf        `json:"updater" yaml:"updater"`
}

// MusicianConf 音乐人任务配置
type MusicianConf struct {
	EnableMain        bool             `json:"enableMain" yaml:"enableMain"`
	EnableSecondaries bool             `json:"enableSecondaries" yaml:"enableSecondaries"`
	IdentityCacheDays *int             `json:"identityCacheDays" yaml:"identityCacheDays"`
	EnableVipNote     *bool            `json:"enableVipNote" yaml:"enableVipNote"`
	EnableVipPlay     *bool            `json:"enableVipPlay" yaml:"enableVipPlay"`
	Play              MusicianPlayConf `json:"play" yaml:"play"`
}

// NoteConf 笔记发布公共配置
type NoteConf struct {
	Titles       []string      `json:"titles" yaml:"titles"`
	TitlesFile   StringOrSlice `json:"titlesFile" yaml:"titlesFile"`
	Messages     []string      `json:"messages" yaml:"messages"`
	MessagesFile StringOrSlice `json:"messagesFile" yaml:"messagesFile"`
	ImageURLs    StringOrSlice `json:"imageUrls" yaml:"imageUrls"`
	Type         int           `json:"type" yaml:"type"`
	AutoDelete   *bool         `json:"autoDelete" yaml:"autoDelete"`
}

// DailySongShareConf 每日推歌发布配置。
type DailySongShareConf struct {
	EnableMain        bool                       `json:"enableMain" yaml:"enableMain"`
	EnableSecondaries bool                       `json:"enableSecondaries" yaml:"enableSecondaries"`
	SongId            string                     `json:"songId" yaml:"songId"`
	PlaylistId        string                     `json:"playlistId" yaml:"playlistId"`
	ImageMode         string                     `json:"imageMode" yaml:"imageMode"`
	ImageURLs         StringOrSlice              `json:"imageUrls" yaml:"imageUrls"`
	TitleMode         string                     `json:"titleMode" yaml:"titleMode"`
	Titles            []string                   `json:"titles" yaml:"titles"`
	TitlesFile        StringOrSlice              `json:"titlesFile" yaml:"titlesFile"`
	Messages          []string                   `json:"messages" yaml:"messages"`
	MessagesFile      StringOrSlice              `json:"messagesFile" yaml:"messagesFile"`
	Topics            []DailySongShareTopicConf  `json:"topics" yaml:"topics"`
	AutoDelete        *bool                      `json:"autoDelete" yaml:"autoDelete"`
	Lottery           *DailySongShareLotteryConf `json:"lottery" yaml:"lottery"`
}

type DailySongShareTopicConf struct {
	Id        string `json:"id" yaml:"id"`
	Name      string `json:"name" yaml:"name"`
	Type      int    `json:"type" yaml:"type"`
	SubType   int    `json:"subType" yaml:"subType"`
	Selected  *bool  `json:"selected" yaml:"selected"`
	CanChange *bool  `json:"canChange" yaml:"canChange"`
}

type DailySongShareLotteryConf struct {
	Enabled      bool   `json:"enabled" yaml:"enabled"`
	ActivityId   string `json:"activityId" yaml:"activityId"`
	AutoRegister *bool  `json:"autoRegister" yaml:"autoRegister"`
}

// VipMemberGiftConf configures the VIP member gift task.
type VipMemberGiftConf struct {
	EnableMain        bool                   `json:"enableMain" yaml:"enableMain"`
	EnableSecondaries bool                   `json:"enableSecondaries" yaml:"enableSecondaries"`
	EnableGift        bool                   `json:"enableGift" yaml:"enableGift"`
	EnableClaim       bool                   `json:"enableClaim" yaml:"enableClaim"`
	Refer             string                 `json:"refer" yaml:"refer"`
	Cloud             VipMemberGiftCloudConf `json:"cloud" yaml:"cloud"`
}

type VipMemberGiftCloudConf struct {
	BaseURL string `json:"baseUrl" yaml:"baseUrl"`
	Token   string `json:"token" yaml:"token"`
}

// MusicianPlayConf 播放任务配置
type MusicianPlayConf struct {
	IDs     string        `json:"ids" yaml:"ids"`
	IDsFile StringOrSlice `json:"idsFile" yaml:"idsFile"`
	RunMin  int64         `json:"run_min" yaml:"run_min"`
	RunMax  int64         `json:"run_max" yaml:"run_max"`
	GapMin  int64         `json:"gap_min" yaml:"gap_min"`
	GapMax  int64         `json:"gap_max" yaml:"gap_max"`
}

func (c *Config) Validate() error {
	if c.Updater == nil {
		c.Updater = &UpdaterConf{}
	}
	if c.Updater.Check == nil {
		chk := true
		c.Updater.Check = &chk
	}
	if c.Updater.AutoUpdate == nil {
		auto := false
		c.Updater.AutoUpdate = &auto
	}

	if c.Accounts != nil {
		if c.Accounts.Main == "" && c.Accounts.Primary != "" {
			c.Accounts.Main = c.Accounts.Primary
		}
		if c.Accounts.AntiCheatTokens == nil {
			c.Accounts.AntiCheatTokens = make(map[string]string)
		}
	}
	if c.PlayIds != nil {
		if !c.PlayIds.EnableMain && c.PlayIds.EnablePrimary {
			c.PlayIds.EnableMain = c.PlayIds.EnablePrimary
		}
	}
	if c.Sign != nil {
		if !c.Sign.EnableMain && c.Sign.EnablePrimary {
			c.Sign.EnableMain = c.Sign.EnablePrimary
		}
		if c.Sign.EnableVipTask == nil {
			enable := true
			c.Sign.EnableVipTask = &enable
		}
		if c.Sign.EnableVipTask == nil {
			enable := true
			c.Sign.EnableVipTask = &enable
		}
		if c.Sign.YunbeiTask == nil {
			c.Sign.YunbeiTask = &YunbeiTaskConf{
				EnableViewVipCenter:      true,
				EnableLikeComment:        true,
				EnableListenIndie:        true,
				EnableReserve:            true,
				EnableFollowArtist:       true,
				EnableLikeSong:           true,
				EnableCollectSong:        true,
				EnablePublishNote:        true,
				EnablePlayDailyRecommend: false,
			}
		}
	}
	if c.Musician != nil {
		if c.Musician.IdentityCacheDays == nil {
			days := 30
			c.Musician.IdentityCacheDays = &days
		}
		if c.Musician.EnableVipNote == nil {
			enable := true
			c.Musician.EnableVipNote = &enable
		}
		if c.Musician.EnableVipPlay == nil {
			enable := true
			c.Musician.EnableVipPlay = &enable
		}
	}
	if c.DailySongShare != nil {
		if c.DailySongShare.PlaylistId == "" {
			c.DailySongShare.PlaylistId = "13848930701"
		}
		if c.DailySongShare.ImageMode == "" {
			c.DailySongShare.ImageMode = "songCover"
		}
		if c.DailySongShare.TitleMode == "" {
			c.DailySongShare.TitleMode = "note"
		}
		if c.DailySongShare.Lottery == nil {
			c.DailySongShare.Lottery = &DailySongShareLotteryConf{}
		}
		if c.DailySongShare.Lottery.ActivityId == "" {
			c.DailySongShare.Lottery.ActivityId = "11066304"
		}
		if c.DailySongShare.Lottery.AutoRegister == nil {
			autoRegister := true
			c.DailySongShare.Lottery.AutoRegister = &autoRegister
		}
		for i := range c.DailySongShare.Topics {
			if c.DailySongShare.Topics[i].Type == 0 {
				if c.DailySongShare.Topics[i].SubType == 11 || strings.Contains(c.DailySongShare.Topics[i].Name, "乐迷团") {
					c.DailySongShare.Topics[i].Type = 3
				} else {
					c.DailySongShare.Topics[i].Type = 2
				}
			}
			if c.DailySongShare.Topics[i].Selected == nil {
				selected := true
				c.DailySongShare.Topics[i].Selected = &selected
			}
			if c.DailySongShare.Topics[i].CanChange == nil {
				canChange := true
				c.DailySongShare.Topics[i].CanChange = &canChange
			}
		}
	}
	if c.VipMemberGift != nil {
		if strings.TrimSpace(c.VipMemberGift.Cloud.BaseURL) == "" {
			c.VipMemberGift.Cloud.BaseURL = decodeB64("aHR0cHM6Ly9uY21tLmhhb3ouY2MuY2Q=")
		}
		if strings.TrimSpace(c.VipMemberGift.Cloud.Token) == "" {
			c.VipMemberGift.Cloud.Token = decodeB64("eXVqaWVyZGFOQjY2Ng==")
		}
	}
	if c.Task != nil {
		if c.Task.Mode == "" {
			c.Task.Mode = "by-task-group"
		}
		if len(c.Task.FastTasks) == 0 {
			c.Task.FastTasks = []string{
				"VipTask", "Reserve", "ViewVipCenter", "LikeComment",
				"FollowArtist", "LikeSong", "CollectSong", "PublishNote",
				"musician-sign", "note", "daily-song-share", "vip-member-gift", "fansgroup",
			}
		}
		if len(c.Task.SlowTasks) == 0 {
			c.Task.SlowTasks = []string{
				"ListenIndie", "PlayDailyRecommend", "playids", "musician-vip",
			}
		}
	}
	return nil
}

func GetDefault() *Config {
	return defaultConfig
}

func New(cfgPath ...string) (*Config, error) {
	var (
		conf Config
		opts = func(m *mapstructure.DecoderConfig) {
			m.TagName = "yaml"
			m.DecodeHook = mapstructure.ComposeDecodeHookFunc(
				mapstructure.StringToTimeDurationHookFunc(),
				mapstructure.StringToSliceHookFunc(","),
				func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
					if t == reflect.TypeOf(StringOrSlice{}) {
						switch v := data.(type) {
						case string:
							return StringOrSlice{v}, nil
						case []interface{}:
							var res StringOrSlice
							for _, item := range v {
								if s, ok := item.(string); ok {
									res = append(res, s)
								} else {
									return nil, fmt.Errorf("invalid element type in StringOrSlice: %T", item)
								}
							}
							return res, nil
						case []string:
							return StringOrSlice(v), nil
						}
					}
					return data, nil
				},
			)
		}
		_cfgPath string
	)
	if len(cfgPath) > 0 {
		_cfgPath = cfgPath[0]
	}

	v := viper.New()
	v.SetTypeByDefaultValue(true)
	v.SetEnvPrefix("ncmm")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.AllowEmptyEnv(true)
	v.SetConfigType("yaml")
	v.SetConfigFile(_cfgPath)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("ReadInConfig: %w", err)
	}
	if err := v.UnmarshalExact(&conf, opts); err != nil {
		return nil, fmt.Errorf("UnmarshalExact: %w", err)
	}
	conf.syncMainCookie()
	if err := conf.Validate(); err != nil {
		return nil, err
	}
	return &conf, nil
}

// ReplaceMagicVariables 替换配置文件中的魔法变量。注意该方法只能调用一次再次调用则不会生效.
func (c *Config) ReplaceMagicVariables(name, value string) (*Config, bool) {

	var (
		isset   bool
		mapping = func(k string) string {
			switch k {
			case name:
				isset = true
				return value
			}
			return ""
		}
	)

	c.Log.Rotate.Filename = os.Expand(c.Log.Rotate.Filename, mapping)
	c.Network.Cookie.Filepath = os.Expand(c.Network.Cookie.Filepath, mapping)
	c.Database.Path = os.Expand(c.Database.Path, mapping)
	if c.Accounts != nil {
		c.Accounts.Main = os.Expand(c.Accounts.Main, mapping)
		c.Accounts.Primary = os.Expand(c.Accounts.Primary, mapping)
		for i, sec := range c.Accounts.Secondary {
			c.Accounts.Secondary[i] = os.Expand(sec, mapping)
		}
	}
	if c.PlayIds != nil {
		for i, file := range c.PlayIds.IDsFile {
			c.PlayIds.IDsFile[i] = os.Expand(file, mapping)
		}
	}
	if c.Musician != nil {
		for i, file := range c.Musician.Play.IDsFile {
			c.Musician.Play.IDsFile[i] = os.Expand(file, mapping)
		}
	}
	if c.Note != nil {
		for i, file := range c.Note.MessagesFile {
			c.Note.MessagesFile[i] = os.Expand(file, mapping)
		}
		for i, file := range c.Note.TitlesFile {
			c.Note.TitlesFile[i] = os.Expand(file, mapping)
		}
		for i, file := range c.Note.ImageURLs {
			c.Note.ImageURLs[i] = os.Expand(file, mapping)
		}
	}
	if c.DailySongShare != nil {
		for i, file := range c.DailySongShare.MessagesFile {
			c.DailySongShare.MessagesFile[i] = os.Expand(file, mapping)
		}
		for i, file := range c.DailySongShare.TitlesFile {
			c.DailySongShare.TitlesFile[i] = os.Expand(file, mapping)
		}
		for i, file := range c.DailySongShare.ImageURLs {
			c.DailySongShare.ImageURLs[i] = os.Expand(file, mapping)
		}
	}
	return c, isset
}

func (c *Config) syncMainCookie() {
	if c.Accounts != nil && c.Accounts.Main != "" {
		if c.Network != nil {
			c.Network.Cookie.Filepath = c.Accounts.Main
		}
	}
}

type verStruct struct {
	Version string `yaml:"version"`
}

// AutoUpgradeConfigIfNeeded 检查配置文件版本，并在版本不符时自动执行迁移和结构升级合并
func AutoUpgradeConfigIfNeeded(cfgPath string) error {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	var v verStruct
	_ = yaml.Unmarshal(data, &v)

	if v.Version != defaultConfig.Version {
		// 先执行原有的字段名重命名迁移
		_ = MigrateConfigFile(cfgPath)
		// 再执行 AST 级的新配置模板字段与注释合并升级
		return AutoUpgradeConfig(cfgPath)
	}
	return nil
}

// AutoUpgradeConfig 读取旧配置文件，将其与嵌入的新配置模板进行 AST 级递归合并，保存最新结构与中文注释的同时保留用户值
func AutoUpgradeConfig(cfgPath string) error {
	userData, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	var userRoot yaml.Node
	if err := yaml.Unmarshal(userData, &userRoot); err != nil {
		return err
	}

	var defaultRoot yaml.Node
	if err := yaml.Unmarshal(defaultConfigByte, &defaultRoot); err != nil {
		return err
	}

	if len(defaultRoot.Content) > 0 && len(userRoot.Content) > 0 {
		mergeYAMLNodes(defaultRoot.Content[0], userRoot.Content[0])
	}

	output, err := yaml.Marshal(&defaultRoot)
	if err != nil {
		return err
	}

	return os.WriteFile(cfgPath, output, 0644)
}

func mergeYAMLNodes(dest, src *yaml.Node) {
	if dest.Kind != src.Kind {
		return
	}
	switch dest.Kind {
	case yaml.DocumentNode:
		if len(dest.Content) > 0 && len(src.Content) > 0 {
			mergeYAMLNodes(dest.Content[0], src.Content[0])
		}
	case yaml.MappingNode:
		for i := 0; i < len(dest.Content); i += 2 {
			destKey := dest.Content[i].Value
			destVal := dest.Content[i+1]

			var srcVal *yaml.Node
			for j := 0; j < len(src.Content); j += 2 {
				if src.Content[j].Value == destKey {
					srcVal = src.Content[j+1]
					break
				}
			}

			if srcVal != nil {
				if destKey == "version" {
					continue
				}
				if destVal.Kind == yaml.MappingNode && srcVal.Kind == yaml.MappingNode {
					mergeYAMLNodes(destVal, srcVal)
				} else if destVal.Kind == yaml.SequenceNode && srcVal.Kind == yaml.SequenceNode {
					destVal.Content = srcVal.Content
				} else {
					destVal.Value = srcVal.Value
					destVal.Tag = srcVal.Tag
					destVal.Kind = srcVal.Kind
					destVal.Content = srcVal.Content
				}
			}
		}
	}
}

// MigrateConfigFile 自动将旧的 primary / enablePrimary 配置字段升级迁移为 main / enableMain
func MigrateConfigFile(cfgPath string) error {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}

	modified := migrateNode(&root)
	if !modified {
		return nil
	}

	output, err := yaml.Marshal(&root)
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, output, 0644)
}

func migrateNode(node *yaml.Node) bool {
	var modified bool
	if node.Kind == yaml.DocumentNode {
		for _, child := range node.Content {
			if migrateNode(child) {
				modified = true
			}
		}
		return modified
	}

	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]

			// Case 1: accounts mapping
			if keyNode.Value == "accounts" && valNode.Kind == yaml.MappingNode {
				for j := 0; j < len(valNode.Content); j += 2 {
					subKey := valNode.Content[j]
					if subKey.Value == "primary" {
						subKey.Value = "main"
						modified = true
					}
				}
			}

			// Case 2: playids mapping
			if keyNode.Value == "playids" && valNode.Kind == yaml.MappingNode {
				for j := 0; j < len(valNode.Content); j += 2 {
					subKey := valNode.Content[j]
					if subKey.Value == "enablePrimary" {
						subKey.Value = "enableMain"
						modified = true
					}
				}
			}

			// Case 3: sign mapping
			if keyNode.Value == "sign" && valNode.Kind == yaml.MappingNode {
				for j := 0; j < len(valNode.Content); j += 2 {
					subKey := valNode.Content[j]
					if subKey.Value == "enablePrimary" {
						subKey.Value = "enableMain"
						modified = true
					}
				}
			}

			// Case 4: Rename musicianVip to musician at top-level
			if keyNode.Value == "musicianVip" {
				keyNode.Value = "musician"
				modified = true
			}

			// Case 5: Rename old task keys
			if keyNode.Value == "task" && valNode.Kind == yaml.MappingNode {
				for j := 0; j < len(valNode.Content); j += 2 {
					subKey := valNode.Content[j]
					switch subKey.Value {
					case "musicianVip", "musician":
						subKey.Value = "musician-sign"
						modified = true
					case "fansGroup":
						subKey.Value = "fansgroup"
						modified = true
					}
				}
			}

			// Case 6: Rename top-level fansGroup to fansgroup
			if keyNode.Value == "fansGroup" {
				keyNode.Value = "fansgroup"
				modified = true
			}



			if migrateNode(valNode) {
				modified = true
			}
		}
	}

	if node.Kind == yaml.SequenceNode {
		for _, child := range node.Content {
			if migrateNode(child) {
				modified = true
			}
		}
	}

	return modified
}



// UpdateAccountsInFile 更新配置文件中的 accounts 并为每个账号添加昵称注释，同时保持原有文件的注释和排版
func UpdateAccountsInFile(cfgPath string, mainPath string, mainNickname string, secondaryPaths []string, secondaryNicknames []string) error {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}

	var docMapping *yaml.Node
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		docMapping = root.Content[0]
	}

	if docMapping == nil || docMapping.Kind != yaml.MappingNode {
		docMapping = &yaml.Node{Kind: yaml.MappingNode}
		root.Kind = yaml.DocumentNode
		root.Content = []*yaml.Node{docMapping}
	}

	// 查找或创建 'accounts' 映射
	var accountsMapping *yaml.Node
	for i := 0; i < len(docMapping.Content); i += 2 {
		if docMapping.Content[i].Value == "accounts" {
			if docMapping.Content[i+1].Kind == yaml.MappingNode {
				accountsMapping = docMapping.Content[i+1]
			}
			break
		}
	}

	if accountsMapping == nil {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "accounts"}
		accountsMapping = &yaml.Node{Kind: yaml.MappingNode}
		docMapping.Content = append(docMapping.Content, keyNode, accountsMapping)
	}

	// 收集现有的注释以便复用
	oldComments := make(map[string]string)
	for i := 0; i < len(accountsMapping.Content); i += 2 {
		key := accountsMapping.Content[i].Value
		valNode := accountsMapping.Content[i+1]
		if key == "main" || key == "primary" {
			if valNode.LineComment != "" {
				oldComments["main"] = valNode.LineComment
			}
		}
		if key == "secondary" && valNode.Kind == yaml.SequenceNode {
			for _, item := range valNode.Content {
				if item.LineComment != "" {
					oldComments[item.Value] = item.LineComment
				}
			}
		}
	}

	// 更新 'main' 字段
	var mainKeyNode, mainValNode *yaml.Node
	for i := 0; i < len(accountsMapping.Content); i += 2 {
		if accountsMapping.Content[i].Value == "main" || accountsMapping.Content[i].Value == "primary" {
			mainKeyNode = accountsMapping.Content[i]
			mainValNode = accountsMapping.Content[i+1]
			mainKeyNode.Value = "main" // 确保是 main
			break
		}
	}

	if mainKeyNode == nil {
		mainKeyNode = &yaml.Node{Kind: yaml.ScalarNode, Value: "main"}
		mainValNode = &yaml.Node{Kind: yaml.ScalarNode}
		accountsMapping.Content = append(accountsMapping.Content, mainKeyNode, mainValNode)
	}

	mainValNode.Kind = yaml.ScalarNode
	mainValNode.Value = mainPath
	if mainNickname != "" {
		mainValNode.LineComment = "# 昵称: " + mainNickname
	} else if oldComm, ok := oldComments["main"]; ok {
		mainValNode.LineComment = oldComm
	} else {
		mainValNode.LineComment = ""
	}

	// 更新 'secondary' 字段
	var secKeyNode, secValNode *yaml.Node
	for i := 0; i < len(accountsMapping.Content); i += 2 {
		if accountsMapping.Content[i].Value == "secondary" {
			secKeyNode = accountsMapping.Content[i]
			secValNode = accountsMapping.Content[i+1]
			break
		}
	}

	if secKeyNode == nil {
		secKeyNode = &yaml.Node{Kind: yaml.ScalarNode, Value: "secondary"}
		secValNode = &yaml.Node{Kind: yaml.SequenceNode}
		accountsMapping.Content = append(accountsMapping.Content, secKeyNode, secValNode)
	}

	secValNode.Kind = yaml.SequenceNode
	secValNode.Content = nil
	for i, path := range secondaryPaths {
		itemNode := &yaml.Node{Kind: yaml.ScalarNode, Value: path}
		var comment string
		if i < len(secondaryNicknames) && secondaryNicknames[i] != "" {
			comment = "# 昵称: " + secondaryNicknames[i]
		} else if oldComm, ok := oldComments[path]; ok {
			comment = oldComm
		}

		if comment != "" {
			trimmed := strings.TrimSpace(comment)
			if !strings.HasPrefix(trimmed, "#") {
				itemNode.LineComment = "# " + trimmed
			} else {
				itemNode.LineComment = comment
			}
		}
		secValNode.Content = append(secValNode.Content, itemNode)
	}

	output, err := yaml.Marshal(&root)
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, output, 0644)
}
