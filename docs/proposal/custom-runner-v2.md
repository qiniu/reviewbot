## Proposal: 自定义执行节点

## 背景

在 [提案一](custom-runner.md) 中，提出了给节点打标签的方案。但综合评估发现，这个方案实际上对用户的使用要求较高。用户需要理解各节点的定位，以及在相应的 repo/linter 配置上选择适合的 Label。而在打标签和选择标签的实施过程，实际上可能涉及到两个角色的交互，一是 部署 reviewbot 的人，二是维护 repo/linter 配置的人，无疑增加了交互的复杂度。

而在技术实现上，也需要引入额外的节点管理服务，以及节点选择算法，复杂度也是有所增加。

## 应对

在方案一中，我们提到了一个原则：

> 不假设运行环境是 docker 或者 k8s，节点可以是任意类型，只要能运行 Reviewbot 的二进制文件即可

但考虑到自定义节点应该是少数情况，且自定义节点通常需要自定义运行环境，那把这种自定义环境做成 docker image，然后通过 docker run 来运行，应该是一个比较常见的做法。而这种方式也能满足我们的原始需求，且在技术实现上，也能简化不少。

## 具体设计

在 linter 结构体中，增加一个字段，用于指定运行时的 docker image。

```go
type Linter struct {
	// ...
	// DockerAsRunner is the docker image to run the linter.
	// Optional, if not empty, use the docker image to run the linter.
	// e.g. "golang:1.23.4"
	DockerAsRunner string `json:"dockerAsRunner,omitempty"`
}
```

在执行 linter 时，如果 linter 的 DockerImage 字段有值，则使用该字段指定的 docker image 来运行 linter。该 linter 涉及到运行环境的配置，也会同步生效。克隆的仓库代码，也会挂载到 docker image 的 工作目录下，方便 linter 进行处理。

处理完成后，linter 的输出，会写入到克隆的仓库代码的根目录下，方便 reviewbot 进行处理。后续流程不变。

## 劣势

该方案的优点是，能够支持任意类型的自定义运行环境，且在技术实现上，也能简化不少。用户在使用时，只需要关注单个 linter 的配置，而不需要关注节点的选择。

但是，可能也有一些问题，比如：

- 环境依赖：

  - 如果要支持自定义 linter 的运行环境，那 reviewbot 的运行环境，也需要支持 docker。
  - reviewbot 的运行环境，需要拉取和保持 docker image，会使 reviewbot 占用的存储空间增加。

- 运行时资源消耗：

  - 多个 Docker 容器同时运行可能会消耗大量系统资源，特别是在处理大型项目或多个 PR 时。
  - 如果 Docker 镜像在执行时需要从网络上拉取，这增加了对网络的依赖，可能影响可靠性。
