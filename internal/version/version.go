// Package version 保存构建版本信息。
package version

// Version 是当前版本号，格式 fmg<YY><MM>.<NNN>，可在编译时用 -ldflags 覆盖。
var Version = "fmg2609.001"

// Commit 是构建时的 Git 提交号。
var Commit = "dev"

// BuildTime 是构建时间。
var BuildTime = "unknown"
