package webui

import (
	"strings"

	"github.com/3899/ncmm/config"
	"gopkg.in/yaml.v3"
)

const configSchemaVersion = 1

type configSchemaView struct {
	SchemaVersion int                           `json:"schemaVersion"`
	Targets       map[string]configTargetSchema `json:"targets"`
}

type configTargetSchema struct {
	ConfigVersion string                 `json:"configVersion,omitempty"`
	Categories    []configSchemaCategory `json:"categories"`
	Fields        []configSchemaField    `json:"fields"`
}

type configSchemaCategory struct {
	ID          string              `json:"id"`
	Title       string              `json:"title"`
	Icon        string              `json:"icon"`
	Description string              `json:"description"`
	Groups      []configSchemaGroup `json:"groups"`
}

type configSchemaGroup struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Badge string `json:"badge,omitempty"`
	Tone  string `json:"tone,omitempty"`
}

type configSchemaField struct {
	Path        string               `json:"path"`
	Title       string               `json:"title"`
	Description string               `json:"description,omitempty"`
	Group       string               `json:"group"`
	Type        string               `json:"type"`
	Widget      string               `json:"widget"`
	Unit        string               `json:"unit,omitempty"`
	Placeholder string               `json:"placeholder,omitempty"`
	Default     any                  `json:"default,omitempty"`
	Min         *float64             `json:"min,omitempty"`
	Max         *float64             `json:"max,omitempty"`
	Step        *float64             `json:"step,omitempty"`
	Options     []configSchemaOption `json:"options,omitempty"`
	Presets     []any                `json:"presets,omitempty"`
	Sensitive   bool                 `json:"sensitive,omitempty"`
	ReadOnly    bool                 `json:"readOnly,omitempty"`
	Advanced    bool                 `json:"advanced,omitempty"`
}

type configSchemaOption struct {
	Value any    `json:"value"`
	Label string `json:"label"`
}

func number(value float64) *float64 { return &value }

func schemaGroup(id, title, badge, tone string) configSchemaGroup {
	return configSchemaGroup{ID: id, Title: title, Badge: badge, Tone: tone}
}

func schemaCategory(id, title, icon, description string, groups ...configSchemaGroup) configSchemaCategory {
	return configSchemaCategory{ID: id, Title: title, Icon: icon, Description: description, Groups: groups}
}

func schemaField(path, title, description, group, kind, widget string) configSchemaField {
	return configSchemaField{Path: path, Title: title, Description: description, Group: group, Type: kind, Widget: widget}
}

func schemaOption(value any, label string) configSchemaOption {
	return configSchemaOption{Value: value, Label: label}
}

func configurationSchema() configSchemaView {
	main := mainConfigSchema()
	notify := notifyConfigSchema()
	var mainDefaults any
	if yaml.Unmarshal(config.DefaultYAML(), &mainDefaults) == nil {
		applySchemaDefaults(&main, mainDefaults)
		if root, ok := mainDefaults.(map[string]any); ok {
			main.ConfigVersion, _ = root["version"].(string)
		}
	}
	var notifyDefaults any
	if data, err := yaml.Marshal(defaultNotifyConfig()); err == nil && yaml.Unmarshal(data, &notifyDefaults) == nil {
		applySchemaDefaults(&notify, notifyDefaults)
	}
	return configSchemaView{
		SchemaVersion: configSchemaVersion,
		Targets: map[string]configTargetSchema{
			"config": main,
			"notify": notify,
		},
	}
}

func applySchemaDefaults(target *configTargetSchema, defaults any) {
	for index := range target.Fields {
		if value, ok := schemaValueAt(defaults, target.Fields[index].Path); ok {
			target.Fields[index].Default = value
		}
	}
}

