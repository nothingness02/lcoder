# SWE-bench Lite MVP 评估平台

用 SWE-bench Lite 真实任务,端到端衡量 Lcoder 的软件工程能力(理解→定位→修复→验证)。
实施依据见 `../../docs/mvp-swe-bench-lite.md`。

## 架构

本机 host shell 无外网,仅 Docker 容器有外网。因此**全流程在容器内运行**:

```
host (run.py)                      container (python:3.11 + git + lcoder)
  ├─ 交叉编译 linux lcoder   ──┐
  ├─ docker build           ──┤──>  /usr/local/bin/lcoder
  ├─ select_task (容器内)    ──┘     clone → checkout → pip install
  └─ run_task   (容器内)            → baseline(apply test_patch,验 F2P 失败)
                                     → 撤销 test_patch
                                     → lcoder agent 修复(经 Kimi 网关)
                                     → 提取 patch.diff
                                     → 叠加 test_patch → 跑 F2P+P2P → 分类
```

模型经 Kimi coding 网关(Anthropic 兼容)驱动 `kimi-k2.7-code`,provider 名用
`moonshot` 以经别名从 models.dev 加载指标;鉴权令牌由 host 通过
`ANTHROPIC_AUTH_TOKEN` 注入容器,见 `config/lcoder.yaml`。

## 目录

```
config/lcoder.yaml        评估专用 lcoder 配置(网关 + 全放行权限 + 钉死上下文窗口)
Dockerfile                评估镜像
prompts/swe_task.txt      任务 prompt 模板
scripts/select_task.py    从 HF 拉取并筛选任务 -> data/tasks.json
scripts/run_in_container.py  容器内单任务编排(setup/baseline/agent/patch/eval/metrics)
runner/run.py             host 编排(编译 + 构建 + 筛选 + 运行 + 汇总)
data/tasks.json           筛选出的任务
results/<instance_id>/    每任务产物
```

## 用法

```bash
# 一键:交叉编译 + 构建镜像 + 筛选(默认 psf/requests 最小任务)+ 运行
python eval/swe-bench-lite/runner/run.py --build --select

# 指定具体任务
python eval/swe-bench-lite/runner/run.py --build --select --instance psf__requests-2317

# 已构建/已选,仅重跑
python eval/swe-bench-lite/runner/run.py
```

## 产物(results/<instance_id>/)

| 文件 | 含义 |
|------|------|
| `result.json` | 状态分类 + 阶段 + baseline + 指标 |
| `patch.diff` | agent 的代码改动(不含 gold 测试) |
| `test_patch.diff` | 注入的 gold 测试 |
| `test_before.log` / `test_after.log` | 修复前/后 FAIL_TO_PASS 结果 |
| `test_before_p2p.log` / `test_after_p2p.log` | PASS_TO_PASS 结果 |
| `events.jsonl` | 完整事件流 |
| `install.log` / `agent.stderr.log` | 安装日志 / agent stderr |

## 结果分类

| 状态 | 条件 |
|------|------|
| resolved | FAIL_TO_PASS 全过 且 PASS_TO_PASS 全过 |
| partial | FAIL_TO_PASS 全过,PASS_TO_PASS 有失败 |
| failed | FAIL_TO_PASS 仍有失败 |
| timeout | agent 超过 `AGENT_TIMEOUT_S`(默认 1500s) |
| error | 环境/clone/install/打补丁等异常 |

## MVP 已知约束

- PASS_TO_PASS 默认只评测前 `P2P_CAP`(20)个,`result.json` 的 `p2p_capped` 字段会标记是否截断。
- 测试命令默认 `python -m pytest`;复杂仓库需在 task 的 `test_cmd` / `install_cmd` 覆盖。
- `kimi-k2.7-code` 经 `moonshot`->`moonshotai` 别名从 models.dev 加载指标(窗口/pricing);容器内无网时回退 `config/lcoder.yaml` 钉死的窗口,cost 估算可能为 0。
