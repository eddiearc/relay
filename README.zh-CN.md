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
Then inspect the repository, write a repository-specific pipeline, rewrite the task as a Relay issue with explicit acceptance criteria, and tell me whether to run relay serve --once or relay serve persistently.
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
- 编写 repository-specific pipeline
- 把任务改写成带明确验收条件的 Relay issue
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

### 这个产品在做什么

你可以把 Relay 理解成一个专门服务于编码 Agent 的执行层：

- `pipeline` 定义项目级执行契约
- `issue` 定义一次具体任务
- `serve` 轮询任务队列并驱动 planning 和 coding loops
- `feature_list.json` 是任务完成的结构化真相源
- `progress.txt` 是每轮之间的交接日志
- `events.log`、`runs/` 和 `issue.json` 让失败和执行状态可追查

Relay 当前默认通过本机 `codex` CLI 执行 agent。

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

### 常用命令

具体示例和流程说明优先看 `relay help` 和 `relay help <command>`。

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

npm 包结构和首次接入 npm registry 的细节见 [`npm/README.md`](./npm/README.md)。

推荐的 npm 发布方式是 GitHub Actions OIDC 的 Trusted Publishing。当前 release workflow 已经带上了 `id-token: write`；你只需要在 npm 后台为每个 `@eddiearc/*` 包配置 Trusted Publisher，并把 workflow filename 填成 `release.yml`。

当前还不建议发 Windows 包，因为运行时里有 `zsh` 和 `SIGKILL` 这样的 Unix 假设。

### 项目范围

Relay 当前是一个面向真实代码仓库和真实编码任务的聚焦型 harness。

它的目标不是做一个通用的 AI 聊天壳，而是提供一个围绕 planning、coding、持久化和验证展开的耐久执行模型。
