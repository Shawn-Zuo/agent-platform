# agent-platform

面向复杂任务场景设计的 Agent 开发平台，基于 Go 构建 Workflow 编排引擎，支持 Planner、Executor、Memory、RAG 等多类 Agent 协作；统一接入 Claude/DeepSeek、大模型工具调用及知识库能力，实现任务自动规划、工具调用与结果生成闭环。项目同时提供 MCP Server 和 MCP Client：既能把内置工具暴露给外部 AI 应用，也能发现并调用其他 MCP Server 提供的工具。

## 架构概览

```
用户目标 (Goal)
    │
    ▼
┌─────────────────────────────────────┐
│         Workflow Engine              │
│  ┌──────────┐                        │
│  │ Planner  │  将目标分解为有序步骤     │
│  └──────────┘                        │
│       │  Plan (Steps + 依赖关系)      │
│       ▼                              │
│  ┌────────────────────────────────┐  │
│  │    拓扑排序执行器                │  │
│  │  ┌──────────┐ ┌─────────────┐ │  │
│  │  │ Executor │ │MemoryAgent  │ │  │
│  │  │  Agent   │ │  Agent      │ │  │
│  │  └──────────┘ └─────────────┘ │  │
│  │  ┌──────────┐                 │  │
│  │  │   RAG    │                 │  │
│  │  │  Agent   │                 │  │
│  │  └──────────┘                 │  │
│  └────────────────────────────────┘  │
└─────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│           Tool Registry              │
│  calculator │ search_knowledge_base  │
│  memory_read/write/search/delete     │
└─────────────────────────────────────┘
         ▲                    │
         │                    ▼
┌─────────────────┐  ┌─────────────────────┐
│ MCP Client      │  │ MCP Server (stdio)  │
│ remote discovery│  │ tools/list/call     │
└─────────────────┘  └─────────────────────┘
         ▲
         │ stdio
┌─────────────────────────────────────┐
│       External MCP Servers           │
└─────────────────────────────────────┘
```

## 目录结构

```
agent-platform/
├── main.go              # 入口：命令行演示 或 -web 启动可视化服务
├── go.mod
│
├── core/
│   └── types.go         # 核心接口与数据类型（Agent、Tool、Plan、Step 等）
│
├── llm/
│   ├── claude.go        # Claude API 客户端（Chat / ChatAgentLoop）
│   ├── deepseek.go      # DeepSeek 客户端（OpenAI 兼容格式）
│   └── helpers.go       # 消息构建工具函数
│
├── agents/
│   ├── planner.go       # PlannerAgent：调用 LLM 将目标分解为 JSON 计划
│   ├── executor.go      # ExecutorAgent：按步骤调用工具注册表
│   ├── memory_agent.go  # MemoryAgent：读写短期上下文与共享记忆
│   └── rag.go           # RAGAgent：检索增强生成，多轮工具调用循环
│
├── tools/
│   ├── registry.go      # Tool Registry：注册、查找、执行工具
│   ├── calculator.go    # 四则运算工具
│   ├── search.go        # 模拟知识库检索工具
│   └── memory_tool.go   # 记忆读写、检索、删除工具
│
├── memory/
│   └── memory.go        # 命名空间、TTL、检索与可选 JSON 持久化
├── mcpserver/
│   ├── server.go        # MCP Server：将 Tool Registry 暴露为 MCP Tools
│   └── server_test.go   # MCP 工具发现、调用与共享内存测试
├── mcpclient/
│   ├── config.go        # 外部 MCP Server 配置与校验
│   ├── client.go        # stdio 连接、工具发现与会话生命周期
│   ├── tool.go          # 将远程 MCP Tool 适配为 core.Tool
│   └── *_test.go        # 配置、子进程连接、调用与命名测试
│
├── workflow/
│   └── engine.go        # Workflow Engine：拓扑排序执行，依赖管理，事件钩子
│
├── web/
│   ├── server.go        # Web 可视化服务：HTTP + SSE 实时事件流
│   └── index.go         # 单页可视化前端（内嵌 HTML/CSS/JS，零外部依赖）
│
└── openspec/            # OpenSpec 规格驱动开发（specs / changes）
```

