# agent-platform

面向复杂任务场景设计的 Agent 开发平台，基于 Go 构建 Workflow 编排引擎，支持 Planner、Executor、Memory、RAG 等多类 Agent 协作；统一接入 Claude 大模型及知识库能力，实现任务自动规划、工具调用与结果生成闭环。

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
│  memory_read │ memory_write          │
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
│   ├── memory_agent.go  # MemoryAgent：读写持久化内存
│   └── rag.go           # RAGAgent：检索增强生成，多轮工具调用循环
│
├── tools/
│   ├── registry.go      # Tool Registry：注册、查找、执行工具
│   ├── calculator.go    # 四则运算工具
│   ├── search.go        # 模拟知识库检索工具
│   └── memory_tool.go   # 内存读写工具（memory_read / memory_write）
│
├── memory/
│   └── memory.go        # 线程安全的 KV 内存存储（支持 Tag 检索）
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
2. 使用拓扑排序逐步执行，确保依赖步骤先完成
3. 所有步骤共享 `WorkflowContext`（包含历史结果和内存快照）
4. 检测循环依赖，遇到死锁时返回错误

### 工具（Tools）

| 工具名 | 描述 |
|--------|------|
| `calculator` | 四则运算（add / subtract / multiply / divide） |
| `search_knowledge_base` | 模拟知识库检索，内置 go、agent、rag、mcp、llm 等词条 |
| `memory_read` | 按 key 读取内存，key 为空时返回全部内存摘要 |
| `memory_write` | 将 key-value 写入持久化内存 |

## 快速开始

### 前置条件

- Go 1.21+
- Anthropic API Key **或** DeepSeek API Key

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
```

### 演示效果

程序内置两个演示目标：

1. **计算并记忆**：`Calculate 123 multiplied by 456, then store the result in memory under key 'product'`
   - Planner 生成计划：`calculator` → `memory_write`
   - ExecutorAgent 执行计算，MemoryAgent 持久化结果

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

- **语言**：Go 1.21
- **LLM**：[Anthropic Claude](https://www.anthropic.com)（默认模型：`claude-opus-4-8`）与 [DeepSeek](https://www.deepseek.com)（OpenAI 兼容）
- **SDK**：[anthropic-sdk-go](https://github.com/anthropics/anthropic-sdk-go) v1.0.0、[go-openai](https://github.com/sashabaranov/go-openai) v1.41.2
- **可视化**：Go `net/http` + Server-Sent Events（SSE），零外部前端依赖
- **规格管理**：[OpenSpec](https://github.com/Fission-AI/OpenSpec) v1.5.0

## 环境变量

| 变量名 | 说明 | 必填 |
|--------|------|------|
| `ANTHROPIC_API_KEY` | Anthropic API 密钥 | 二选一 |
| `DEEPSEEK_API_KEY` | DeepSeek API 密钥（当未设置 `ANTHROPIC_API_KEY` 时使用） | 二选一 |

> 两者都设置时优先使用 `ANTHROPIC_API_KEY`。