func schemaValueAt(root any, path string) (any, bool) {
	current := root
	for _, key := range strings.Split(path, ".") {
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = mapping[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func mainConfigSchema() configTargetSchema {
	categories := []configSchemaCategory{
		schemaCategory("task", "批量任务", "", "定义 task 命令包含的业务以及跨账号执行顺序。",
			schemaGroup("tasks", "任务开关", "按需启用", "primary"),
			schemaGroup("queue", "执行顺序", "高级", "gray")),
		schemaCategory("accounts", "账号与凭据", "", "主辅账号 Cookie 路径以及移动端防风控凭证。",
			schemaGroup("cookie", "Cookie 路径配置", "基础凭据", "cyan"),
			schemaGroup("tokens", "移动端 X-antiCheatToken", "推歌 / 领会员", "amber")),
		schemaCategory("network", "网络与代理", "", "请求超时、重试和多端协议 User-Agent。",
			schemaGroup("network-main", "网络通信参数", "全局", "cyan"),
			schemaGroup("user-agent", "多端 User-Agent", "协议", "gray")),
		schemaCategory("sign", "日常签到", "", "云贝签到、VIP 任务以及十项云贝子任务。",
			schemaGroup("sign-main", "签到控制", "每日", "success"),
			schemaGroup("yunbei", "云贝精细化子任务", "10 项", "cyan")),
		schemaCategory("playids", "指定播放", "", "配置有效播放量目标、播放间隔和歌曲来源。",
			schemaGroup("play-target", "账号与播放目标", "核心", "primary"),
			schemaGroup("play-source", "间隔与歌曲源", "播放池", "cyan")),
		schemaCategory("mixPlay", "日推混听", "", "使用日推歌曲穿插降低连续播放行为的单一性。",
			schemaGroup("mix", "混听策略", "防风控", "success")),
		schemaCategory("musician", "音乐人与 VIP", "", "音乐人签到、VIP 进阶任务和专属播放覆盖参数。",
			schemaGroup("musician-main", "音乐人主控参数", "身份任务", "cyan"),
			schemaGroup("musician-play", "进阶专属播放参数", "可继承", "gray")),
		schemaCategory("note", "动态笔记", "", "公共笔记标题、正文、图片池和发布方式。",
			schemaGroup("note-mode", "发布方式", "通用", "primary"),
			schemaGroup("note-content", "随机素材库", "内容", "cyan")),
		schemaCategory("dailySongShare", "每日推歌", "", "每日分享歌曲、配图、文案、话题和抽奖配置。",
			schemaGroup("share-main", "推歌发布控制", "每日", "primary"),
			schemaGroup("share-content", "内容与素材", "可继承", "cyan"),
			schemaGroup("share-community", "抽奖与社区话题", "社区", "amber")),
		schemaCategory("fansgroup", "乐迷团打卡", "", "音乐合伙人乐迷团打卡和临时笔记清理策略。",
			schemaGroup("fans", "乐迷团任务", "每日", "success")),
		schemaCategory("vipMemberGift", "会员礼品卡", "", "黑胶会员礼品的发布、领取与云端互助服务。",
			schemaGroup("gift-main", "会员礼品任务", "黑胶会员", "primary"),
			schemaGroup("gift-cloud", "云端服务", "高级", "gray")),
		schemaCategory("notify", "失败通知策略", "", "控制何时发送失败汇总；通道凭证位于推送配置。",
			schemaGroup("notify", "通知策略", "汇总推送", "amber")),
		schemaCategory("updater", "自动更新", "", "版本检查、自动替换和 GitHub 代理镜像。",
			schemaGroup("updater", "更新策略", "版本", "success")),
		schemaCategory("log", "日志与轮转", "", "应用日志格式、级别、输出和文件轮转。",
			schemaGroup("log-main", "日志输出", "运行日志", "cyan"),
			schemaGroup("log-rotate", "文件轮转", "存储", "gray")),
		schemaCategory("database", "本地数据库", "", "任务进度和播放状态的本地缓存位置。",
			schemaGroup("database", "数据库存储", "本地", "cyan")),
	}

	fields := []configSchemaField{
		schemaField("accounts.main", "主账号 Cookie 路径", "音乐人或 VIP 主力账号使用的 Cookie JSON 文件。", "cookie", "string", "text"),
		schemaField("accounts.secondary", "辅助账号列表", "支持多个辅助账号接力执行任务。", "cookie", "string-list", "tags"),
		func() configSchemaField {
			f := schemaField("accounts.antiCheatTokens", "账号 Token 映射", "按 Cookie 文件分别保存移动端抓包得到的 X-antiCheatToken。", "tokens", "string-map", "map")
			f.Sensitive = true
			return f
		}(),

		schemaField("network.debug", "HTTP 调试日志", "仅在排查接口故障时开启。", "network-main", "boolean", "switch"),
		func() configSchemaField {
			f := schemaField("network.timeout", "全局请求超时", "Go duration 格式，例如 15s、1m。", "network-main", "duration", "duration")
			f.Presets = []any{"15s", "30s", "60s"}
			return f
		}(),
		func() configSchemaField {
			f := schemaField("network.retry", "网络失败重试次数", "网络抖动时的自动重试次数。", "network-main", "integer", "stepper")
			f.Min = number(0)
			f.Max = number(10)
			return f
		}(),
		schemaField("network.user_agent.default", "默认 User-Agent", "其他协议 UA 留空时使用的兜底值。", "user-agent", "string", "textarea"),
		schemaField("network.user_agent.weapi", "Web / PC User-Agent", "WEAPI 协议请求使用。", "user-agent", "string", "textarea"),
		schemaField("network.user_agent.eapi", "iOS User-Agent", "iPhone/iPad EAPI 请求使用。", "user-agent", "string", "textarea"),
		schemaField("network.user_agent.xeapi", "Android User-Agent", "Android XEAPI/AEAPI 请求使用。", "user-agent", "string", "textarea"),

		schemaField("task.sign", "日常一键签到", "在 task 批量执行中包含 sign。", "tasks", "boolean", "switch"),
		schemaField("task.playids", "播放指定歌曲", "在 task 批量执行中包含 playids。", "tasks", "boolean", "switch"),
		schemaField("task.musician-sign", "音乐人日常签到", "执行音乐人每日签到和云豆领取。", "tasks", "boolean", "switch"),
		schemaField("task.musician-vip", "音乐人 VIP 进阶", "执行每月领取 VIP 的进阶任务，可能耗时较长。", "tasks", "boolean", "switch"),
		schemaField("task.note", "自动发布笔记", "执行公共图文笔记发布任务。", "tasks", "boolean", "switch"),
		schemaField("task.daily-song-share", "每日推歌与抽奖", "需要匹配的移动端 Cookie、UA 和 Token。", "tasks", "boolean", "switch"),
		schemaField("task.vip-member-gift", "黑胶会员赠送与领取", "领取任务需要对应账号的 antiCheatToken。", "tasks", "boolean", "switch"),
		schemaField("task.fansgroup", "乐迷团日常任务", "执行配置的乐迷团打卡。", "tasks", "boolean", "switch"),
		func() configSchemaField {
			f := schemaField("task.mode", "任务执行模式", "跨账号分组串行（全部账号先快后慢）或单账号完整串行（单账号依次跑完全部任务）。", "queue", "string", "segmented")
			f.Options = []configSchemaOption{schemaOption("by-task-group", "跨账号分组串行"), schemaOption("by-account", "单账号完整串行")}
			return f
		}(),
		func() configSchemaField {
			f := schemaField("task.fast_tasks", "快任务列表", "秒级任务的内部执行顺序；通常无需修改。", "queue", "string-list", "tags")
			f.Advanced = true
			return f
		}(),
		func() configSchemaField {
			f := schemaField("task.slow_tasks", "慢任务列表", "听歌类长任务的内部执行顺序；通常无需修改。", "queue", "string-list", "tags")
			f.Advanced = true
			return f
		}(),

		schemaField("sign.automatic", "自动领取任务奖励", "执行签到时自动完成并领取可用奖励。", "sign-main", "boolean", "switch"),
		schemaField("sign.enableMain", "主账号签到", "主账号执行云贝和 VIP 签到。", "sign-main", "boolean", "switch"),
		schemaField("sign.enableSecondaries", "辅助账号签到", "全部辅助账号依次执行签到。", "sign-main", "boolean", "switch"),
		schemaField("sign.enableVipTask", "黑胶 VIP 专属任务", "执行会员成长值相关任务。", "sign-main", "boolean", "switch"),
		schemaField("sign.yunbeiTask.enableViewVipCenter", "浏览会员中心", "云贝中心浏览任务。", "yunbei", "boolean", "switch"),
		schemaField("sign.yunbeiTask.enableLikeComment", "点赞评论和动态", "完成互动类云贝任务。", "yunbei", "boolean", "switch"),
		schemaField("sign.yunbeiTask.enableListenIndie", "探索小众歌曲", "听歌类慢任务，预计耗时较长。", "yunbei", "boolean", "switch"),
		schemaField("sign.yunbeiTask.enableReserve", "预约领云贝", "完成预约类云贝任务。", "yunbei", "boolean", "switch"),
		schemaField("sign.yunbeiTask.enableFollowArtist", "关注歌手", "完成关注类云贝任务。", "yunbei", "boolean", "switch"),
		schemaField("sign.yunbeiTask.enableLikeSong", "红心歌曲", "完成收藏喜爱歌曲任务。", "yunbei", "boolean", "switch"),
		schemaField("sign.yunbeiTask.enableCollectSong", "收藏歌曲", "完成歌曲收藏任务。", "yunbei", "boolean", "switch"),
		schemaField("sign.yunbeiTask.enablePublishNote", "发布图文动态", "使用笔记配置完成发布任务。", "yunbei", "boolean", "switch"),
		schemaField("sign.yunbeiTask.enableShareSong", "分享歌曲", "完成歌曲分享任务。", "yunbei", "boolean", "switch"),
		schemaField("sign.yunbeiTask.enablePlayDailyRecommend", "播放日推歌曲", "听歌类慢任务，约 31 至 45 分钟。", "yunbei", "boolean", "switch"),

		schemaField("playids.enableMain", "主账号参与播放", "允许主账号执行指定歌曲播放。", "play-target", "boolean", "switch"),
		schemaField("playids.enableSecondaries", "辅助账号参与播放", "允许全部辅助账号执行指定歌曲播放。", "play-target", "boolean", "switch"),
		func() configSchemaField {
			f := schemaField("playids.daily_min", "每日目标下限", "每天首次执行时随机生成当日目标。", "play-target", "integer", "number")
			f.Unit = "首"
			f.Min = number(0)
			return f
		}(),
		func() configSchemaField {
			f := schemaField("playids.daily_max", "每日目标上限", "必须不小于每日目标下限。", "play-target", "integer", "number")
			f.Unit = "首"
			f.Min = number(0)
			return f
		}(),
		func() configSchemaField {
			f := schemaField("playids.run_min", "单次目标下限", "0 表示不限制最低单次目标。", "play-target", "integer", "number")
			f.Unit = "首"
			f.Min = number(0)
			return f
		}(),
		func() configSchemaField {
			f := schemaField("playids.run_max", "单次目标上限", "限制每次运行处理的歌曲数量。", "play-target", "integer", "number")
			f.Unit = "首"
			f.Min = number(0)
			return f
		}(),
		func() configSchemaField {
			f := schemaField("playids.gap_min", "播放间隔下限", "两首歌曲之间的最小等待时间。", "play-source", "integer", "number")
			f.Unit = "秒"
			f.Min = number(0)
			return f
		}(),
		func() configSchemaField {
			f := schemaField("playids.gap_max", "播放间隔上限", "两首歌曲之间的最大等待时间。", "play-source", "integer", "number")
			f.Unit = "秒"
			f.Min = number(0)
			return f
		}(),
		schemaField("playids.ids", "歌曲 ID 池", "使用逗号分隔保存，界面按标签编辑。", "play-source", "csv", "tags"),
		schemaField("playids.idsFile", "歌曲 ID 文件", "支持本地路径和 HTTP/HTTPS 文本列表。", "play-source", "string-list", "tags"),

		schemaField("mixPlay.enabled", "启用日推混听", "在指定播放中穿插官方日推歌曲。", "mix", "boolean", "switch"),
		func() configSchemaField {
			f := schemaField("mixPlay.dailyRecommendRatio", "日推穿插比例", "0.3 表示约 30% 的播放来自日推。", "mix", "number", "slider")
			f.Min = number(0)
			f.Max = number(1)
			f.Step = number(0.05)
			f.Unit = "%"
			f.Presets = []any{0.15, 0.3, 0.5}
			return f
		}(),
		schemaField("mixPlay.countTarget", "混听计入播放目标", "开启后日推歌曲也计入每日和单次目标。", "mix", "boolean", "switch"),

		func() configSchemaField {
			f := schemaField("note.type", "动态类型", "选择图文笔记或普通动态。", "note-mode", "integer", "segmented")
			f.Options = []configSchemaOption{schemaOption(39, "图文笔记"), schemaOption(35, "普通动态")}
			return f
		}(),
		schemaField("note.autoDelete", "发布后自动删除", "发布成功后删除动态，保持主页整洁。", "note-mode", "boolean", "switch"),
		schemaField("note.titles", "随机标题池", "每行一个标题。", "note-content", "string-list", "textarea-list"),
		schemaField("note.titlesFile", "标题文件来源", "支持本地或远程文本文件。", "note-content", "string-list", "tags"),
		schemaField("note.messages", "随机正文池", "每行一条正文。", "note-content", "string-list", "textarea-list"),
		schemaField("note.messagesFile", "正文文件来源", "支持本地或远程文本文件。", "note-content", "string-list", "tags"),
		schemaField("note.imageUrls", "配图来源", "支持本地图片、图片直链或文本列表。", "note-content", "string-list", "tags"),

		schemaField("musician.enableMain", "主账号执行音乐人任务", "主账号参与音乐人签到和进阶任务。", "musician-main", "boolean", "switch"),
		schemaField("musician.enableSecondaries", "辅助账号执行音乐人任务", "辅助账号具有音乐人身份时才建议开启。", "musician-main", "boolean", "switch"),
		func() configSchemaField {
			f := schemaField("musician.identityCacheDays", "身份缓存时间", "0 为永久有效，-1 为关闭缓存。", "musician-main", "integer", "stepper")
			f.Unit = "天"
			f.Min = number(-1)
			f.Max = number(365)
			return f
		}(),
		schemaField("musician.enableVipNote", "VIP 进阶自动发笔记", "允许进阶任务复用笔记配置。", "musician-main", "boolean", "switch"),
		schemaField("musician.enableVipPlay", "VIP 进阶接力播放", "允许进阶任务执行专属播放目标。", "musician-main", "boolean", "switch"),
		schemaField("musician.play.ids", "专属歌曲 ID", "留空继承指定播放模块的歌曲池。", "musician-play", "csv", "tags"),
		schemaField("musician.play.idsFile", "专属歌曲文件", "留空继承指定播放模块的文件列表。", "musician-play", "string-list", "tags"),
		func() configSchemaField {
			f := schemaField("musician.play.daily_min", "每日目标下限", "0 表示继承指定播放模块。", "musician-play", "integer", "number")
			f.Unit = "首"
			f.Min = number(0)
			return f
		}(),
		func() configSchemaField {
			f := schemaField("musician.play.daily_max", "每日目标上限", "0 表示继承指定播放模块。", "musician-play", "integer", "number")
			f.Unit = "首"
			f.Min = number(0)
			return f
		}(),
		func() configSchemaField {
			f := schemaField("musician.play.run_min", "单次目标下限", "0 表示继承指定播放模块。", "musician-play", "integer", "number")
			f.Unit = "首"
			f.Min = number(0)
			return f
		}(),
		func() configSchemaField {
			f := schemaField("musician.play.run_max", "单次目标上限", "0 表示继承指定播放模块。", "musician-play", "integer", "number")
			f.Unit = "首"
			f.Min = number(0)
			return f
		}(),
		func() configSchemaField {
			f := schemaField("musician.play.gap_min", "播放间隔下限", "0 表示继承指定播放模块。", "musician-play", "integer", "number")
			f.Unit = "秒"
			f.Min = number(0)
			return f
		}(),
		func() configSchemaField {
			f := schemaField("musician.play.gap_max", "播放间隔上限", "0 表示继承指定播放模块。", "musician-play", "integer", "number")
			f.Unit = "秒"
			f.Min = number(0)
			return f
		}(),

		schemaField("dailySongShare.enableMain", "主账号每日推歌", "主账号执行每日分享。", "share-main", "boolean", "switch"),
		schemaField("dailySongShare.enableSecondaries", "辅助账号每日推歌", "辅助账号执行每日分享。", "share-main", "boolean", "switch"),
		schemaField("dailySongShare.songId", "指定歌曲 ID", "留空时从歌单随机选择。", "share-main", "string", "text"),
		schemaField("dailySongShare.playlistId", "选歌歌单 ID", "未指定歌曲时使用的歌单。", "share-main", "string", "text"),
		func() configSchemaField {
			f := schemaField("dailySongShare.imageMode", "配图模式", "指定发布动态使用的图片来源。", "share-content", "string", "segmented")
			f.Options = []configSchemaOption{schemaOption("songCover", "歌曲封面"), schemaOption("playlistCover", "歌单封面"), schemaOption("custom", "自定义")}
			return f
		}(),
		func() configSchemaField {
			f := schemaField("dailySongShare.titleMode", "标题模式", "使用公共笔记标题或歌曲名称。", "share-content", "string", "segmented")
			f.Options = []configSchemaOption{schemaOption("note", "公共笔记"), schemaOption("song", "歌曲名称")}
			return f
		}(),
		schemaField("dailySongShare.imageUrls", "自定义配图", "仅自定义图片模式使用。", "share-content", "string-list", "tags"),
		schemaField("dailySongShare.titles", "专属标题池", "留空继承公共笔记配置。", "share-content", "string-list", "textarea-list"),
		schemaField("dailySongShare.titlesFile", "专属标题文件", "留空继承公共笔记配置。", "share-content", "string-list", "tags"),
		schemaField("dailySongShare.messages", "专属正文池", "留空继承公共笔记配置。", "share-content", "string-list", "textarea-list"),
		schemaField("dailySongShare.messagesFile", "专属正文文件", "留空继承公共笔记配置。", "share-content", "string-list", "tags"),
		schemaField("dailySongShare.autoDelete", "发布后自动删除", "分享完成后删除动态。", "share-main", "boolean", "switch"),
		schemaField("dailySongShare.lottery.enabled", "自动参与抽奖", "发布后参与配置的活动。", "share-community", "boolean", "switch"),
		schemaField("dailySongShare.lottery.activityId", "抽奖活动 ID", "留空使用程序内置活动。", "share-community", "string", "text"),
		schemaField("dailySongShare.lottery.autoRegister", "自动报名活动", "需要时自动完成活动报名。", "share-community", "boolean", "switch"),
		schemaField("dailySongShare.topics", "社区话题", "对象数组包含名称、ID、类型和子类型。", "share-community", "object-list", "json"),

		schemaField("fansgroup.enableMain", "主账号执行乐迷团", "主账号执行配置的乐迷团打卡。", "fans", "boolean", "switch"),
		schemaField("fansgroup.enableSecondaries", "辅助账号执行乐迷团", "辅助账号依次执行乐迷团打卡。", "fans", "boolean", "switch"),
		schemaField("fansgroup.groupIds", "乐迷团群 ID", "支持配置多个乐迷团群。", "fans", "string-list", "tags"),
		schemaField("fansgroup.autoDeleteNote", "自动删除临时笔记", "留空时继承公共笔记的删除设置。", "fans", "boolean", "switch"),

		schemaField("vipMemberGift.enableMain", "主账号参与会员礼品", "允许主账号发布或领取礼品。", "gift-main", "boolean", "switch"),
		schemaField("vipMemberGift.enableSecondaries", "辅助账号参与会员礼品", "允许辅助账号发布或领取礼品。", "gift-main", "boolean", "switch"),
		schemaField("vipMemberGift.enableGift", "自动发布赠礼", "将可赠送会员发布到互助服务。", "gift-main", "boolean", "switch"),
		schemaField("vipMemberGift.enableClaim", "自动领取会员", "需要移动端 Token 才能领取。", "gift-main", "boolean", "switch"),
		func() configSchemaField {
			f := schemaField("vipMemberGift.refer", "请求来源标识", "高级接口参数，通常无需设置。", "gift-cloud", "string", "text")
			f.Advanced = true
			return f
		}(),
		schemaField("vipMemberGift.cloud.baseUrl", "云端服务地址", "留空使用程序内置互助服务。", "gift-cloud", "string", "text"),
		func() configSchemaField {
			f := schemaField("vipMemberGift.cloud.token", "云端服务 Token", "留空使用程序内置凭据。", "gift-cloud", "string", "password")
			f.Sensitive = true
			return f
		}(),

		schemaField("notify.enabled", "启用失败汇总通知", "还需要在推送配置中启用至少一个通道。", "notify", "boolean", "switch"),
		schemaField("notify.on_skip", "任务跳过时通知", "缺少凭据等导致跳过时也纳入汇总。", "notify", "boolean", "switch"),
		schemaField("notify.title_prefix", "通知标题前缀", "用于区分多台设备或多个实例。", "notify", "string", "text"),
		func() configSchemaField {
			f := schemaField("notify.timeout", "单通道请求超时", "Go duration 格式，例如 10s。", "notify", "duration", "duration")
			f.Presets = []any{"5s", "10s", "30s"}
			return f
		}(),
		schemaField("notify.file", "通知配置文件", "相对路径以主配置目录为基准。", "notify", "string", "text"),

		schemaField("updater.check", "检测新版本", "每天最多请求一次版本接口。", "updater", "boolean", "switch"),
		schemaField("updater.auto_update", "自动替换二进制", "容器环境会忽略此选项。", "updater", "boolean", "switch"),
		schemaField("updater.proxy_mirrors", "GitHub 代理镜像", "按顺序尝试，每行一个地址。", "updater", "string-list", "textarea-list"),

		schemaField("log.app", "应用标识", "写入每条结构化日志的应用名称。", "log-main", "string", "text"),
		func() configSchemaField {
			f := schemaField("log.format", "日志格式", "选择便于阅读的文本或结构化 JSON。", "log-main", "string", "segmented")
			f.Options = []configSchemaOption{schemaOption("text", "TEXT"), schemaOption("json", "JSON")}
			return f
		}(),
		func() configSchemaField {
			f := schemaField("log.level", "日志级别", "级别越低输出越详细。", "log-main", "string", "segmented")
			f.Options = []configSchemaOption{schemaOption("debug", "DEBUG"), schemaOption("info", "INFO"), schemaOption("warn", "WARN"), schemaOption("error", "ERROR")}
			return f
		}(),
		schemaField("log.stdout", "输出到控制台", "同时将日志写入标准错误输出。", "log-main", "boolean", "switch"),
		schemaField("log.rotate.filename", "日志文件路径", "支持相对于程序 home 的路径。", "log-rotate", "string", "text"),
		func() configSchemaField {
			f := schemaField("log.rotate.maxsize", "单文件大小上限", "达到上限后自动轮转。", "log-rotate", "integer", "number")
			f.Unit = "MB"
			f.Min = number(1)
			return f
		}(),
		func() configSchemaField {
			f := schemaField("log.rotate.maxage", "日志保留天数", "超过天数的轮转文件会被清理。", "log-rotate", "integer", "number")
			f.Unit = "天"
			f.Min = number(0)
			return f
		}(),
		func() configSchemaField {
			f := schemaField("log.rotate.maxbackups", "最大备份数量", "限制保留的轮转文件数量。", "log-rotate", "integer", "stepper")
			f.Min = number(0)
			return f
		}(),
		schemaField("log.rotate.localtime", "使用本地时间", "轮转文件名使用本地时区。", "log-rotate", "boolean", "switch"),
		schemaField("log.rotate.compress", "压缩备份日志", "使用 gzip 压缩历史日志。", "log-rotate", "boolean", "switch"),

		func() configSchemaField {
			f := schemaField("database.driver", "数据库驱动", "当前仅支持 Badger。", "database", "string", "text")
			f.ReadOnly = true
			return f
		}(),
		schemaField("database.path", "数据存储目录", "保存任务进度和播放状态。", "database", "string", "text"),
	}
	return configTargetSchema{Categories: categories, Fields: fields}
}

func notifyConfigSchema() configTargetSchema {
	categories := []configSchemaCategory{
		schemaCategory("webhook", "自定义 Webhook", "", "按 JSON 请求推送到自定义服务。", schemaGroup("channel", "Webhook 配置", "通用", "cyan")),
		schemaCategory("bark", "Bark", "", "向 Bark 设备发送通知。", schemaGroup("channel", "Bark 配置", "iOS", "success")),
		schemaCategory("serverchan", "Server 酱", "", "通过 Server 酱发送通知。", schemaGroup("channel", "Server 酱配置", "微信", "success")),
		schemaCategory("telegram", "Telegram", "", "通过 Telegram Bot 发送通知。", schemaGroup("channel", "Telegram Bot", "Bot", "cyan")),
		schemaCategory("dingtalk", "钉钉机器人", "", "通过钉钉群机器人发送通知。", schemaGroup("channel", "钉钉机器人", "群通知", "amber")),
		schemaCategory("coolpush", "CoolPush", "", "通过 CoolPush / Qmsg 发送 QQ 消息。", schemaGroup("channel", "CoolPush 配置", "QQ", "cyan")),
		schemaCategory("pushplus", "PushPlus", "", "通过 PushPlus 发送通知。", schemaGroup("channel", "PushPlus 配置", "微信", "success")),
		schemaCategory("wecom_key", "企业微信群", "", "通过企业微信群机器人发送通知。", schemaGroup("channel", "群机器人配置", "企业微信", "success")),
		schemaCategory("wecom_app", "企业微信应用", "", "通过企业微信自建应用发送消息。", schemaGroup("channel", "应用消息配置", "企业微信", "success")),
	}
	fields := []configSchemaField{
		schemaField("webhook.enabled", "启用通道", "启用自定义 Webhook。", "channel", "boolean", "switch"),
		schemaField("webhook.url", "Webhook 地址", "接收通知的 HTTP 地址。", "channel", "string", "text"),
		func() configSchemaField {
			f := schemaField("webhook.method", "请求方法", "发送 Webhook 使用的 HTTP 方法。", "channel", "string", "segmented")
			f.Options = []configSchemaOption{schemaOption("POST", "POST"), schemaOption("PUT", "PUT")}
			return f
		}(),
		schemaField("webhook.headers", "请求头", "键值映射，将随 Webhook 请求发送。", "channel", "string-map", "map"),
		schemaField("webhook.body_template", "请求体模板", "留空使用 NCMM 默认 JSON 格式。", "channel", "string", "textarea"),
		schemaField("bark.enabled", "启用通道", "启用 Bark 通知。", "channel", "boolean", "switch"),
		func() configSchemaField {
			f := schemaField("bark.key", "设备 Key", "Bark App 提供的设备 Key。", "channel", "string", "password")
			f.Sensitive = true
			return f
		}(),
		schemaField("bark.server", "服务地址", "留空使用 Bark 官方服务。", "channel", "string", "text"),
		schemaField("serverchan.enabled", "启用通道", "启用 Server 酱通知。", "channel", "boolean", "switch"),
		func() configSchemaField {
			f := schemaField("serverchan.sckey", "SendKey", "Server 酱提供的发送 Key。", "channel", "string", "password")
			f.Sensitive = true
			return f
		}(),
		schemaField("telegram.enabled", "启用通道", "启用 Telegram Bot 通知。", "channel", "boolean", "switch"),
		func() configSchemaField {
			f := schemaField("telegram.bot_token", "Bot Token", "BotFather 提供的 Token。", "channel", "string", "password")
			f.Sensitive = true
			return f
		}(),
		schemaField("telegram.user_id", "用户或群组 ID", "接收消息的用户、群组或频道 ID。", "channel", "string", "text"),
		schemaField("telegram.api_host", "API 地址", "可选 Telegram API 反向代理。", "channel", "string", "text"),
		schemaField("telegram.proxy", "HTTP 代理", "访问 Telegram 使用的代理地址。", "channel", "string", "text"),
		schemaField("dingtalk.enabled", "启用通道", "启用钉钉群机器人。", "channel", "boolean", "switch"),
		func() configSchemaField {
			f := schemaField("dingtalk.access_token", "Access Token", "钉钉机器人访问令牌。", "channel", "string", "password")
			f.Sensitive = true
			return f
		}(),
		func() configSchemaField {
			f := schemaField("dingtalk.secret", "加签 Secret", "启用加签时填写。", "channel", "string", "password")
			f.Sensitive = true
			return f
		}(),
		schemaField("coolpush.enabled", "启用通道", "启用 CoolPush / Qmsg。", "channel", "boolean", "switch"),
		func() configSchemaField {
			f := schemaField("coolpush.skey", "SKEY", "CoolPush 服务密钥。", "channel", "string", "password")
			f.Sensitive = true
			return f
		}(),
		schemaField("coolpush.mode", "发送模式", "例如 send 或 group。", "channel", "string", "text"),
		schemaField("pushplus.enabled", "启用通道", "启用 PushPlus。", "channel", "boolean", "switch"),
		func() configSchemaField {
			f := schemaField("pushplus.token", "Token", "PushPlus 发送 Token。", "channel", "string", "password")
			f.Sensitive = true
			return f
		}(),
		schemaField("wecom_key.enabled", "启用通道", "启用企业微信群机器人。", "channel", "boolean", "switch"),
		func() configSchemaField {
			f := schemaField("wecom_key.key", "机器人 Key", "企业微信群机器人 Webhook Key。", "channel", "string", "password")
			f.Sensitive = true
			return f
		}(),
		schemaField("wecom_app.enabled", "启用通道", "启用企业微信应用消息。", "channel", "boolean", "switch"),
		schemaField("wecom_app.corp_id", "企业 ID", "企业微信 Corp ID。", "channel", "string", "text"),
		func() configSchemaField {
			f := schemaField("wecom_app.corp_secret", "应用 Secret", "企业微信应用 Secret。", "channel", "string", "password")
			f.Sensitive = true
			return f
		}(),
		schemaField("wecom_app.to_user", "接收用户", "默认 @all，可填写成员账号。", "channel", "string", "text"),
		schemaField("wecom_app.agent_id", "应用 ID", "企业微信应用 Agent ID。", "channel", "string", "text"),
		schemaField("wecom_app.media_id", "媒体 ID", "填写后发送图文消息。", "channel", "string", "text"),
	}
	return configTargetSchema{Categories: categories, Fields: fields}
}
