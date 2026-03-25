# Relay

[English](./README.md) | 中文

### 项目简介

Relay 是一个面向长任务的软件开发 Agent Harness。

它不是一个“单次 prompt 调一下模型”的工具，而是一个把任务拆成可持续执行、可恢复、可观测、可验证流程的调度层。Relay 用 Go 实现，围绕 `pipeline -> issue -> workspace -> artifacts -> loop` 这一套模型，把真实代码仓库上的 AI 执行过程托管起来。

当前版本重点解决的是软件开发场景里的三个问题：

- 任务不是一次就能做完，需要多轮 planning / coding。
- 模型上下文会丢，执行过程需要显式持久化。
- 真正的完成判断不能只靠模型自述，而要靠结构化状态和外部验证。

### 这个产品在做什么

你可以把 Relay 理解成一个专门服务于编码 Agent 的“执行外壳”：

- `pipeline` 定义项目级执行方式，包括初始化仓库的方法、循环次数、planning prompt、coding prompt。
- `issue` 定义一次具体任务，包括目标、描述、所属 pipeline 和当前状态。
- `serve` 常驻轮询 `todo` 任务，自动创建 workspace、初始化仓库、启动 planning agent、再按 loop 驱动 coding agent。
- `feature_list.json` 作为任务完成的结构化真相源。
- `progress.txt` 作为每轮之间的交接日志。
- `runs/`、`events.log`、`issue.json` 作为问题排查和状态追踪依据。

Relay 当前默认通过本机 `codex` CLI 执行 agent。

### 设计来源

这个项目的产品思路主要受两篇文章启发：

