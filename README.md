<a href="https://github.com/3899/ncmm">
  <img src="https://socialify.git.ci/3899/ncmm/image?description=1&descriptionEditable=%E5%9F%BA%E4%BA%8E%20Go%20%E7%9A%84%E7%BD%91%E6%98%93%E4%BA%91%E9%9F%B3%E4%B9%90%E4%BA%BA%E5%8A%A9%E6%89%8B%EF%BC%9A%E4%B8%80%E9%94%AE%E7%AD%BE%E5%88%B0%E3%80%81%E8%87%AA%E5%8A%A8%E4%BB%BB%E5%8A%A1%E3%80%81%E6%8E%A5%E5%8A%9B%E5%88%B7%E6%92%AD&font=Source%20Code%20Pro&logo=https%3A%2F%2Fp6.music.126.net%2Fobj%2Fwo3DlcOGw6DClTvDisK1%2F62177614927%2F22ad%2F1953%2Fa6cf%2Fe7007953d5942445a0444ca346bd06be.png%3Fraw%3Dtrue&name=1&owner=1&pattern=Floating%20Cogs&theme=Auto" alt="ncmm" />
</a>

<div align="center">
  <br/>

  <div>
    <a href="./LICENSE">
      <img
        src="https://img.shields.io/github/license/3899/ncmm?style=flat-square"
      />
    </a>
    <a href="https://github.com/3899/ncmm/releases">
      <img
        src="https://img.shields.io/github/v/release/3899/ncmm?style=flat-square"
      />
    </a>
    <a href="https://github.com/3899/ncmm/releases">
      <img
        src="https://img.shields.io/github/downloads/3899/ncmm/total?style=flat-square"
      />  
    </a>
    <a href="https://github.com/3899/ncmm/pkgs/container/ncmm">
      <img
        src="https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fghcr-badge.elias.eu.org%2Fapi%2F3899%2Fncmm&query=downloadCount&style=flat-square&logo=docker&label=Docker%20Pulls&color=2496ed"
      />
    </a>
    <a href="https://github.com/3899/ncmm/pkgs/container/ncmm">
      <img
        src="https://ghcr-badge.egpl.dev/3899/ncmm/size?color=%23FF7F50&tag=latest&style=flat-square"
      />
    </a>
  </div>
</div>

# 🎵 ncmm

`ncmm` 是一个专门为**网易云音乐人**设计的命令行助手工具，基于 Go 语言开发。

本项目旨在帮助网易云音乐人 / 普通账号一键完成日常签到、自动执行黑胶 VIP 进阶任务（包括图文笔记自动发布与秒删、多粉丝号接力刷播放量等），帮助音乐人轻松获取并维持黑胶会员权益。工具严格遵循防风控设计，支持多账号安全隔离、播放量分摊回退、日推歌曲混听干扰以及本人播放拦截等安全策略。

---

## 🚀 核心功能

1. **🔑 账号登录管理 (`ncmm login`)**：支持扫码登录、Cookie 导入与 CookieCloud 同步。
2. **🎵 模拟歌曲播放 (`ncmm playids`)**：真实模拟音频流量下载、播放时长等待以及歌曲播放动作上报。
3. **📊 每日播放目标控制**：支持随机每日播放上限、限额自增与达标退出机制，防范防刷检测。
4. **📅 每日任务一键签到 (`ncmm sign`)**：一键完成黑胶 VIP 签到、云贝日常任务做任务（浏览、点赞、小众听歌等）。
5. **🎖️ 音乐人及黑胶进阶任务 (`ncmm musician`)**：日常云豆签到领取、VIP 图文发布及多账号接力刷播放量任务。
6. **🎧 乐迷团任务 (`ncmm fansgroup`)**：一键打卡已加入乐迷团的日常任务，包含播放、发布笔记、点赞分享等。
7. **📝 笔记发布独立命令 (`ncmm note`)**：单独发布图文动态，并支持发布后自动秒删，维持个人主页整洁。
8. **📢 每日歌曲分享与抽奖 (`ncmm daily-song-share`)**：自动选择歌曲（固定或随机）发布到移动端动态，支持挂载专属活动话题，并在分享成功后自动参与活动抽奖，支持发布后自动删除动态以保持主页整洁。
9. **🎁 黑胶会员赠送与领取 (`ncmm vip-member-gift`)**：自动将账号内多余的免费黑胶会员天数生成赠送 Token 并上报云端；同时支持从云端拉取可用 Token 自动为自己领取会员天数，支持私有化云服务部署。
10. **📁 灵活的 `--home` 隔离机制**：多账号下配置、Cookie、数据库、日志自动隔离，安全无干扰。

---

## ⚡ 快速上手

### 1. 账号登录
推荐使用 Cookie 导入：
```bash
# 导入主账号 Cookie 并标记为 -m (Main)
./ncmm login cookie '你的MUSIC_U_cookie串' -m
```

### 2. 一键运行批量任务
运行以下命令，即可在默认工作目录下根据配置文件规则自动执行日常一键打卡签到任务：
```bash
./ncmm task
```
*(更多有关多账号隔离管理和 Docker 自动部署，请参阅下方详细文档。)*

---

## 📚 详细文档

为了获得更好的阅读体验，本项目的详细使用手册已拆分为以下子文档：

* ⚙️ [配置文件详解](docs/configuration.md) — 了解 `config.yaml` 详细配置字段及各项任务开关说明。
* 🐳 [Docker 部署指南](docs/docker.md) — 了解如何通过 Docker/Docker Compose 一键部署并配合定时任务运行。
* 📖 [命令行使用说明](docs/cli.md) — 查看完整的命令树、通用参数以及所有子命令的使用实例。
* 👥 [多账号隔离最佳实践](docs/multi-accounts.md) — 学习如何使用 `--home` 管理多个粉丝账号，实现全自动接力刷量。
* 📝 [版本更新记录](docs/changelog.md) — 查看历史版本的新增功能、架构优化与 Bug 修复记录。

---

## ⚠️ 免责声明

本项目仅供学术研究和 Golang 学习探讨之用，请勿用于任何商业用途或违反网易云音乐服务条款的行为。对于使用本项目带来的任何账号封禁、数据丢失等不良后果，由使用者自行承担，本项目不提供任何连带保证。

---

## 🎖️ 鸣谢

### 👥 贡献者

感谢大家为 ncmm 做出的宝贵贡献！如果你也希望为 ncmm 做出贡献，请查阅 [贡献指南](./.github/CONTRIBUTING.md)。

<a href="https://github.com/3899/ncmm/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=3899/ncmm" />
</a>


### 📦 参考项目
| 项目 | 说明 |
| :--- | :--- |
| [chaunsin/netease-cloud-music](https://github.com/chaunsin/netease-cloud-music) | 网易云音乐 API |
| [crossgg/netease-cloud-music](https://github.com/crossgg/netease-cloud-music) | 网易云音乐人任务 |
| 所有依赖的开源项目 | |