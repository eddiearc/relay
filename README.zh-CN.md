# Relay

[English](./README.md) | 中文

### Relay 是什么

Relay 是一个面向长任务软件工程场景的 agent-first、CLI-first harness framework。

它不是一个“单次 prompt 调一下模型”的工具，而是一个围绕执行结构设计出来的调度层：持久化任务状态、初始化仓库、先 planning 再 coding、按 loop 启动 fresh agent，并且用显式校验来判断完成。

Relay 适用于那些需要不止一轮模型交互、也不止一个上下文窗口的真实编码任务。

最快的理解方式：

- agent-first：首选用法是让 agent 按正确流程来操作 Relay
- CLI-first：系统通过明确的命令和持久化状态来驱动
- harness framework：Relay 为编码 agent 提供 orchestration、memory、verification 和 recovery

### 快速开始

优先路径是：先通过 `npx skills` 全局安装 skill，再让支持 skills 的 agent 通过这个 skill 使用 Relay。

#### 1. 通过 skill 快速开始

先全局安装 Relay CLI，再全局安装 `relay-operator` skill：

```bash
npm install -g @eddiearc/relay && \
npx skills add https://github.com/eddiearc/relay --skill relay-operator -g -y
```

这样 skill 会对你的各个 agent / 仓库复用。`relay-operator` 是默认的自包含安装入口。如果你明确希望安装到当前项目而不是全局目录，就去掉 `-g`。

可以把 `relay-operator` 理解成当前版本下比较好的最佳实践路径，而不是一套冻结不变、唯一正确的流程。目标是让社区一起把默认路径打磨得更清晰。如果你摸索出比当前更好的使用路径，欢迎直接到 GitHub 提 PR，优化这个 skill 和对应的 CLI 指引。

然后给任意支持已安装 skills 的 agent 这样一句指令：

```text
Use the installed relay-operator skill to set up Relay for <repository-path>.
Start by running relay help and relay upgrade --check, and summarize whether Relay or the relay-operator skill should be refreshed.
Then inspect the repository, choose or write a repository-specific pipeline, summarize its planning focus, coding focus, verification path, reusable project assets, and any missing E2E or unit-test coverage in a few concise bullets, ask whether that direction sounds right, then rewrite the task as a Relay issue with explicit acceptance criteria, call out any weak goal, scope, non-goals, or verification details that still need correction, and tell me whether to run relay serve --once or relay serve persistently.
```

第一次让 agent 接手之前，先运行：

```bash
relay help
relay upgrade --check
```

这一步会把开场自检统一成一条命令，直接告诉你：

- 当前 Relay 版本和安装方式
- 是否有更新版本可升级
- 规范的命令地图和工作流
- `relay-operator` skill 刷新命令

具体怎么操作 Relay，以 CLI help 为准，优先看：

```bash
relay help
relay help pipeline
relay help issue
relay help serve
```

这个 skill 会引导 agent 去：

- 先执行 `relay help` 和 `relay upgrade --check` 开场检查
- 用 `relay help ...` 作为具体操作的真相源
- 检查 `relay` 是否已安装
- 阅读目标仓库
- 用 `relay pipeline list` 和 `relay pipeline show` 检查已有 pipeline
- 如果某条 pipeline 明显匹配就直接选；如果有多个候选就让用户确认；如果没有合适的就从 `relay pipeline template` 开始创建 repository-specific pipeline
- 在真正采用某条 pipeline 之前，先用几条简练 bullet 概括它的 planning 重点、coding 重点、验证路径、项目里可复用的资产，以及 E2E / 单测覆盖是否缺失
- 如果 issue 的目标、scope、非目标、验证方式写得不清楚，要直接指出问题，而不是默默替用户脑补
- 在真正创建 issue 前，要结合 pipeline 配置和 prompt 意图解释关键方向性决策，再让用户确认这个方向对不对
- 把任务改写成带明确验收条件的 Relay issue
- 如果目标结果、scope / 非目标、验证方式仍然不清楚，就一次性向用户提几个聚焦问题
- 决定应该执行 `relay serve --once` 还是常驻 `relay serve`
- 出问题时检查 artifact 和宿主机日志

#### 2. 直接使用 CLI

如果你想直接使用 Relay，先安装它：

