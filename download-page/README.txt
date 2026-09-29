YCFMG 下载站说明
=================

本目录是一个纯静态下载页，无需数据库与后端：

  index.html          下载页（已内联样式，含文件大小）
  logo.svg            站点图标
  files/              安装包与二进制

一、如何对外提供访问
-------------------
方式 A：任意静态服务器（推荐）

  docker run -d --name ycfmg-download \
    -p 8899:80 \
    -v "/vol5/1000/空间4/YCFMG:/usr/share/nginx/html:ro" \
    --restart unless-stopped \
    nginx:alpine

  然后访问 http://<设备IP>:8899

方式 B：飞牛自带 Web 服务
  把本目录配置为站点根目录即可（确保允许下载 .fpk / .tar.gz / .exe 等扩展名）。

方式 C：临时预览
  cd "/vol5/1000/空间4/YCFMG" && python3 -m http.server 8899

二、更新下载站
-------------------
在本机源码仓库执行：

  bash scripts/build.sh          # 重新构建各平台二进制
  bash scripts/build-fpk.sh      # 用官方 fnpack 重新打包 fpk
  bash scripts/deploy-download.sh  # 重新发布到本目录

三、校验
-------------------
  sha256sum files/*              查看各文件校验值
  tar -tzf files/YCFMG_fmg2609.001.fpk | head   查看 fpk 结构

版本：fmg2609.001