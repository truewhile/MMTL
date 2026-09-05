# 【开源推荐】MeBox：把 NAS / 网盘 / 远程 Emby 统一家里的观影入口，Docker 一键部署

> 配图已托管在 GitHub 仓库（`raw.githubusercontent.com` 直链），发帖时可直接引用，或下载 `docs/tutorial-screenshots/` 后作为附件上传。

---

## 写在前面

给论坛的朋友们推荐一个我维护的开源项目 —— **MeBox**，一个面向 NAS 与家庭影音场景的**自托管私人媒体中心**（GPL-3.0，Go + React）。

GitHub：https://github.com/truewhile/MeBox

一句话介绍：**部署一个服务，同时获得媒体库后台、网盘 STRM 整理、Emby 客户端协议网关三件套。** 内置完整 Emby/Jellyfin 服务端协议实现——手机、电视、平板上的 Infuse、SenPlayer、Fileball、Emby/Jellyfin 官方客户端直接「添加 Emby 服务器」就能连，一套账号体系全搞定，Emby 老用户零学习成本。

项目 fork 自 MediaStationGo 并持续二开，围绕网盘播放、任务队列、远程挂载和权限体系做了大量增强。

---

## 它能解决什么问题？

家里看电影电视的痛点，MeBox 基本一把梭：

| 痛点 | MeBox 的解法 |
| --- | --- |
| 硬盘散落各处，海报墙乱七八糟 | 多根目录媒体库 + TMDb/Bangumi/Douban 自动刮削，海报墙、继续观看、多季剧集一应俱全 |
| 网盘资源看一部下一部太麻烦 | OpenList / CloudDrive2 / 115 / WebDAV 接入，STRM 同步 + 直链/302 播放，不占本地空间 |
| 已经有一台 Emby，出门还得开 App | **远程 Emby 挂载**：把远程 Emby 的媒体库直接挂进 MeBox 界面统一浏览 |
| 家人乱动设置、小孩看不该看的 | 多用户 + 有效期 + 成人内容开关 + 播放配置 PIN，细粒度权限 |
| 每个设备装一套专属 App 太折腾 | **完整兼容 Emby/Jellyfin 客户端**：Infuse、SenPlayer、Fileball、官方客户端按「添加 Emby 服务器」填地址 + MeBox 账号即可，海报墙、观看进度、多用户直接同步 |

---

## 特点一览

**1. 现代化 Web UI，海报墙开箱即用**

