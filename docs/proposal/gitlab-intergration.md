## Proposal: GitLab集成
## Background

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



## 集成方式调研与选择
基于与gitlab无缝衔接的原则调研了有两种与gitllab集成的方式，
第一种：
通过在待检查项目中，添加pipeline，在pippline中的 job 中，写入 reviewbot 检查结果，这样既能动态展示当前代码检查运行的状态及进度，又能展示检查结果
第二种：
在gitllab中的comment 展示代码检查结果，在discussion精确展示代码行信息


| 方式	     | 优点     | 缺点                             |技术难度     |
| -------- | -------- |--------------------------------|-------- |
| 方案一：Gitlab job | 能动态展示当前代码检查的运行状态，运行日志。 | 需要在每一个代码仓库中添加.gitlab-ci .YML文件 | job中日志的更新，状态的更新，需要调用gitlab未公开的内部API,并没有相关文档，需要从gitlab-runner 源码中获取内部api的使用方法 |
| 方案二：Comment/Disscuion | 精确到代码行展示代码检查结果	低版本不支持discussion，不能实时显示运行状态 | 不能实时显示运行状态                         |接调用gitlab公开的API就能实现，有详细的使用文档，在gitlab的低版本中，不支持discussion，需要对不同的版本进行区分 |

在方案一由于gitlab 需要再每个代码仓库中手动配.gitlab-ci.YML，不方便用户使用，不利于推广使用，故最终确定为通过 方案二的方式进行gitlab与reviewbot集成

## Gitlab 版本兼容
因为gitlbab 支持本地化部署，不可避免存在多个版本使用的情况，为了兼容gitlbab 中的可能存在的多版本兼容性问题。
我们做了如下设计。
1.通过API 获取当前系统版本，获取配置文件中，reportfomart（如无配置，根据系统自动确定结果展示方式。）。
2.如果系统版本 》=10.08 版本，则使用 comment+discussion的方式展示代码检查结果。
如果gitlab版本《10.08版本，使用commet方式展示代码检查结果。
 
1.如果存在配置，根据当前系统的类型（gitlab，github….）及系统的版本，对配置进行校验，如果配置，通过校验，则按照配置方式进行结果展示，如果配置校验不通过，提示配置异常。根据系统自动确定结果展示方式。

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
### Agent
```go
type Agent struct {

	GitLabClient             *gitlab.Client
	MergeRequestEvent        *gitlab.MergeEvent
	MergeRequestChangedFiles []*gitlab.MergeRequestDiff
	Report       report.Report
}
```
### Report
```go
type Report interface {

Report(log *xlog.Logger, a Agent, lintResults map[string][]LinterOutput)   error
}

func (l *gitlabreport) Report(log *xlog.Logger, a Agent, lintResults map[string][]LinterOutput)

func (l *githubreport) Report(log *xlog.Logger, a Agent, lintResults map[string][]LinterOutput)
```

###









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