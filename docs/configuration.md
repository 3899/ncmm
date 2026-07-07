# ⚙️ 配置文件说明 (`config.yaml`)

默认配置文件路径为 `~/config.yaml`（支持在运行时通过 `-c` 或 `--config` 指定）。配置字段说明如下：

```yaml
# 配置文件版本
version: 1.0

# 顶级多账号管理
accounts:
  # 音乐人主账号 Cookie 文件路径
  main: "${HOME}/.ncmm/cookie.json"
  # 辅助刷量账号 Cookie 列表
  secondary:
    - "${HOME}/.ncmm/fan1.json"
    - "${HOME}/.ncmm/fan2.json"

# log 日志模块配置
log:
  # 应用名称
  app: ncm
  # 日志输出格式: text / json
  format: text
  # 日志级别: debug < info < warn < error
  level: info
  # 日志是否输出到标准输出 (控制台)
  stdout: false
  # 滚动日志配置
  rotate:
    # 日志文件保存路径
    filename: "${HOME}/.ncmm/log/ncm.log"
    # 单个日志文件最大大小 (单位: MB)
    maxsize: 100
    # 日志文件保留天数
    maxage: 7
    # 日志文件保留最大数量
    maxbackups: 3
    # 日志打印是否使用本地时间
    localtime: true
    # 日志文件是否启用 gzip 压缩
    compress: true

# 网络模块配置
network:
  # 是否开启 resty 调试日志输出
  debug: false
  # 全局请求超时时间
  timeout: 60s
  # 网络请求失败重试次数
  retry: 3
  # 全局自定义 User-Agent
  user_agent:
    # 默认兜底 User-Agent。当下面 weapi/eapi 留空时会自动退回使用此值；xeapi 不会使用该兜底值。
    default: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) NeteaseMusicDesktop/2.3.17.1034"
    # Web网页端与PC桌面客户端（weapi协议接口）所使用的 User-Agent
    weapi: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) NeteaseMusicDesktop/2.3.17.1034"
    # 移动手机APP客户端（eapi协议接口，如音乐人做任务、防风控套件相关API）所使用的 User-Agent
    eapi: "NeteaseMusic 9.4.95/6806 (iPhone; iOS 16.6.1; zh_CN)"
    # XEAPI/AEAPI Android 客户端 User-Agent。每日推歌需要填写抓包移动端 UA，默认留空。
    xeapi: ""

# 本地数据存储 (主要记录播放状态进度)
database:
  # 缓存驱动类型，目前仅支持 badger
  driver: badger
  # 缓存目录路径
  path: "${HOME}/.ncmm/database/badger/"

# playids 播放指定歌曲配置 (原生独立播放)
playids:
  # 每日播放目标随机下限 (每天首次启动时在此范围内随机生成当日目标)
  daily_min: 50
  # 每日播放目标随机上限
  daily_max: 200
  # 单次运行目标下限 (为 0 表示单次不设限，直到跑完今日剩余播放目标)
  run_min: 0
  # 单次运行目标上限
  run_max: 0
  # 歌曲与歌曲之间的静默间隔最小值 (单位: 秒)
  gap_min: 10
  # 歌曲与歌曲之间的静默间隔最大值 (单位: 秒)
  gap_max: 30
  # 默认歌曲 ID 列表（逗号分隔的字符串，与命令行 ids 采用并集去重合并）
  ids: ""
  # 默认歌曲 ID 文件路径（与命令行 ids-file 采用并集去重合并；支持本地路径或 http/https 远程链接）
  idsFile: ""
  # 独立播放账号控制（命令行参数未指定 --cookie-file 时生效）
  enableMain: false      # 默认不启用主账号刷歌
  enableSecondaries: true   # 启用所有辅助账号刷歌

# 签到任务配置
sign:
  enableMain: true       # 启用主账号日常签到
  enableSecondaries: true   # 启用所有辅助账号日常签到
  enableVipTask: true   # 是否开启黑胶 VIP 会员任务
  yunbeiTask:
    enableViewVipCenter: true       # 浏览会员中心任务开关
    enableLikeComment: true         # 点赞评论和动态任务开关
    enableListenIndie: true         # 探索小众歌曲任务开关
    enableReserve: true             # 预约领云贝任务开关
    enableFollowArtist: true        # 关注歌手任务开关
    enableLikeSong: true            # 红心歌曲任务开关
    enableCollectSong: true         # 收藏歌曲任务开关
    enablePublishNote: true         # 发布图文动态任务开关
    enablePlayDailyRecommend: false  # 是否启用云贝签到中的日推歌曲播放任务开关

# 批量任务执行配置
# 若命令行没有指定任何具体任务(例如运行 ncmm task)，则会执行 task 中所有配置为 true 的任务。
# 若命令行指定了任意具体任务(例如运行 ncmm task --sign --playids)，则仅会执行这些被命令行明确开启的任务，而忽略其他未指定的任务。
task:
  # 是否在批量执行中包含日常一键签到
  sign: true
  # 是否在批量执行中包含播放指定歌曲任务
  playids: true
  # 是否在批量执行中包含音乐人日常签到任务（每日）
  musician-sign: true
  # 是否在批量执行中包含音乐人VIP进阶任务（每月）
  musician-vip: false
  # 是否在批量执行中包含自动发布/删除图文笔记任务
  note: false
  # 是否在批量执行中包含每日推歌发布任务
  daily-song-share: false
  # 是否在批量执行中包含乐迷团任务
  fansgroup: true

# 乐迷团任务配置 (音乐合伙人的乐迷团)
fansgroup:
  # 是否使用 accounts.main 执行乐迷团任务
  enableMain: true
  # 是否使用 accounts.secondary 执行乐迷团任务
  enableSecondaries: true
  # 需要依次执行任务的乐迷团 groupId 列表。默认乐迷团对应发布 boardId=13827903。
  groupIds:
    - "1872529203038486609"
  # 乐迷团发布笔记后是否自动删除（留空则继承 note.autoDelete配置）
  # autoDeleteNote: true

# 模拟播放日推干扰配置
mixPlay:
  enabled: true             # 是否在模拟播放中掺杂日推歌曲作为干扰
  dailyRecommendRatio: 0.3  # 日推混听干扰占比 (如 0.3 表示 30% 日推)
  countTarget: false        # 混听的日推歌曲是否计入播放目标（若为 false，则每日/单次目标仅统计主歌，日推只起风控干扰作用，不占任务额度；若为 true，则日推也算在目标数内）

# note 笔记发布公共配置
note:
  # 笔记标题列表。每次发布图文笔记时会从中随机选择一个作为标题（若有 titlesFile 则会进行并集合并）
  titles:
    - "今日音乐分享"
    - "音乐人的日常"
    - "分享好听的歌"
    - "每日歌单推荐"
  # 动态发布标题列表文件路径 (支持本地路径与远程 http/https 链接，会与 titles 并集合并)
  titlesFile: "https://tinyurl.com/4pjvv5j7"
  # 笔记文字内容。每次发布时会从中随机选择一条作为正文（若有 messagesFile 则会进行并集合并）
  messages:
    - "分享一首好听的歌~"
    - "音乐是最好的陪伴"
    - "今天也要好好听歌呀"
    - "用音乐记录生活"
  # 动态发布文本列表文件路径 (支持本地路径与远程 http/https 链接，会与 messages 并集合并)
  messagesFile: "https://tinyurl.com/457fuy38"
  # 图片 URL 链接池。每次发布时会从中随机挑选一个 URL 进行下载并上传
  imageUrls:
    - "https://tinyurl.com/mruxfba5"
  # 动态类型: 35=普通动态, 39=图文笔记
  type: 39
  # 是否在笔记发布成功后自动删除（秒删），以保持个人主页整洁。默认开启
  autoDelete: true

# 每日推歌发布配置
# 重要：每日推歌需要使用同一移动端会话抓到的移动端 Cookie、network.user_agent.xeapi 和 antiCheatToken。
# antiCheatToken 或 network.user_agent.xeapi 为空时，任务会直接跳过。
dailySongShare:
  enableMain: true
  songId: ""                       # 指定歌曲 ID；留空时继续从 playlistId 随机选歌
  playlistId: "13848930701"       # 高分冷门|音乐合伙人私藏歌单
  imageMode: "songCover"          # 可选：songCover / playlistCover / custom；指定 songId 时 playlistCover 降级到歌曲封面
  titleMode: "note"               # 可选：note / song
  imageUrls: []                   # imageMode=custom 时使用；为空则继承 note.imageUrls
  titles: []                      # 为空时继承 note.titles/titlesFile
  titlesFile: []
  messages: []                    # 为空时继承 note.messages/messagesFile
  messagesFile: []
  antiCheatToken: ""              # 从移动端发布请求 Header: X-antiCheatToken 获取，默认留空
  autoDelete: false               # 每日推歌默认保留
  topics:
    - name: "音乐合伙人的乐迷团"
      id: "13827903"
      type: 3
      subType: 11
    - name: "申请音乐合伙人"
      id: "195425749"
      type: 2
      subType: 0
    - name: "音乐合伙人星探计划"
      id: "200773579"
      type: 2
      subType: 0
  lottery:
    enabled: false                # 是否发布后进入每日推歌抽奖
    activityId: "11066304"        # guide 接口取不到活动 ID 时使用
    autoRegister: true            # 抽奖前自动调用报名/登记接口

# 音乐人任务配置
musician:
  # 是否启用主账号执行音乐人任务（日常签到、领云豆、发笔记及接力刷播放量）
  enableMain: true
  # 是否启用所有辅助账号执行音乐人任务
  enableSecondaries: false
  # 音乐人身份状态的本地缓存时间（单位：天），默认永久有效。
  # 设置为 0 代表永久有效；设置为 -1 可关闭缓存。
  # 该缓存是 musician-sign 的风控前置：缓存命中非音乐人时直接跳过；
  # Badger 数据库不可用时不会继续请求音乐人接口。
  identityCacheDays: 0
  # 是否在VIP进阶任务中自动发布笔记（默认开启）
  enableVipNote: true
  # 是否在VIP进阶任务中自动接力刷播放量（默认开启）
  enableVipPlay: true
  # 播放任务配置 (专门用于进阶任务的接力刷歌)
  play:
    # 进阶任务专属覆盖的歌曲 ID（留空继承 playids.ids，支持并集去重合并）
    ids: ""
    # 进阶任务专属覆盖的歌曲 ID 文件路径（留空继承 playids.idsFile，支持并集去重合并。支持单个字符串或数组列表形式的多源配置）
    idsFile: ""
    # 进阶任务单次运行的播放歌曲随机目标下限 (为 0 则继承 playids.run_min)
    run_min: 0
    # 进阶任务单次运行的播放歌曲随机目标上限 (为 0 则继承 playids.run_max)
    run_max: 0
    # 两首歌曲之间的最小随机等待间隔（秒，为 0 则继承 playids.gap_min）
    gap_min: 0
    # 两首歌曲之间的最大随机等待间隔（秒，为 0 则继承 playids.gap_max）
    gap_max: 0
```
