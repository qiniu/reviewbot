## Code Review Comments Learning

## Details

* https://github.com/goplus/pkgsite/pull/1#discussion_r1512593188
    * golangci-lint 的 typecheck 是指编译阶段遇到了错误
      * 要保证提交的是完整可运行的代码
    * 导入的包，我们一般会要求分组，标准库放最上面
      * 参见 https://go.dev/wiki/CodeReviewComments#imports

* https://github.com/goplus/pkgsite/pull/1#discussion_r1513925551
    * 改动原有逻辑要慎重
    * 你的PR(代码)就是你的门面，务必多花些心思

## Reference

* [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
