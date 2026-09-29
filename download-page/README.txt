YCFMG 下载站
=============

本目录即下载页根目录，已由 lucky 反向代理对外提供：

  https://ycfmg.202693.xyz/

目录内容：

  index.html      下载页（版本、更新记录、各平台下载）
  version.json    版本清单，应用「设置 → 在线更新」读取此文件
  logo.svg        站点图标
  files/          安装包与二进制

lucky 反代目标指向本目录所在的静态服务即可，
需允许 .fpk / .tar.gz / .exe / .json 等扩展名。

更新下载站
--------
在源码仓库执行：

  bash scripts/build.sh           重新构建各平台二进制
  bash scripts/build-fpk.sh       用官方 fnpack 重新打包 fpk
  bash scripts/deploy-download.sh 重新发布到本目录

应用内在线更新
------------
检查顺序：本页 version.json → GitHub Release → Docker Hub 标签。
GitHub 与 Docker Hub 在境内通常需要代理，在应用「设置 → 在线更新」中填写即可。

版本：fmg2609.003