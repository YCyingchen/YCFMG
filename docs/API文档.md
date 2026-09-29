# YCFMG API 文档

所有接口返回统一 JSON 信封：成功为 `{"ok":true,"data":...}`，失败为 `{"ok":false,"error":"原因"}`。
除 `/api/health`、`/api/public`、`/api/login` 与 `/s/*` 分享接口外，其余接口均需登录会话 Cookie。

## 1. 系统与账号

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/health` | 健康检查，返回名称、版本、运行时长 |
| GET | `/api/public` | 公开配置：品牌名、文件库、绑定域名、当前访问地址、语义检索开关 |
| POST | `/api/login` | 登录，body `{username,password}`，成功后写入会话 Cookie |
| POST | `/api/logout` | 注销当前会话 |
| GET | `/api/me` | 当前登录用户 |
| POST | `/api/password` | 修改密码，body `{old,new}` |
| GET | `/api/stats` | 统计：文件类型分布、总量、占用、分享数、缓存体积、索引进度 |
| GET | `/api/settings` | 完整配置（域名、文件库、索引、分享、语义） |
| POST | `/api/settings` | 保存运行时配置（`public_urls`、品牌名、自动标签、哈希容差） |
| POST | `/api/thumbs/clean` | 清空缩略图缓存 |

## 2. 文件管理

### GET `/api/fs/list`

参数：`path`（绝对路径）、`hidden=1`（显示隐藏文件）。

返回 `{path, parent, writable, crumbs[], entries[]}`；每个条目包含：

```json
{
  "id": 12, "name": "日落海边.jpg", "path": "/data/photos/旅行/日落海边.jpg",
  "is_dir": false, "size": 35999, "mtime": 1790698090, "kind": "image", "ext": ".jpg",
  "writable": true, "indexed": true, "width": 1200, "height": 800,
  "color": "#505070", "classify": "风景", "tags": "风景,横向,高清",
  "favorite": false, "rating": 0, "camera": "", "taken_at": 0,
  "file_count": 0, "image_count": 0
}
```

目录条目会附带 `file_count` / `image_count`（用于资源管理器显示目录摘要）。

### 其它文件接口

| 方法 | 路径 | Body / 参数 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/fs/tree` | - | 文件库与浅层目录树（导航窗格） |
| POST | `/api/fs/mkdir` | `{path,name}` | 新建文件夹 |
| POST | `/api/fs/rename` | `{path,new_name}` | 重命名 |
| POST | `/api/fs/move` | `{paths[],dst}` | 移动到目录 |
| POST | `/api/fs/copy` | `{paths[],dst}` | 复制到目录 |
| POST | `/api/fs/delete` | `{paths[],forever}` | 删除（默认进回收站） |
| POST | `/api/fs/upload` | multipart：`path`,`file`（可选 `upload_id`,`chunk`,`total` 分片） | 上传 |
| GET | `/api/fs/trash` | - | 回收站列表 |
| POST | `/api/fs/trash/restore` | `{id}` | 恢复 |
| POST | `/api/fs/trash/purge` | `{ids[]}`（空数组清空） | 彻底删除 |
| GET | `/api/fs/index` | `path`,`recursive=1` | 立即索引指定目录 |
| GET | `/api/file` | `path`,`download=1` | 原文件流（支持 HTTP Range） |
| GET | `/api/thumb` | `path`,`size` | 缩略图（JPEG，默认 256） |

## 3. 图库与检索

### GET `/api/gallery`

参数：`q`（关键词）、`lib`、`kind`（默认 image）、`classify`、`color`、`camera`、`tag`、
`favorite=1`、`rating`、`from`/`to`（时间戳）、`min_w`/`min_h`、`sort`（taken/mtime/size/name/rating/random）、
`desc=1`、`limit`、`offset`。

返回 `{items[], total, offset, limit}`。

### 检索接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/search` | 以文搜图，参数同 `/api/gallery`，`q` 为自然语言 |
| POST | `/api/search/image` | 以图搜图，multipart 上传 `file`，返回相似图并按相似度排序 |
| GET | `/api/search/similar` | 以库内图片为基准，参数 `id`、`min_score`（0-100，默认 45） |
| GET | `/api/search/suggest` | 关键词建议：分类、标签、相机、色系、年份 |
| GET | `/api/gallery/facets` | 分组统计：year/month/classify/color/camera/lib/ext/lens |
| GET | `/api/gallery/duplicates` | 重复图分组，参数 `lib`、`tolerance` |
| GET | `/api/gallery/map` | 含 GPS 的图片点位 |
| POST | `/api/file/meta` | 修改收藏/评分/隐藏/备注/标签：`{id或path, favorite, rating, hidden, note, add_tags[], del_tags[]}` |
| GET | `/api/tags` | 标签列表（含关联数量） |

## 4. 分享

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/shares` | 分享列表，返回 `items[]` 与总览统计 |
| POST | `/api/shares` | 创建分享 |
| GET | `/api/shares/{id}` | 分享详情 |
| PATCH | `/api/shares/{id}` | 修改标题、描述、密码、有效期、下载限制、停用 |
| DELETE | `/api/shares/{id}` | 删除分享 |
| GET | `/api/shares/{id}/links` | 该分享在全部绑定域名下的链接 + 最近访问记录 |

创建分享请求体：

```json
{
  "title": "旅行相册",
  "descr": "国庆出行",
  "paths": ["/data/photos/旅行/日落海边.jpg"],
  "password": "1234",
  "expire_days": 7,
  "max_download": 0,
  "allow_download": true
}
```

### 公开分享页

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/s/{token}` | 专属分享页（服务端渲染；加密分享先展示密码页） |
| POST | `/s/{token}` | 提交提取密码，成功写入签名 Cookie 并跳转 |
| GET | `/s/{token}/thumb?p=路径` | 分享内缩略图（校验路径属于该分享） |
| GET | `/s/{token}/raw?p=路径&download=1` | 分享内原文件 |
| GET | `/s/{token}/zip` | 打包下载全部分享内容 |

## 5. 索引管理

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/index/status` | 当前索引进度、最近任务、统计 |
| POST | `/api/index/scan` | `{lib,path,force}`，`path` 为空时全量扫描（异步） |
| GET | `/api/index/jobs` | 索引任务历史 |
| GET | `/api/libraries` | 各文件库的文件数、图片数、占用、是否可写 |

## 6. 错误码

| HTTP | 含义 |
| --- | --- |
| 200 | 成功（业务失败也在 200 内以 `ok:false` 表达时仅限部分接口） |
| 400 | 参数错误或操作非法（如越过只读库） |
| 401 | 未登录或会话过期 |
| 403 | 路径越界、分享越权、分享已停用 |
| 404 | 资源不存在 |
| 500 | 服务端错误 |