## 核心概念

### Agent 类型

| 类型 | 实现 | 职责 |
|------|------|------|
| `planner` | `PlannerAgent` | 接收用户目标，调用 Claude 生成结构化 JSON 执行计划，包含步骤依赖关系 |
| `executor` | `ExecutorAgent` | 执行单个工具调用步骤，支持 `$step_N.output` 占位符引用前序结果 |
| `memory` | `MemoryAgent` | 专用于内存读写操作，将结果同步到 `WorkflowContext.Memory` |
| `rag` | `RAGAgent` | 通过多轮 LLM + 工具调用循环检索知识库，生成综合回答（最多 5 轮） |

### Workflow Engine

`workflow.Engine` 是核心编排组件：

1. 调用 `PlannerAgent` 将目标分解为带依赖关系的步骤列表
2. 执行前静态校验计划，拦截重复步骤、未知工具、非法 Agent、无效依赖与依赖环，避免部分执行
3. 使用拓扑排序逐步执行，确保依赖步骤先完成
4. 所有步骤共享本次运行的 `WorkflowContext`（包含步骤结果和工作流内存）

### 工具（Tools）

| 工具名 | 描述 |
|--------|------|
| `calculator` | 四则运算（add / subtract / multiply / divide） |
| `search_knowledge_base` | 模拟知识库检索，内置 go、agent、rag、mcp、llm 等词条 |
| `memory_read` | 在指定 namespace 中按 key 读取记忆，key 为空时返回摘要 |
| `memory_write` | 写入带 namespace、Tags 和可选 TTL 的共享记忆 |
| `memory_search` | 按关键词检索 key、value 与 Tags，结果按相关性和更新时间排序 |
| `memory_delete` | 删除指定 namespace 中的一条记忆 |

Memory 分为两层：单次执行中的 `WorkflowContext.Memory` 保存短期步骤上下文；共享 `memory.Store` 使用 namespace 隔离用户/会话，支持 TTL、Tags 和关键词召回。默认仍为纯内存模式；传入 `-memory-file` 后，写入和删除会通过临时文件 + 原子替换同步到 JSON，进程重启后自动恢复未过期记忆。

## 快速开始

### 前置条件

- Go 1.24+
- Workflow/网页模式需要 Anthropic API Key **或** DeepSeek API Key；MCP 模式不需要

### 安装与运行

```bash
# 克隆项目
git clone <repo-url>
cd agent-platform

# 设置 API Key（二选一，优先使用 ANTHROPIC_API_KEY）
export ANTHROPIC_API_KEY=sk-ant-...
# 或
export DEEPSEEK_API_KEY=sk-...

# 命令行模式：运行内置演示
go run .

# Web 可视化模式：启动服务后浏览器访问 http://localhost:8080
go run . -web :8080

# 启用跨进程持久化记忆（CLI / Web / MCP 模式通用）
go run . -memory-file ./data/memory.json
```

如果只启动 MCP Server，不需要配置任何 LLM API Key：

```bash
go run . -mcp
```

### 演示效果

程序内置两个演示目标：

1. **计算并记忆**：`Calculate 123 multiplied by 456, then store the result in memory under key 'product'`
   - Planner 生成计划：`calculator` → `memory_write`
   - ExecutorAgent 执行计算，MemoryAgent 将结果保存到进程内共享 Store

2. **RAG 检索**：`Search the knowledge base to learn about RAG and agents, then summarize what you found`
   - Planner 生成计划：`search_knowledge_base`（由 RAGAgent 执行）
   - RAGAgent 多轮检索后生成综合摘要

## Web 可视化

通过 `-web` 参数启动一个内置的可视化服务，在浏览器中实时观察工作流的执行过程：

```bash
go run . -web :8080   # 访问 http://localhost:8080
```

功能：