![登录页](https://raw.githubusercontent.com/truewhile/MeBox/main/docs/tutorial-screenshots/01-login.png)

深色系登录页，默认账号 `admin / admin123`（首次登录请立即改密）。

![首页](https://raw.githubusercontent.com/truewhile/MeBox/main/docs/tutorial-screenshots/02-home.png)

首页自带焦点推荐轮播 + 媒体库入口卡片，继续观看、最近添加直接呈现。

**2. 媒体库与刮削**

![媒体库总览](https://raw.githubusercontent.com/truewhile/MeBox/main/docs/tutorial-screenshots/03-libraries.png)

20 个媒体库、1600+ 条目一眼尽收：每库自带封面拼贴、条目数统计，支持「全库修复+重刮」「刮削队列」批量处理。

![海报墙](https://raw.githubusercontent.com/truewhile/MeBox/main/docs/tutorial-screenshots/04-library-posters.png)

库内海报墙带评分、集数角标，支持按最后集添加日期排序，点开即看。

**3. 详情页与多季管理**

![详情页](https://raw.githubusercontent.com/truewhile/MeBox/main/docs/tutorial-screenshots/05-media-detail.png)

剧情简介、类型标签、多季分集（特别篇/第 1-N 季）、每集缩略图与时长；一键立即播放、调用外部播放器、加入收藏。

**4. Emby/Jellyfin 客户端无缝兼容**

这是我最想强调的一点：**MeBox 内置了完整的 Emby 服务端协议实现**。手机、电视、平板上的 Infuse、SenPlayer、Fileball，甚至 Emby/Jellyfin 官方客户端，都不需要任何插件或改造——按「添加 Emby 服务器」填入 `http://服务器IP:18080`，用 MeBox 账号登录，海报墙、观看进度、收藏、多用户权限全部无缝衔接。已经习惯 Emby 生态的朋友可以零成本迁移，家人只用电视端 App 也完全无感。

**5. 网页播放器 + 弹幕自动匹配**

![播放器与弹幕](https://raw.githubusercontent.com/truewhile/MeBox/main/docs/tutorial-screenshots/06-player-danmu.png)

内置网页播放器支持 HLS 转码、字幕、播放配置档；**弹幕按剧名自动匹配全季分集**（截图中自动匹配到《一拳超人》39 集），屏幕占比/透明度/字号随意调，追新番体验直接拉满。

**6. 网盘 STRM：网盘当本地盘用**

![STRM 管理](https://raw.githubusercontent.com/truewhile/MeBox/main/docs/tutorial-screenshots/07-strm-cloud.png)

添加网盘账号（**115 支持二维码扫码登录**）→ 添加同步目录 → 系统把网盘/本地目录里的视频生成 `.strm` 文件，元数据经下载/上传队列双向同步，播放走直链/302 不落盘。

**7. 远程 Emby 挂载（特色功能）**

![Emby 挂载](https://raw.githubusercontent.com/truewhile/MeBox/main/docs/tutorial-screenshots/08-emby-mount.png)

已有远程 Emby 服务器？填一次账号，按需勾选要挂载的媒体库（支持同服务器多线路自动切换、直连开关、排序），远程库直接出现在 MeBox 首页，不必再开 Emby 客户端。

**8. 任务队列统一管理**

![任务队列](https://raw.githubusercontent.com/truewhile/MeBox/main/docs/tutorial-screenshots/09-task-queue.png)

刮削 / 下载 / 上传三类任务统一看板，排队中、进行中、已匹配、失败分类计数，支持搜索与批量清理。

**9. 下载与自动整理**

![文件管理](https://raw.githubusercontent.com/truewhile/MeBox/main/docs/tutorial-screenshots/11-file-manager.png)

接入 qBittorrent，下载目录定时自动整理入媒体库：智能分类子库、自动注册目的地媒体库、复制/移动/硬链/软链多种整理方式，命名规则可配。

**10. 多用户与权限**

![用户管理](https://raw.githubusercontent.com/truewhile/MeBox/main/docs/tutorial-screenshots/12-user-admin.png)

管理员/普通用户分级、单实例用户数上限、账号有效期、成人内容开关、播放配置 PIN——给家人开号放心给。

**11. 运维省心**

![系统设置](https://raw.githubusercontent.com/truewhile/MeBox/main/docs/tutorial-screenshots/10-settings.png)

FFmpeg/FFprobe 一键下载安装、转码与硬件加速开关、TMDb 语言、识别词、弹幕、Adult/NSFW 开关全在设置页分组管理；另有 DLNA 投屏、存储统计、海报墙聚合视图：

![海报墙聚合](https://raw.githubusercontent.com/truewhile/MeBox/main/docs/tutorial-screenshots/13-poster-wall.png)

---

## 使用教程：从零到海报墙只要 5 步

### 第 1 步：Docker 一键部署

推荐 Docker Compose（仓库提供 4 份互相独立的完整模板，无需 `.env`）：

```bash
mkdir -p MeBox && cd MeBox

# 最省心：单镜像 + 内置 SQLite
curl -fsSL https://raw.githubusercontent.com/truewhile/MeBox/main/docker-compose.simple.yml -o docker-compose.yml

# 多用户/大数据量可选 PostgreSQL 档、Redis 档、OpenSearch 档，见仓库 README「部署档位」

docker compose up -d
```

浏览器访问 `http://服务器IP:18080`，镜像：`ghcr.io/truewhile/mebox:latest`（amd64 / arm64 都有，也提供 Windows/Linux/macOS 单文件可执行程序，不想装 Docker 直接下载跑）。

### 第 2 步：登录并修改密码

默认账号 `admin / admin123`，登录后右上角头像 → 个人资料修改密码。

### 第 3 步：创建媒体库 + 扫库

后台 → 媒体库 → 管理媒体库，添加本地路径（Docker 部署记得填**容器内**路径，如 `/media/电影`，`volumes` 左侧挂宿主机真实目录）→ 执行扫库。

### 第 4 步：配置元数据刮削

系统设置 → 外部 API，填入 TMDb / Bangumi / Douban 等 API Key；媒体库页可对单个库「全库修复+重刮」，刮削进度在任务队列实时可见。

### 第 5 步（可选但强烈推荐）：

- **网盘用户**：STRM 管理 → 添加网盘账号（115 可扫码）→ 添加同步目录 → 生成 STRM 后直链播放；
- **已有 Emby**：Emby 挂载 → 添加 Emby 账号 → 勾选要挂载的媒体库；
- **第三方播放器（Emby 客户端全兼容）**：Infuse / SenPlayer / Fileball / Emby、Jellyfin 官方客户端，按「添加 Emby 服务器」填 `http://服务器IP:18080`，用 MeBox 账号登录即可，原有使用习惯完全不变；
- **下载党**：设置里接入 qBittorrent，配置下载目录自动整理，下完即入库。

### 路径映射小抄（Docker 最常见坑）

```yaml
volumes:
  - /vol1/1000/Media:/media        # 左：宿主机真实路径；右：容器内路径（网页里填这个）
environment:
  MEBOX_MEDIA_DIR: /vol1/1000/Media
  MEBOX_MEDIA_CONTAINER_DIR: /media
```

硬链接要求同一文件系统/子卷，跨盘请改复制或软链。

---

## 部署档位怎么选？

| 档位 | 文件 | 组件 | 适合 |
| --- | --- | --- | --- |
| 极简 | `docker-compose.simple.yml` | 单镜像 + SQLite | 个人使用、低配设备 |
| 标准 | `docker-compose.yml` | + PostgreSQL | 多用户家庭共享 |
| 增强 | `docker-compose.standard.yml` | + Redis | 大媒体库高频访问 |
| 搜索 | `docker-compose.search.yml` | + OpenSearch | 超大库全文搜索 |

---

## 技术栈与致谢

- 后端：Go · Gin · GORM · SQLite/PostgreSQL · 可选 Redis / OpenSearch
- 前端：React 18 · Vite · TypeScript · Tailwind CSS · Zustand
- 部署：Docker Compose 多档模板，amd64/arm64 镜像 + 单文件可执行

感谢上游 [MediaStationGo](https://github.com/ShukeBta/MediaStationGo) 的奠基，网盘同步/STRM/整理部分参考了 [qmediasync](https://github.com/qicfan/qmediasync) 的思路。

---

## 链接

- GitHub：https://github.com/truewhile/MeBox
- Issue / PR：欢迎提 bug（附部署方式+复现步骤+日志）与功能建议
- License：GPL-3.0

觉得有用的话求个 Star ⭐，也欢迎论坛里的朋友反馈使用体验，我长期维护。