- OpenAI: [Harness Engineering](https://openai.com/en/index/harness-engineering/)
- Anthropic: [Effective harnesses for long-running agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)

Relay 并不是对文章做简单复刻，而是把里面对 “agent harness” 的关键判断，落成一个可以直接运行在代码仓库上的产品模型。

对应到 Relay，大致体现为：

- Prompt 不是核心，harness 才是核心。模型本身只是一环，真正决定稳定性的，是任务分阶段、状态持久化、失败恢复、外部验证这些机制。
- 长任务必须有外部记忆。Relay 不依赖模型“记住上一次聊了什么”，而是把 `issue.json`、`feature_list.json`、`progress.txt`、`runs/` 都落到磁盘。
- 要把“完成”从自然语言里剥离出来。Relay 不以模型最后一句“我完成了”为准，而是重新读取 `feature_list.json`，只有全部 `passes: true` 才算 done。
- 每一轮 agent 都应该是 fresh run。Relay 的 planning 和每一轮 coding 都是新的执行，依赖持久化 artifact 恢复上下文，而不是绑定单个长 session。
- 真实环境比 benchmark 更重要。Relay 不是在虚构沙盒里跑 demo，而是在初始化出来的真实仓库里执行命令、改代码、提交 git、保留现场。

### 核心模型

#### Pipeline

项目级配置，持久化在：

```text
~/.relay/pipelines/<name>.yaml
```

字段：

- `name`
- `init_command`
- `loop_num`
- `plan_prompt`
- `coding_prompt`

#### Issue

单任务控制对象，持久化在：

```text
~/.relay/issues/<issue-id>/issue.json
```

主要字段：

- `id`
- `pipeline_name`
- `goal`
- `description`
- `status`
- `current_loop`
- `artifact_dir`
- `workspace_path`
- `workdir_path`
- `last_error`
- `interrupt_requested`

#### Workspace

每个 issue 独立的临时工作目录，默认位于：

```text
~/relay-workspaces/<issue-id>-<hash>/
```

这里运行 `init_command`，并把最终落点目录持久化为 `workdir_path`，供后续 agent 运行使用。

#### Artifacts

每个 issue 的持久化执行痕迹放在：

```text
~/.relay/issues/<issue-id>/
  issue.json
  feature_list.json
  progress.txt
  events.log
  runs/
```

其中：

- `feature_list.json` 是完成状态真相源。
- `progress.txt` 是交接日志，不参与完成判断。
- `events.log` 记录 orchestrator 级别事件。
- `runs/` 保存 planning / coding 每轮的 stdout、stderr、final message。

### 执行流程

`relay serve` 会轮询 issue 列表，找到 `todo` 任务后按下面的固定流程执行：

1. 创建 issue workspace。
2. 在 workspace 内运行 `init_command`。
3. 记录 `init_command` 结束时的工作目录，并写入 `workdir_path`。
4. 启动 planning agent。
5. planning agent 在 issue artifact 目录创建 `feature_list.json` 和 `progress.txt`。
6. orchestrator 校验 artifact 文件是否合法。
7. 进入 coding loop。
8. 每轮启动一个 fresh coding agent，在 repo 中改代码，并更新 issue artifact。
9. 每轮结束后重读 `feature_list.json`。
10. 如果全部 `passes: true`，则任务标记为 `done`；否则继续直到 `loop_num` 用尽。

### 为什么 `feature_list.json` 和 `progress.txt` 要分开

- `feature_list.json` 负责结构化判断“是否完成”。
- `progress.txt` 负责为下一轮 agent 提供上下文和交接信息。

这两者分离后，Relay 可以避免把“完成判断”建立在一大段自然语言总结上。

### 安装

对外最快的安装方式：

```bash
npm install -g @eddiearc/relay
```

前提：

- macOS 或 Linux
- 本机已安装 `codex`，并且在 `PATH` 里

如果你更偏向源码安装：

```bash
go install github.com/eddiearc/relay/cmd/relay@latest
```

### 当前能力边界

当前 Relay 主要聚焦于单仓库编码任务的托管：

- 支持 pipeline / issue 的增删改查。
- 支持 `serve` 持续轮询和 `serve --once` 单轮执行。
- 支持 issue interrupt。
- 支持 planning run + coding loop。
- 支持 issue 级 artifact 持久化。
- 支持真实 E2E 场景验证。

当前默认 runner 是：

- `codex` CLI 直连执行。

### 快速开始

#### 1. 准备一个 pipeline

```bash
relay pipeline add demo-pipeline \
  --init-command 'git clone https://example.com/repo.git app' \
  --loop-num 3 \
  --plan-prompt-file plan.md \
  --coding-prompt-file coding.md
```

也可以直接导入 YAML：

```bash
relay pipeline import -file pipeline.yaml
```

示例：

```yaml
name: demo-pipeline
init_command: git clone https://example.com/repo.git app
loop_num: 3
plan_prompt: |
  Understand the task and repo. Create feature_list.json and progress.txt.
coding_prompt: |
  Continue the task. Update feature_list.json and progress.txt based on actual progress.
```

#### 2. 创建一个 issue

```bash
relay issue add \
  --pipeline demo-pipeline \
  --goal "Implement the requested feature" \
  --description "Use the repository initialized by init_command."
```

也可以导入 JSON：

```bash
relay issue import -file issue.json
```

#### 3. 启动 orchestrator

```bash
relay serve
```

或者只跑当前待执行队列一次：

```bash
relay serve --once
```

### 常用命令

- `relay pipeline add <name> --init-command ... --plan-prompt-file ... --coding-prompt-file ...`
- `relay pipeline edit <name> [--init-command ...] [--loop-num ...] [--plan-prompt-file ...] [--coding-prompt-file ...]`
- `relay pipeline import -file pipeline.yaml`
- `relay pipeline list`
- `relay pipeline delete <name>`
- `relay issue add --pipeline ... --goal ... --description ...`
- `relay issue edit --id ... [--pipeline ...] [--goal ...] [--description ...]`
- `relay issue import -file issue.json`
- `relay issue list`
- `relay issue interrupt --id ...`
- `relay issue delete --id ...`
- `relay serve [--workspace-root /path/to/workspaces]`
- `relay serve --once`
- `relay status -issue <issue-id>`
- `relay report -issue <issue-id>`
- `relay kill -issue <issue-id>`
- `relay version`

### 构建与发版

当前项目本地快速打包优先走 Makefile：

```bash
make build
```

查看二进制里写入的版本信息：

```bash
./bin/relay version
```

给当前平台生成一个本地发布压缩包：

```bash
make package
```

一次性打出和 CI 一样的全部发布包：

```bash
make package-all
```

基于这些 release 压缩包生成 npm 包：

```bash
npm --prefix npm run prepare-release -- \
  --version v0.1.0 \
  --dist-dir "$PWD/dist" \
  --out-dir "$PWD/npm/out"
```

发布生成出来的 npm 包：

```bash
npm --prefix npm run publish-release -- \
  --version v0.1.0 \
  --packages-dir "$PWD/npm/out"
```

版本号本身不放在源码常量里硬编码，默认由 CI 读取 git tag 注入。仓库里的 GitHub Actions 会：

- 直接复用本地 `Makefile`，保证本地和 CI 的打包逻辑一致
- 只在 GitHub Release 发布时触发，避免日常 push 频繁触发
- 先执行 `make test`
- 再执行 `make package-all VERSION=<release-tag>`
- 最后把 `linux/amd64`、`linux/arm64`、`darwin/amd64`、`darwin/arm64` 四个压缩包上传到这个 release
- 再根据这些压缩包生成 npm 包
- 先发布四个平台包，再发布 `@eddiearc/relay`

如果你想在本地手动指定版本号，也可以这样打包：

```bash
make package VERSION=v0.1.0
```

如果你想在 GitHub 上发起正式打包，直接发布一个 release 即可，比如：

```bash
gh release create v0.1.0 --generate-notes
```

npm 包结构和首次接入 npm registry 的细节见 [`npm/README.md`](./npm/README.md)。

当前还不建议发 Windows 包，因为运行时里有 `zsh` 和 `SIGKILL` 这样的 Unix 假设。

### 项目范围

Relay 当前是一个面向真实代码仓库和真实编码任务的聚焦型 harness。

它的目标不是做一个通用的 AI 聊天壳，而是提供一个围绕 planning、coding、持久化和验证展开的耐久执行模型。
