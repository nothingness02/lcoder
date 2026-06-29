# 集成测试 (test/integration)

端到端测试,直接用本机的**真实配置和 API key**(`~/.lcoder/config.yaml` + credentials + 环境变量)
跑完整的 agent loop,验证真实供应商下的行为。

所有测试都用 `integration` build tag 隔离,默认的 `go test ./...` 和 CI **不会**触网、也不需要凭证。

## 运行

```bash
# 跑第一个真实用例(读取 go.mod 并展示每个 turn 的结构化上下文与工具调用)
go test -tags integration ./test/integration/ -run TestAgentRealRun -v

# 并行工具调用(需真实 provider+key,否则 t.Skip)
go test -tags integration ./test/integration/ -run TestParallelToolCalls -v

# 各压缩机制验证(确定性子用例无需 provider 始终运行;真实 LLM 摘要子用例无 key 时自动 skip)
go test -tags integration ./test/integration/ -run TestCompactionMechanisms -v
```

无配置 / 无 API key 时,测试会自动 `t.Skip`,不会失败。

## 当前用例

### TestAgentRealRun

模拟真实情况:加载真实配置与 api_key,发送一个简单请求(让模型用工具读取仓库根目录的
`go.mod` 并回答 module 名称),订阅事件总线捕获**每个 turn 之后**的结构化数据:

- `assistant_message` —— 该 turn 的助手消息(含 thinking / 文本 / tool_calls)
- `tool_results` —— 该 turn 的工具结果消息
- `tool_calls` —— 工具调用的名称、参数、结果文本、是否出错
- `context_stats` —— `contextmgr.Manager.Stats()` 的快照(各 block 的 token 占用与预算)

结果以 JSON 写入 `test/integration/output/agent_run_<时间戳>.json`。
报告**不含** api_key 或任何凭证。

### TestParallelToolCalls

验证 agent 真的**并发执行同一轮里的多个 tool_call**,而非串行逐个等待。

测试内注册一个仅供本用例使用的慢工具 `slow_probe`(`Execute` 内固定 `time.Sleep(300ms)`
后返回一行结果)——真实文件读取在微秒级完成,无法证明区间重叠,故用可测量的慢工具。
Prompt 强指令模型在**同一轮**里并行调用 `slow_probe` 三次(标签 alpha/beta/gamma)。

事件总线上的 `ToolExecutionStart/End` 事件来自并发 goroutine,捕获 handler 加锁并在
handler 内用 `time.Now()` 抓墙钟时间(事件本身不带时间戳)。对所有区间做扫描线求最大并发数,
断言 `MaxConcurrency >= 2`(区间重叠 = 真并发)。

结果写入 `output/parallel_tools_<时间戳>.{md,json}`:markdown 含 ASCII 甘特图、区间明细表、
并发结论与助手消息。

> 不确定性:模型行为非确定。理论上模型可能把三次调用拆到不同 turn 串行执行,此时
> `MaxConcurrency == 1` 断言失败——这正确反映了"未并行"。Prompt 已尽量强约束并行;
> 观测到的真实结果为 3 个调用完全重叠。

### TestCompactionMechanisms

逐一驱动 `compaction`、`contextmgr` 窗口策略截断与 `MaybeCompact` 已提交压缩的每条路径。用 `t.Run` 分 6 个子用例,
**前 5 个确定性子用例无需 provider 始终运行**,第 6 个真实 LLM 摘要子用例无 key 时单独 skip:

1. `SimpleSummarize_KeepRecent` —— `compaction.KeepRecent.Compact` 占位摘要。
2. `WindowPolicy_Truncation_NoSummarizer` —— 无摘要器时尾部截断到预算内。
3. `MaybeCompact_EagerCompaction_SimpleSummarizer` —— `total > CompactLimit` 时 `Manager.MaybeCompact` 折叠旧消息为一条摘要并原地提交进 recent 块。
4. `StripLeadingOrphanToolResults` —— 截断后剥离开头的孤儿 `tool_result`。
5. `CircuitBreaker_Fallback` —— 断路器 3 次失败后开路;摘要失败时 `MaybeCompact` 返回非致命 error 且不 mutate,`BuildTurnRequest` 与压缩解耦仍只截断、不报错。
6. `RealLLMSummarizer_EagerCompaction` —— 真实 provider 经 `MaybeCompact` 生成摘要并提交(无 key 时 skip)。

结果写入 `output/compaction_<时间戳>.{md,json}`:每个机制一节,展示压缩前后的消息序列与
token 变化、PASS/SKIP 结论。报告**不含**任何凭证。

## 说明

- `output/` 目录不纳入版本控制(见 `.gitignore`)。
- 该用例复刻了 `cmd/lcoder/main.go` 里 `buildEngine` 的接线逻辑(catalog → engine →
  client → RegisterProvider),因为 `package main` 无法被测试导入。
