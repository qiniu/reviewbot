---
title: note-check
sidebar_position: 1
---

参考[go doc note](https://pkg.go.dev/go/doc#Note)要求，**Reviewbot** 推荐在写Note时,带上自己的GitHub ID，类似:

```go
// TODO(CarlJi): 需要处理xx情况
```

这样好处是能清晰的知道这个标记是谁加的。

:::tip
当然，通过`git history`也能看到这行是谁改的，但不够精确，容易被更改。
:::