## Proposal: 自定义执行节点

## Background

> see [issue](https://github.com/qiniu/reviewbot/issues/215)

随着引入的 linter 越来越多，执行环境需要安装的依赖也越来越多, 此时环境本身的维护成本会越来越高。且如果执行环境是 docker 或者 k8s，image 的 size 也会越来越大, 这必然不利于分发和维护。

所以本提案的目标是，提供一种机制，让用户可以自定义执行节点，从而避免上述问题。

## 设计原则

从简化管理和使用的角度考虑:

- 不推荐太多节点，只有在必要时才需要拆分节点。比如不同的 org 有不同的业务类型，需要安装不同的基础组件；单个 image 的 size 已超 1G 等等
- 不假设运行环境是 docker 或者 k8s，节点可以是任意类型，只要能运行 Reviewbot 的二进制文件即可
- Reviewbot 本身暂时不负责节点的启、停
- 节点的选择是基于 repo/linter 粒度的
  > 当然，如果后续有需要，可以做更灵活的配置，比如基于 repo 粒度，基于 org 粒度，或者基于 linter 粒度

## 整体设计

不破坏现有部署架构，请求路径仍然是 webhook -> reviewbot 实例. 由 Reviewbot 实例来决策是自己执行还是转发给其他实例执行。

> 方便起见，Reviewbot 实例下面会简称为节点

为支持节点选择，引入一个节点管理服务，用于管理节点。节点从节点管理服务获取到全局的节点列表。

- 不通过数据库来保存节点信息，是因为节点会有启停变化，需要做心跳保活，并即时感知。

- 当然也可以用 consul 来实现，但需要做一定的适配，这个后面再考虑。当前认为引入 consul 会增加项目理解复杂度，所以考虑用一个简单小服务来实现

节点管理服务需要支持节点注册、保活、心跳上报、节点列表获取等操作。节点管理服务设计为单实例服务，无状态，方便部署和维护。

节点在启动阶段，会向节点管理服务注册，并定时上报心跳。类似:

```bash
./reviewbot -discovery.http-addr=http://127.0.0.1:8080/hook -discovery.ws-addr=ws://127.0.0.1:8081 -discovery.node-labels=node1,node2 ...
```

Node Labels 是节点的标签，由部署节点时指定，可以有多个，用于区分不同的节点。其值是自定义的。

Label 机制的实现将会参考 https://onsi.github.io/ginkgo/#spec-labels.

节点管理服务使用 websocket 与节点保持连接，并维护一个全局的节点列表，并定时更新。

节点从节点管理服务获取到全局的节点列表，当接受到 Webhook 事件时，会根据 Webhook 的 repo/linter 信息，从全局的节点列表中选择一个节点来执行。

节点的选择逻辑如下:

- 如果 PR 的 repo/linter 信息在配置中配置了 node_labels，那么将选择匹配该规则的节点
  - 节点知道自己的 Labels，如果当前节点匹配了 node_labels，则优先在当前节点继续执行
  - 如果当前节点不匹配，则通过负载均衡策略从匹配的节点中选择一个节点
  - 如果选择的节点不可用，那么会重试，继续选择一个可用的节点
- 如果 PR 的 repo/linter 没有配置强制指定 node_labels，那优先选择没有 Label 的节点执行，如果都没有，则继续在当前节点执行

节点会缓存全局的节点列表，当节点管理服务有更新时，会全量更新缓存的节点列表。当节点管理服务不可用时，缓存的节点列表仍然有效。而当节点管理服务重新可用时，会自动重新建立连接。

考虑在节点管理服务启动的初始阶段，从节点管理服务获得的节点列表可能少于实际的节点数量，节点侧会只有在从节点管理服务获得的节点数量大于等于当前缓存的节点数量时，才会更新缓存的节点列表。这个逻辑只会在节点管理服务启动的初始阶段生效，当节点管理服务可用时间超过 2 分钟后，会强制更新缓存的节点列表。

## 详细设计

### 节点管理服务结构

```go
type Node struct {
    HTTPAddress string
    WSConn     *websocket.Conn
    Tags       []string
    LastPing   time.Time
}

type NodeManager struct {
    // key 是 node 的 HTTPAddress,保证唯一性
    nodes map[string]*Node
    mu sync.Mutex
}

// 返回给节点的信息
type NodeInfo struct {
    Name string `json:"name"`
    Tags []string `json:"tags"`
    HTTPAddress string `json:"http_address"`
}
```

###
