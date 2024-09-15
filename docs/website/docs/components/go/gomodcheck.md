---
title: gomodcheck
sidebar_position: 3
---

`gomodcheck` 是一个专门用于检查 Go 项目中 `go.mod` 文件的 linter。它的主要目的是限制跨仓库的 local replace 使用，以确保项目依赖管理的一致性和可重现性。

比如:

```go
replace github.com/qiniu/go-sdk/v7 => ../another_repo/src/github.com/qiniu/go-sdk/v7
```

`../another_repo` 代表当前仓库的父目录下的 `another_repo` 目录. 这种用法非常的不推荐.

### 为什么要限制跨仓库的 local replace？

1. **可重现性**: 跨仓库的 local replace 使得构建过程依赖于本地文件系统结构，这可能导致不同环境下的构建结果不一致。

2. **依赖管理**: 它绕过了正常的依赖版本控制，可能引入未经版本控制的代码。

3. **协作困难**: 其他开发者或 CI/CD 系统可能无法访问本地替换的路径，导致构建失败。

4. **版本跟踪**: 使用 local replace 难以追踪依赖的具体版本，增加了项目维护的复杂性。

### 这种情况推荐怎么做？

尽可能使用正式发布的依赖版本, 即使是 private repo 也是一样的。

:::info

可以使用 go env -w GOPRIVATE 来设置私有仓库, 方便 go mod 下载依赖.

:::
