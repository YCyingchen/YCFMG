// Package web 内嵌前端静态资源。
package web

import (
    "embed"
    "io/fs"
)

//go:embed all:index.html all:css all:js all:img
var assets embed.FS

// Assets 返回可直接用于 http.FileServer 的文件系统。
func Assets() fs.FS { return assets }