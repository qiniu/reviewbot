# phony

模拟发送 Github 事件，主要参考 [phony](https://github.com/kubernetes-sigs/prow/tree/main/cmd/phony)

## 使用例子

在当前目录下执行:

```bash
go run . --hmac=<your-webhook-secret> -payload ./pr-open.json --event=pull_request
```