```bash
npm install -g @eddiearc/relay
```

然后创建一个 pipeline：

```bash
relay pipeline add demo-pipeline \
  --init-command 'git clone https://example.com/repo.git app' \
  --plan-prompt-file plan.md \
  --coding-prompt-file coding.md
```

如果省略 `--loop-num`，Relay 会默认使用 `20`。

也可以直接导入 YAML：

```bash
relay pipeline import -file pipeline.yaml
```

创建一个 issue：

```bash
relay issue add \
  --pipeline demo-pipeline \
  --goal "Implement the requested feature" \
  --description "Use the repository initialized by init_command."
```

启动 orchestrator：

```bash
relay serve
```

或者只跑当前待执行队列一次：

```bash
relay serve --once
```

### 相关文章

Relay 的产品思路主要受这两篇文章启发：

- OpenAI: [Harness Engineering](https://openai.com/en/index/harness-engineering/)
- Anthropic: [Effective harnesses for long-running agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)
- Anthropic: [Demystifying evals for AI agents](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents)
- Anthropic: [Building agents with the Claude Agent SDK](https://www.anthropic.com/engineering/building-agents-with-the-claude-agent-sdk/)

### Verification Doctrine

在 Relay 里，verification 不是可选补丁，而是 harness 契约的一部分。

- OpenAI 的 harness engineering 文章强调，真正让 agent 稳定工作的关键是环境设计和 feedback loops，而不只是 prompt 文本本身。
- Anthropic 的 eval 文章强调，真正该看的不是 agent 最后说了什么，而是环境里的最终状态；eval harness 应该把任务端到端跑完，再对结果打分。
- Anthropic 的 agent SDK 文章也明确提到，对前端类工作要把浏览器自动化接进 workflow，让 agent 能直接检查截图、视口和交互元素。

基于这些文章，Relay 进一步抽出了一套面向项目工作的操作政策：

- 对于有实际行为变化的任务，agent 应默认选择“现实里最强、最贴近项目级”的验证路径
- 如果某个任务不值得走更重的验证链路，也可以走更窄的验证，但要明确告诉用户这是例外，并说明原因
- 前端项目通常应优先有浏览器 E2E，能够模拟点击等真实交互并验证关键用户路径
- 后端项目通常应优先有本地启动或部署路径，以及针对运行中服务的集成验证
- CLI 项目通常应优先有面向本地二进制或真实命令入口的端到端校验
- 移动端或桌面端项目通常应优先有 simulator、emulator 或 UI 自动化，能够驱动真实应用壳层
- 库或 SDK 项目通常应优先有面向使用方的集成测试、示例应用或 fixture project，用来覆盖公共 API
- worker、队列、cron 或数据流水线项目通常应优先有基于 fixture 的端到端运行，验证任务投递、持久化产物或下游副作用
- 基础设施或 IaC 项目通常应优先有可重复执行的 plan / dry-run 校验，以及针对目标环境或模拟目标的 smoke validation
- 如果这些验证层还不存在，agent 要明确告诉用户这一点，并建议补对应的脚本、测试集或 skill，而不是假装任务已经定义完整
- 即便有 E2E，单元测试缺失也依然是单独的风险信号，需要更明确地提示

### 这个产品在做什么

你可以把 Relay 理解成一个专门服务于编码 Agent 的执行层：

- `pipeline` 定义项目级执行契约
- `issue` 定义一次具体任务
- `serve` 轮询任务队列并驱动 planning 和 coding loops
- `feature_list.json` 是任务完成的结构化真相源
- `progress.txt` 是每轮之间的交接日志
- `events.log`、`runs/` 和 `issue.json` 让失败和执行状态可追查

Relay 会按这个顺序解析本机 runner：issue 的 `agent_runner` 覆盖 → pipeline 的 `agent_runner` → 默认 `codex`。
当前支持的本机 runner 是 `codex` 和 `claude`。

### Planning 和 Coding 的分工

Relay 默认把 planning 和 coding 放在两个不同粒度上执行。

- planning 默认站在 phase、依赖关系、验证边界、验收边界上思考，尤其是 repo 级或系统级改动
- feature 更适合定义成相对闭环的功能切片，而不是零散的小文件任务
- 每一轮 coding 默认只拿一个主 feature，最多再带一小簇紧耦合任务，并且在动手前先决定这一轮要怎么验证
- 没做完的 rollout 范围必须继续留在 `feature_list.json` 里；`progress.txt` 负责交接，不负责偷偷缩 scope
- 默认验证顺序是：先跑最贴切的包级 proof，再跑 `go test ./...`，最后跑能覆盖实际 CLI 表面的最小 `go run ./cmd/relay ...` 命令

当前 E2E 缺口：共享的 `relay-e2e` skill 目前只有在 issue 进入 `done` 且 `feature_list.json` 全部 `passes: true` 时才算 PASS。所以它适合作为 orchestration smoke coverage，但还不能作为“某一轮 coding 只完成一个切片、其余 feature 继续显式 pending”这种健康中间态的验收证明。

### 项目内置 Agent Skill

这个仓库内置了一份给其他 agent 使用的自包含顶层 skill：

```text
skills/relay-operator/
```

只安装 `relay-operator` 就足够覆盖常见使用场景，包括：

- repository-specific pipeline 编写
- 带明确验收条件的 issue 拆解
- `relay serve` 常驻运行
- 结合 artifact 和宿主机日志排障

正式发布的 npm 包里还会带上 `skills/relay-operator/skill.json`。这个元数据会跟随发布 tag 写入版本号，用来让 skill 和打包出去的 Relay CLI 保持同一版本语义，并固定刷新命令。

skill 的安装方式优先使用 `npx skills` 分发，而不是手动复制目录。

### 设计来源

Relay 并不是对相关文章做简单复刻，而是把里面对 agent harness 的关键判断，落成一个可以直接运行在代码仓库上的产品模型。

对应到 Relay，大致体现为：

- Prompt 不是核心，harness 才是核心。真正决定稳定性的，是任务分阶段、状态持久化、失败恢复、外部验证这些机制。
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
- `agent_runner`（可选：`codex` 或 `claude`；未设置时默认 `codex`）
- `loop_num`（可选，默认值是 `20`）
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
- `agent_runner`（可选覆盖；不填时继承 pipeline runner，再回退到 `codex`）
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
- 本机已安装 `codex` 或 `claude`，并且在 `PATH` 里
- 如果 issue 和 pipeline 都没有设置 `agent_runner`，Relay 会默认使用 `codex`

如果你更偏向源码安装：

```bash
go install github.com/eddiearc/relay/cmd/relay@latest
```

### 常用命令

具体示例和流程说明优先看 `relay help` 和 `relay help <command>`。

- `relay pipeline add <name> --init-command ... --plan-prompt-file ... --coding-prompt-file ...`
- `relay pipeline edit <name> [--init-command ...] [--agent-runner codex|claude] [--loop-num ...] [--plan-prompt-file ...] [--coding-prompt-file ...]`
- `relay pipeline import -file pipeline.yaml`
- `relay issue add --pipeline <name> [--agent-runner codex|claude] --goal ... --description ...`
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
- `relay upgrade`
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

如果你想在发正式版本前先做一轮冒烟验证，可以在 Actions 页面手动运行 `Release Smoke Test` workflow。它会创建一个临时的草稿 release tag，例如 `v0.0.0-smoke.<run_id>`，上传各平台压缩包，生成 npm 包，并用 `npm pack --dry-run` 做一轮校验，最后再把这个临时 release 和 tag 清理掉。

npm 包结构和首次接入 npm registry 的细节见 [`npm/README.md`](./npm/README.md)。

推荐的 npm 发布方式是 GitHub Actions OIDC 的 Trusted Publishing。当前 release workflow 已经带上了 `id-token: write`；你只需要在 npm 后台为每个 `@eddiearc/*` 包配置 Trusted Publisher，并把 workflow filename 填成 `release-policy.yml`。

当前还不建议发 Windows 包，因为运行时里有 `zsh` 和 `SIGKILL` 这样的 Unix 假设。

### 项目范围

Relay 当前是一个面向真实代码仓库和真实编码任务的聚焦型 harness。

它的目标不是做一个通用的 AI 聊天壳，而是提供一个围绕 planning、coding、持久化和验证展开的耐久执行模型。
