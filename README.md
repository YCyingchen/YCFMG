<div align="center">
  <img src="assets/logo.svg" width="112" height="112" alt="YCFMG"/>
  <h1>YCFMG</h1>
  <p><b>文件管理 × 智能图库 —— 一台设备上的 Windows 式文件中心与以图搜图图库</b></p>
  <p>
    <code>fmg2609.001</code> ·
    <img src="https://img.shields.io/badge/Go-1.27-00ADD8" alt="Go"/>
    <img src="https://img.shields.io/badge/Docker-ycyingchen%2Fycfmg-2496ED" alt="Docker"/>
    <img src="https://img.shields.io/badge/fnOS-fpk-FF6A00" alt="fnOS"/>
    <img src="https://img.shields.io/badge/License-MIT-green" alt="MIT"/>
  </p>
</div>

---

## 它是什么

YCFMG（**Y**ing**C**hen **F**ile **M**anager & **G**allery）把「传统文件管理器」和「现代智能图库」合成一个应用：

| 维度 | 传统文件管理器 | 传统图库 | YCFMG |
| --- | --- | --- | --- |
| 目录自由度 | 强（真实路径、任意层级） | 弱（必须导入库） | **强**：直接挂载真实目录，不搬迁文件 |
| 查找方式 | 只能按文件名 | 只能按时间/相册 | **双向**：文件名 + 内容语义 + 以图搜图 |
| 界面心智 | 树 + 列表 + 右键 | 卡片瀑布流 | **同一份数据两套视图**，一键切换 |
| 分享 | 依赖外部网盘 | 弱 | **专属分享页 + 多域名绑定** |

一句话：**用文件管理器的自由度，配图库的检索体验。**

## 核心特性

### 一、Windows 式文件管理
- 左侧导航窗格 + 顶部面包屑/地址栏 + 工具栏 + 右侧详细信息面板 + 底部状态栏
- 三种视图：大图标、平铺列表、详细信息（表头可排序）
- 多选（Ctrl 点选 / Shift 连选 / Ctrl+A / 框选取消）、右键上下文菜单
- 快捷键：Ctrl+C/X/V、F2 重命名、Delete 删除、Enter 打开、Esc 关闭灯箱
- 真实文件操作：新建文件夹、上传（多文件 / 文件夹 / 拖拽）、重命名、移动（含拖拽到目录）、复制、粘贴、删除
- 回收站：删除可恢复，元数据记录原路径，支持彻底删除与一键清空
- 只读模式：全局开关 + 单个文件库开关
- 亮色 / 暗色主题，跟随系统并可手动切换

### 二、智能图库
- 时间线瀑布流，按日分组，自动排版（响应式列数）
- 筛选维度：分类、色调、收藏、文件库
- EXIF：拍摄时间、相机、镜头、光圈、快门、ISO、焦段、GPS
- 自动标签：分类、画幅、分辨率档位、色调、亮度/对比度/清晰度、相机机型、地理信息
- 分类体系：照片、人物、风景、美食、夜景、截图、长截图、文档、二维码、表情包、壁纸、头像、图标、地图、极简、动漫
- 重复图检测与相似聚合（感知哈希分桶 + 并查集聚类）

### 三、三种检索
1. **以图搜图**：上传 / 拖拽 / 库内图片一键检索；pHash + dHash + aHash + 颜色直方图融合打分；
   命中重复会标记「疑似重复」。**无模型依赖，开箱即用。**
2. **以文搜图**：中文分词（单字 + 二元组）+ 50 组同义词扩展 + 多字段加权打分（标签 3.5 / 文件名 3.0 / 分类 2.5 / 目录 1.2 / 相机 1.0）。
3. **语义增强（可选）**：配置任意 OpenAI 兼容的 `/v1/embeddings` 端点后启用向量检索；未配置时前两条路径不受影响。

### 四、分享与专属分享页
- 分享单文件、多文件、整个文件夹；可设标题、描述、提取密码、有效期、下载次数上限、禁止下载
- 专属分享页 `/s/{token}`：服务端渲染，封面、信息胶囊、瀑布流、灯箱、打包下载（zip）、复制链接
- **多域名绑定**：访问者用哪个域名打开，分享链接就用哪个域名生成；后台可查看全部域名链接并标注首选/当前
- 访问统计：浏览次数、下载次数、最近访问来源 IP 与行为
- 反向代理友好：识别 `X-Forwarded-Proto/Host`，支持子路径部署

### 五、部署形态
- **fpk**：飞牛 NAS（fnOS）应用中心手动安装
- **二进制**：Linux amd64 / arm64 静态单文件，无运行时依赖
- **Docker**：`ycyingchen/ycfmg`，多架构（amd64 + arm64）
- **VPS**：systemd 单元，附一键部署脚本（含 SSH 远程部署）

## 快速开始