- **输入目标**：手动输入 Goal，或点击预设示例
- **DAG 步骤图**：实时渲染 Planner 生成的执行计划，每个步骤展示 ID、Agent 类型（彩色标签）、工具名、输入参数与依赖关系
- **实时状态**：步骤状态随执行流式更新——等待 → 运行中（脉冲）→ 完成 / 错误，并展开显示每步输出
- **侧边栏**：已注册工具列表 + Memory Store 实时内容

技术实现：Go `net/http` 提供 HTTP 服务，通过 **Server-Sent Events (SSE)** 将 `workflow.Engine` 的生命周期事件（`plan` / `step_start` / `step_done` / `workflow_done` / `error`）推送到前端，前端为单页应用，无任何外部依赖。

> 引擎侧通过 `Engine.OnEvent` 事件钩子暴露执行事件，命令行模式不受影响。

## MCP Server

[Model Context Protocol（MCP）](https://modelcontextprotocol.io/) 是 AI 应用连接外部工具和上下文的标准协议。本项目实现 MCP Server，通过 `stdio` 传输暴露 Tool Registry 中默认的六个工具；如果同时指定 `-mcp-config`，也会暴露发现到的远程工具：

| MCP Tool | 输入 |
|----------|------|
| `calculator` | `operation`、`a`、`b` |
| `search_knowledge_base` | `query` |
| `memory_read` | `key`、可选 `namespace`，key 为空时列出该 namespace |
| `memory_write` | `key`、`value`、可选 `namespace` / `tags` / `ttl_seconds` |
| `memory_search` | `query`、可选 `namespace` / `limit` |
| `memory_delete` | `key`、可选 `namespace` |

MCP 客户端可以通过标准的 `tools/list` 发现工具，通过 `tools/call` 调用工具。MCP 适配层直接复用项目的 `core.Tool` 和 `tools.Registry`，因此工作流模式与 MCP 模式的工具行为保持一致。

### 启动 MCP Server

建议先编译一个固定路径的可执行文件：

```bash
go build -o bin/agent-platform .
```

然后在支持 MCP 的客户端中添加配置，将 `command` 替换为实际的绝对路径：

```json
{
  "mcpServers": {
    "agent-platform": {
      "command": "/absolute/path/to/agent-platform/bin/agent-platform",
      "args": ["-mcp", "-memory-file", "/absolute/path/to/memory.json"]
    }
  }
}
```

`-mcp` 模式使用 stdin/stdout 传输 MCP JSON-RPC 消息，所以不要将普通日志写入 stdout；服务错误会写入 stderr。该模式不启动 Workflow/LLM，也不需要 `ANTHROPIC_API_KEY` 或 `DEEPSEEK_API_KEY`。

### MCP Client：调用外部工具

创建一个配置文件，例如 `mcp-servers.json`：

```json
{
  "mcpServers": {
    "demo": {
      "command": "/absolute/path/to/external-mcp-server",
      "args": ["--stdio"],
      "env": {
        "SERVICE_API_KEY": "replace-me"
      }
    }
  }
}
```

然后启动命令行或网页模式：

```bash
# CLI Workflow 使用本地工具和远程 MCP 工具
go run . -mcp-config ./mcp-servers.json -goal "Use the demo MCP server to echo hello"

# Web Workflow 使用本地工具和远程 MCP 工具
go run . -web :8080 -mcp-config ./mcp-servers.json
```

启动时 MCP Client 会依次完成：

1. 启动配置中的 stdio MCP Server 子进程并初始化连接。
2. 通过 `tools/list` 获取全部远程工具及其 JSON Schema。
3. 将工具注册为 `服务器名__工具名`，例如 `demo__echo`，避免覆盖本地工具。
4. Planner 从 Registry 动态生成工具清单，Executor/RAG 通过统一接口调用远程工具。
5. 主程序退出时关闭 MCP 会话和子进程。

也可以同时使用 MCP Client 和 MCP Server，将外部工具经过本项目重新暴露：

```bash
go run . -mcp -mcp-config ./mcp-servers.json
```

连接或工具发现失败会阻止程序启动，避免在工具集合不完整的状态下执行任务。不要把包含真实密钥的 MCP 配置提交到版本库；`env` 中的值会覆盖子进程继承的同名环境变量。

当前 MCP Client 支持 stdio Tool Discovery/Tool Call，暂未支持 Streamable HTTP、MCP Resources、Prompts，以及运行期间的工具列表热更新。

## OpenSpec（规格驱动开发）

项目集成了 [OpenSpec](https://github.com/Fission-AI/OpenSpec) v1.5.0，用于以「规格驱动」的方式管理开发：先写提案与规格，再落地实现。

```
openspec/
├── config.yaml          # schema、项目 context、artifact 规则
├── specs/               # 主规格库（已定型的能力规格）
└── changes/             # 变更提案（含 archive/ 归档目录）
```

每个变更（change）包含若干工件（artifact）：`proposal.md`（做什么/为什么）、`design.md`（怎么做）、`tasks.md`（实现步骤）、以及增量规格（delta specs）。

在 Claude Code 中通过斜杠命令驱动完整流程：

| 命令 | 作用 |
|------|------|
| `/opsx:explore <想法>` | 思考模式，讨论与澄清需求（只思考不写代码） |
| `/opsx:propose <名称/描述>` | 创建变更，一步生成 proposal / design / tasks |
| `/opsx:apply [名称]` | 按 tasks.md 逐条实现 |
| `/opsx:sync [名称]` | 将增量规格智能合并进主规格 |
| `/opsx:archive [名称]` | 归档完成的变更并更新主规格 |

也可直接使用 CLI：`openspec list`、`openspec view`、`openspec new change "<name>"`、`openspec status --change "<name>"`、`openspec validate`、`openspec archive <name>`。

## 扩展指南

### 添加新工具

实现 `core.Tool` 接口并注册：

```go
type MyTool struct{}

func (t *MyTool) Name() string        { return "my_tool" }
func (t *MyTool) Description() string { return "..." }
func (t *MyTool) InputSchema() map[string]any { return map[string]any{...} }
func (t *MyTool) Execute(ctx context.Context, input map[string]any) (string, error) { ... }

// 在 workflow/engine.go 的 NewEngine 中注册：
registry.Register(&MyTool{})
```

### 添加新 Agent 类型

实现 `core.Agent` 接口，并在 `workflow/engine.go` 的 `runStep` 中添加路由分支：

```go
type MyAgent struct{ ... }

func (a *MyAgent) Name() string            { return "MyAgent" }
func (a *MyAgent) Type() core.AgentType    { return core.AgentType("my_type") }
func (a *MyAgent) Run(ctx context.Context, wfCtx *core.WorkflowContext, step core.Step) (core.StepResult, error) { ... }
```

## 技术栈

- **语言**：Go 1.24+
- **LLM**：[Anthropic Claude](https://www.anthropic.com)（默认模型：`claude-opus-4-8`）与 [DeepSeek](https://www.deepseek.com)（OpenAI 兼容）
- **SDK**：[anthropic-sdk-go](https://github.com/anthropics/anthropic-sdk-go) v1.0.0、[go-openai](https://github.com/sashabaranov/go-openai) v1.41.2
- **MCP**：[Model Context Protocol Go SDK](https://github.com/modelcontextprotocol/go-sdk)，stdio Server + Client
- **可视化**：Go `net/http` + Server-Sent Events（SSE），零外部前端依赖
- **规格管理**：[OpenSpec](https://github.com/Fission-AI/OpenSpec) v1.5.0

## 环境变量

| 变量名 | 说明 | 必填 |
|--------|------|------|
| `ANTHROPIC_API_KEY` | Anthropic API 密钥 | 二选一 |
| `DEEPSEEK_API_KEY` | DeepSeek API 密钥（当未设置 `ANTHROPIC_API_KEY` 时使用） | 二选一 |

> 两者都设置时优先使用 `ANTHROPIC_API_KEY`。
