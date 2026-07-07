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
├── main.go              # 入口：初始化并运行两个演示工作流
├── go.mod
│
├── core/
│   └── types.go         # 核心接口与数据类型（Agent、Tool、Plan、Step 等）
│
├── llm/
│   ├── claude.go        # Claude API 客户端（Chat / ChatAgentLoop）
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
└── workflow/
    └── engine.go        # Workflow Engine：拓扑排序执行，依赖管理
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
- Anthropic API Key

### 安装与运行

```bash
# 克隆项目
git clone <repo-url>
cd agent-platform

# 设置 API Key
export ANTHROPIC_API_KEY=sk-ant-...

# 运行演示
go run main.go
```

### 演示效果

程序内置两个演示目标：

1. **计算并记忆**：`Calculate 123 multiplied by 456, then store the result in memory under key 'product'`
   - Planner 生成计划：`calculator` → `memory_write`
   - ExecutorAgent 执行计算，MemoryAgent 持久化结果

2. **RAG 检索**：`Search the knowledge base to learn about RAG and agents, then summarize what you found`
   - Planner 生成计划：`search_knowledge_base`（由 RAGAgent 执行）
   - RAGAgent 多轮检索后生成综合摘要

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
- **LLM**：[Anthropic Claude](https://www.anthropic.com)（默认模型：`claude-opus-4-8`）
- **SDK**：[anthropic-sdk-go](https://github.com/anthropics/anthropic-sdk-go) v1.0.0

## 环境变量

| 变量名 | 说明 | 必填 |
|--------|------|------|
| `ANTHROPIC_API_KEY` | Anthropic API 密钥 | 是 |