### Docker（推荐）
```bash
docker run -d --name ycfmg \
  -p 8686:8686 \
  -e YCFMG_ADMIN_PASSWORD="你的强密码" \
  -e YCFMG_PUBLIC_URLS="http://192.168.1.8:8686,https://fm.example.com" \
  -v /vol1/1000/Photos:/data/photos \
  -v /vol1/1000/ycfmg:/config \
  --restart unless-stopped \
  ycyingchen/ycfmg:fmg2609.001
```
打开 `http://<设备IP>:8686`。

### Docker Compose
```bash
cd docker && docker compose up -d
```

### 二进制
```bash
bash scripts/build.sh                      # 构建全部平台
sudo ./dist/ycfmg-linux-amd64 --config /opt/ycfmg/etc/config.yaml
```

### 飞牛 NAS（fpk）
```bash
bash scripts/build-fpk.sh                  # 产出 dist/YCFMG_fmg2609.001.fpk
```
然后在「应用中心 → 手动安装」中选择该文件。

## 目录结构

```
YCFMG/
├── cmd/ycfmg/            程序入口（含优雅关闭）
├── internal/
│   ├── config/           配置加载（YAML 子集解析器 + 环境变量覆盖）
│   ├── store/            SQLite 数据层（纯 Go 驱动，无 cgo）
│   ├── vision/           感知哈希、颜色直方图、EXIF、规则分类
│   ├── thumbs/           多尺寸缩略图缓存（ffmpeg 兜底 HEIC）
│   ├── indexer/          增量索引、按需索引、定时扫描
│   ├── search/           以文搜图、以图搜图、重复聚类、语义适配
│   ├── fsapi/            文件系统操作与路径守卫
│   ├── share/            分享管理、多域名、分享页渲染
│   ├── auth/             账号、会话、bcrypt
│   ├── media/            HTTP Range 流式传输、zip 打包
│   ├── server/           HTTP 路由与全部接口处理器
│   ├── version/          版本信息（可用 ldflags 注入）
│   ├── util/ logx/       通用工具与日志
│   └── web/              内嵌前端（go:embed）
├── web/                  前端源码（原生 HTML/CSS/JS，无构建步骤）
├── configs/              配置示例
├── docker/               Dockerfile、compose、entrypoint
├── fnos/                 fpk 清单与生命周期脚本
├── scripts/              构建、打包、推送、部署脚本
├── docs/                 架构、API、部署、计划与建设记录
└── assets/               Logo（SVG）与图标（PNG）
```

## 版本号规则

`fmg<YY><MM>.<NNN>` —— 例如 `fmg2609.001`：
- `26` 年份（2026 年）
- `09` 月份（9 月）
- `001` 当月第几次修改（每次发版递增）

## 已验证

本版本已在测试实例上完成端到端验证：

| 项 | 结果 |
| --- | --- |
| 二进制启动 | `YCFMG fmg2609.001 (dev, linux/amd64, 2026-09-30)` |
| 登录 / 会话 | 通过（401 保护生效） |
| 目录浏览 | 通过（面包屑、目录图片计数正常） |
| 增量索引 | 通过（5 张测试图入库，跳过未变化文件） |
| 自动分类与标签 | 通过（风景 / 美食 / 截图 / 夜景 / 人物 及 8 类自动标签） |
| 以图搜图 | 通过（相同图片相似度 100%，标记「疑似重复」） |
| 以文搜图 | 通过（搜索「风景」命中风景类图片） |
| 缩略图 | 通过（256/768 两档 JPEG） |
| 分享页 | 通过（服务端渲染，瀑布流 + 灯箱） |
| 分享打包下载 | 通过（zip 内含全部条目） |
| 多域名链接 | 通过（3 个域名，标注首选/当前） |

## 文档

- [架构设计](docs/架构设计.md)
- [API 文档](docs/API文档.md)
- [部署指南](docs/部署指南.md)
- [开发计划](docs/开发计划.md)
- [建设记录](docs/对话记录.md)

## 许可

MIT © 2026 ycyingchen

## 交付状态（fmg2609.001）

| 交付物 | 位置 / 状态 |
| --- | --- |
| 源码仓库 | https://github.com/YCyingchen/YCFMG（main 分支，含 vfmg2609.001 标签） |
| Docker 镜像 | docker pull ycyingchen/ycfmg:fmg2609.001（linux/amd64，149 MB） |
| fpk 安装包 | dist/YCFMG_fmg2609.001.fpk（5.4 MB，含 manifest、图标与生命周期脚本） |
| 静态二进制 | dist/ycfmg-linux-amd64 / arm64、darwin-amd64 / arm64、windows-amd64.exe |
| 已部署实例 | 飞牛 NAS 192.168.1.8:8686（systemd 服务 ycfmg，开机自启） |
| 验收截图 | docs/screenshots/ 共 9 张 |

> arm64 镜像请在支持 qemu 的机器上执行 `bash scripts/push-docker.sh` 生成多架构 manifest。

### 已部署实例的初始账号

浏览器打开 http://192.168.1.8:8686 ，使用 `admin` / `mPhIzFbjBUzZ` 登录，并请立即在「设置 → 账号安全」中修改密码。
