
## Proposal: gitlab 集成

## Background

> see [issue](https://github.com/qiniu/reviewbot/issues/215)

当前reviewbot只支持以github作为源码管理的代码合并review工具，为了使reviewbot适合更多用户的的使用，让reviewbot与其他源码管理平台的也能无缝集成使用。

所以本提案的目标是使reviewbot与gitlab集成，在gitlab代码Mergerequest流程中触发reviewbot代码检查，并将检查结果回写在gitlab Mergerequest comment和discussion中。

## 设计原则

从简化管理和使用的角度考虑:

- 尽量减少用户配置。 
> webhook ，mergrequest 触发reviewbot工作
> accesstoken：gitlab 相关api调用
> ssh_key: 代码下载
- 
- 与当前代码review 流程无缝集成，无感接入，减低用户学习上使用成本。
> mergerequest 页面的comment 展示总结果
> 代码 discussion 中 展示详细信息，定位到异常代码行
- 尽量保持与其他平台接入方式大致一致
- 节点的选择是基于 repo/linter 粒度的
  > 当然，如果后续有需要，可以做更灵活的配置，比如基于 repo 粒度，基于 org 粒度，或者基于 linter 粒度

## 整体设计

不破坏现有部署架构，请求路径仍然是 webhook ->MergeRequestEvent->LinterCommand> Report,但是 由于gitlab 与github 功能上，API提供上都有些差异，在细节上又些许区别
webhook ->MergeRequestEvent→ gitpull代码-> DiffFile → lineter command-> comments-on-merge-requests/discussions。


> 方便起见，Reviewbot 实例下面会简称为节点

在ServerHttp入口中增加对GitLab Mergerequest事件的处理。

增加 gitLabHandle(ctx context.Context, event *gitlab.MergeRequestEvent)，增加对gitlab.MergeRequestEvent的解析处理，解析处理出reviebot需要用到的org，repo等相关信息

获取变动文件，

根据变动文件，执行对应的Linter，与github保持一致

报告
根据检查结果，创建MergeRequesDiscussion，传入指定行信息，检查结果信息。
根据检查结果，创建MergeRequestComments，在MergeRequest中增加对应的comment信息



## 详细设计




### Handle
```go

func gitLabHandle(ctx context.Context, event *gitlab.MergeRequestEvent){
  
}
// https://gitlab.com/qtest8162535/ptest/-/hooks/43489248/hook_logs/8703291551
```
### ChangeFile
```go
func ListMergeRequstFiles()
//  https://docs.gitlab.com/ee/api/merge_requests.html#list-merge-request-diffs
```

### Report

```go

func CreateMergeRequestDiscussion(ctx context.Context, a.GitlLabClient, lintErrs map[string][]LinterOutput){
// https://docs.gitlab.com/ee/api/discussions.html#create-new-merge-request-thread	
}
func CreateMergeRequestComments(context.Background(), a.GitlLabClient,lintErrs map[string][]LinterOutput)){
//https://docs.gitlab.com/ee/api/notes.html#create-new-merge-request-note
}

```

###
