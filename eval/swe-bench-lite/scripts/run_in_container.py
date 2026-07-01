#!/usr/bin/env python3
"""容器内单任务编排:setup -> baseline -> agent -> patch -> evaluate -> metrics。

在 SWE-bench 镜像内运行。读取 /eval/data/tasks.json,按环境变量 INSTANCE_ID 选任务,
所有产物写入 /eval/results/<instance_id>/。

评测协议(对齐 SWE-bench):
  1. clone 仓库并 checkout 到 base_commit,安装依赖。
  2. baseline 校验:应用 test_patch,跑 FAIL_TO_PASS(应失败)+ PASS_TO_PASS(应通过),
     记录 test_before.log,然后**反向撤销** test_patch,使 agent 看不到 gold 测试。
  3. 在纯净 base 上运行 lcoder agent 修复源码。
  4. 提取 agent 的代码改动为 patch.diff(此时尚未含 test_patch)。
  5. 在 agent 改动之上应用 test_patch,跑 F2P + P2P,记录 test_after.log。
  6. 分类:resolved / partial / failed / timeout / error,汇总指标。
"""
import json
import os
import subprocess
import sys
import time
import glob

WORKDIR = "/workspace/repo"
RESULTS_ROOT = "/eval/results"
TASKS_FILE = "/eval/data/tasks.json"
CONFIG = "/eval/config/lcoder.yaml"
PROMPT_TMPL = "/eval/prompts/swe_task.txt"

AGENT_TIMEOUT_S = int(os.environ.get("AGENT_TIMEOUT_S", "1500"))
INSTALL_TIMEOUT_S = int(os.environ.get("INSTALL_TIMEOUT_S", "1200"))
TEST_TIMEOUT_S = int(os.environ.get("TEST_TIMEOUT_S", "600"))
# PASS_TO_PASS 可能很多,MVP 限制数量以约束耗时(非静默截断:会在结果里记录)。
P2P_CAP = int(os.environ.get("P2P_CAP", "20"))


def run(cmd, cwd=None, timeout=None, env=None, capture=True):
    """执行命令,返回 (returncode, stdout+stderr)。"""
    print(f"$ {cmd if isinstance(cmd, str) else ' '.join(cmd)}", flush=True)
    p = subprocess.run(
        cmd,
        cwd=cwd,
        shell=isinstance(cmd, str),
        timeout=timeout,
        env=env,
        stdout=subprocess.PIPE if capture else None,
        stderr=subprocess.STDOUT if capture else None,
        text=True,
    )
    out = p.stdout or ""
    return p.returncode, out


def load_task():
    with open(TASKS_FILE, encoding="utf-8") as f:
        tasks = json.load(f)
    iid = os.environ.get("INSTANCE_ID", "")
    if iid:
        for t in tasks:
            if t["instance_id"] == iid:
                return t
        raise SystemExit(f"INSTANCE_ID {iid} not in tasks.json")
    return tasks[0]


def git(args, cwd=WORKDIR, timeout=300):
    return run(["git"] + args, cwd=cwd, timeout=timeout)


def test_files_from_patch(patch_text):
    """从 test_patch 的 diff header(+++ b/...)提取被改动的测试文件路径。"""
    files = []
    for line in patch_text.splitlines():
        if line.startswith("+++ b/"):
            files.append(line[len("+++ b/"):].strip())
    return files


def resolve_nodes(nodes, patch_text):
    """把 FAIL_TO_PASS / PASS_TO_PASS 解析为 pytest 可识别的 node id。

    SWE-bench 不同仓库的存法不一:
    - requests 等存完整 node id(含 '::' 或路径分隔),原样可用。
    - sympy 等存裸函数名(如 'test_decompose'),pytest 无法直接定位,
      需用 test_patch 触及的(首个)测试文件做前缀:'<file>::<func>'。
    """
    tfiles = test_files_from_patch(patch_text)
    out = []
    for n in nodes:
        if "::" in n or "/" in n:
            out.append(n)
        elif tfiles:
            out.append(f"{tfiles[0]}::{n}")
        else:
            out.append(n)
    return out


def run_pytest(nodes, outpath, env):
    """运行一组 pytest 节点,返回 (all_passed, returncode)。空集合视为通过。"""
    if not nodes:
        with open(outpath, "w") as f:
            f.write("(no tests in this group)\n")
        return True, 0
    cmd = ["python", "-m", "pytest", "-p", "no:cacheprovider", "--tb=short", "-q"] + nodes
    rc, out = run(cmd, cwd=WORKDIR, timeout=TEST_TIMEOUT_S, env=env)
    with open(outpath, "w", encoding="utf-8") as f:
        f.write(out)
    return rc == 0, rc


def main():
    task = load_task()
    iid = task["instance_id"]
    rdir = os.path.join(RESULTS_ROOT, iid)
    os.makedirs(rdir, exist_ok=True)

    result = {
        "instance_id": iid,
        "repo": task["repo"],
        "base_commit": task["base_commit"],
        "status": "error",
        "stages": {},
        "p2p_evaluated": 0,
        "p2p_total": len(task["pass_to_pass"]),
        "p2p_capped": False,
    }
    t0 = time.time()
    env = dict(os.environ)
    env["HOME"] = "/root"

    test_patch_path = os.path.join(rdir, "test_patch.diff")
    with open(test_patch_path, "w", encoding="utf-8") as f:
        f.write(task["test_patch"])

    try:
        # 1) 取源码 + 造合成 base -------------------------------------------
        # 容器内 git 用 GnuTLS,对 github 的 git-smart-http 握手偶发中断;
        # 改用 codeload 的 tarball 单次 HTTPS GET(Python/OpenSSL 栈,更稳),
        # 解包后 git init 造一个合成 base 提交(评测只需干净 HEAD 可 diff,
        # 不依赖真实 sha)。
        os.makedirs("/workspace", exist_ok=True)
        sha = task["base_commit"]
        tar_url = f"https://codeload.github.com/{task['repo']}/tar.gz/{sha}"
        clone_ok = False
        for attempt in range(1, 7):
            try:
                run(["rm", "-rf", WORKDIR, "/tmp/ex", "/tmp/src.tgz"])
                import urllib.request, tarfile, shutil
                urllib.request.urlretrieve(tar_url, "/tmp/src.tgz")
                os.makedirs("/tmp/ex", exist_ok=True)
                with tarfile.open("/tmp/src.tgz") as tf:
                    tf.extractall("/tmp/ex")
                subs = [os.path.join("/tmp/ex", d) for d in os.listdir("/tmp/ex")]
                shutil.move(subs[0], WORKDIR)
                git(["init", "-q"], timeout=60)
                git(["config", "user.email", "eval@lcoder"], timeout=30)
                git(["config", "user.name", "lcoder-eval"], timeout=30)
                git(["add", "-A"], timeout=300)
                git(["commit", "-q", "-m", f"base {sha}"], timeout=300)
                clone_ok = True
                break
            except Exception as ce:  # noqa: BLE001
                print(f"[clone] attempt {attempt} failed: {ce}", flush=True)
                time.sleep(min(5 * attempt, 30))
        if not clone_ok:
            result["stages"]["clone"] = "failed"
            raise RuntimeError("source fetch failed after retries")
        result["stages"]["clone"] = "ok"

        # 2) install ----------------------------------------------------------
        rc, out = run(task["install_cmd"], cwd=WORKDIR, timeout=INSTALL_TIMEOUT_S, env=env)
        with open(os.path.join(rdir, "install.log"), "w", encoding="utf-8") as f:
            f.write(out)
        result["stages"]["install"] = "ok" if rc == 0 else "failed"
        if rc != 0:
            raise RuntimeError("install failed")

        # 裸函数名(sympy 等)解析为 '<test_file>::<func>',full node id 原样保留。
        f2p = resolve_nodes(task["fail_to_pass"], task["test_patch"])
        p2p_all = resolve_nodes(task["pass_to_pass"], task["test_patch"])
        p2p = p2p_all[:P2P_CAP]
        result["p2p_evaluated"] = len(p2p)
        result["p2p_capped"] = len(p2p) < len(p2p_all)

        # 3) baseline:应用 test_patch,确认 F2P 失败 / P2P 通过 ----------------
        rc, out = run(["git", "apply", test_patch_path], cwd=WORKDIR, timeout=120)
        if rc != 0:
            result["stages"]["baseline_apply_test_patch"] = "failed"
            with open(os.path.join(rdir, "test_patch_apply.log"), "w") as f:
                f.write(out)
            raise RuntimeError("test_patch did not apply on base")
        result["stages"]["baseline_apply_test_patch"] = "ok"

        f2p_before, _ = run_pytest(f2p, os.path.join(rdir, "test_before.log"), env)
        p2p_before, _ = run_pytest(p2p, os.path.join(rdir, "test_before_p2p.log"), env)
        result["baseline"] = {
            "fail_to_pass_passed": f2p_before,  # 期望 False(修复前应失败)
            "pass_to_pass_passed": p2p_before,  # 期望 True
        }

        # 撤销 test_patch,让 agent 看不到 gold 测试
        run(["git", "apply", "-R", test_patch_path], cwd=WORKDIR, timeout=120)
        git(["checkout", "-q", "--", "."])
        run(["git", "clean", "-fdq"], cwd=WORKDIR, timeout=120)

        # 4) agent 运行 -------------------------------------------------------
        with open(PROMPT_TMPL, encoding="utf-8") as f:
            tmpl = f.read()
        prompt = tmpl.format(
            repo=task["repo"],
            workdir=WORKDIR,
            problem_statement=task["problem_statement"],
            fail_to_pass="\n".join(f2p),
            test_cmd=task["test_cmd"] + " " + " ".join(f2p[:3]),
        )
        events_path = os.path.join(rdir, "events.jsonl")
        agent_stderr = os.path.join(rdir, "agent.stderr.log")
        # lcoder 的 --config 路径不展开 {env:...},这里把令牌替换为实际值生成运行时配置。
        runtime_cfg = "/tmp/lcoder-runtime.yaml"
        with open(CONFIG, encoding="utf-8") as cf:
            cfg_text = cf.read()
        cfg_text = cfg_text.replace(
            "{env:ANTHROPIC_AUTH_TOKEN}", os.environ.get("ANTHROPIC_AUTH_TOKEN", "")
        )
        with open(runtime_cfg, "w", encoding="utf-8") as cf:
            cf.write(cfg_text)
        agent_timed_out = False
        agent_start = time.time()
        try:
            with open(events_path, "w", encoding="utf-8") as ev, \
                 open(agent_stderr, "w", encoding="utf-8") as er:
                p = subprocess.run(
                    ["lcoder", "--config", runtime_cfg, "--json", "-p", prompt],
                    cwd=WORKDIR, env=env, stdout=ev, stderr=er,
                    timeout=AGENT_TIMEOUT_S,
                )
            result["stages"]["agent"] = "ok" if p.returncode == 0 else f"exit_{p.returncode}"
        except subprocess.TimeoutExpired:
            agent_timed_out = True
            result["stages"]["agent"] = "timeout"
        result["agent_duration_s"] = round(time.time() - agent_start, 1)

        # 5) 提取 agent patch(尚未含 test_patch) ----------------------------
        run(["git", "add", "-A"], cwd=WORKDIR, timeout=120)
        rc, diff = run(["git", "diff", "--cached", "HEAD"], cwd=WORKDIR, timeout=120)
        with open(os.path.join(rdir, "patch.diff"), "w", encoding="utf-8") as f:
            f.write(diff)
        result["patch_nonempty"] = bool(diff.strip())
        # 取消暂存,保留工作树改动以便叠加 test_patch
        run(["git", "reset", "-q", "HEAD"], cwd=WORKDIR, timeout=120)

        # 6) 评测:在 agent 改动之上应用 test_patch ---------------------------
        rc, out = run(["git", "apply", test_patch_path], cwd=WORKDIR, timeout=120)
        if rc != 0:
            # agent 可能动了测试文件导致冲突;记录并标记评测失败
            with open(os.path.join(rdir, "eval_apply.log"), "w") as f:
                f.write(out)
            result["stages"]["eval_apply_test_patch"] = "failed"
            result["status"] = "error"
        else:
            result["stages"]["eval_apply_test_patch"] = "ok"
            f2p_after, _ = run_pytest(f2p, os.path.join(rdir, "test_after.log"), env)
            p2p_after, _ = run_pytest(p2p, os.path.join(rdir, "test_after_p2p.log"), env)
            result["fail_to_pass_passed"] = f2p_after
            result["pass_to_pass_passed"] = p2p_after
            if agent_timed_out:
                result["status"] = "timeout"
            elif f2p_after and p2p_after:
                result["status"] = "resolved"
            elif f2p_after and not p2p_after:
                result["status"] = "partial"
            else:
                result["status"] = "failed"

        # 7) 指标 -------------------------------------------------------------
        result["metrics"] = collect_metrics(events_path, env)

    except Exception as e:  # noqa: BLE001
        result["error"] = str(e)
        print(f"[run] ERROR: {e}", file=sys.stderr, flush=True)

    result["duration_s"] = round(time.time() - t0, 1)
    with open(os.path.join(rdir, "result.json"), "w", encoding="utf-8") as f:
        json.dump(result, f, indent=2, ensure_ascii=False)
    print(f"[run] {iid} -> status={result['status']} "
          f"({result['duration_s']}s)", flush=True)
    print(json.dumps(result, indent=2, ensure_ascii=False))


def collect_metrics(events_path, env):
    """从 events.jsonl(权威)+ observability(尽力)采集过程指标。"""
    m = {"turns": 0, "tool_calls": 0, "file_edits": 0, "tests_run": 0,
         "messages": 0, "errors": 0}
    try:
        with open(events_path, encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                try:
                    ev = json.loads(line)
                except json.JSONDecodeError:
                    continue
                t = ev.get("type")
                if t == "turn_start":
                    m["turns"] += 1
                elif t == "tool_execution_start":
                    m["tool_calls"] += 1
                    name = (ev.get("tool_name") or "").lower()
                    if name in ("edit", "write"):
                        m["file_edits"] += 1
                    if name == "bash":
                        cmd = str(ev.get("args", {}).get("command", ""))
                        if "pytest" in cmd or "test" in cmd:
                            m["tests_run"] += 1
                elif t == "message_end":
                    m["messages"] += 1
                elif t == "error":
                    m["errors"] += 1
    except FileNotFoundError:
        pass

    # observability:尽力读取 token / cost(模型不在 catalog,cost 可能为 0)
    obs_glob = "/root/.lcoder/observability/sessions/*.jsonl"
    tokens = {"prompt": 0, "completion": 0, "cache_read": 0, "cache_write": 0}
    for path in glob.glob(obs_glob):
        try:
            with open(path, encoding="utf-8") as f:
                for line in f:
                    try:
                        rec = json.loads(line)
                    except json.JSONDecodeError:
                        continue
                    for k_src, k_dst in (
                        ("prompt_tokens", "prompt"),
                        ("completion_tokens", "completion"),
                        ("cache_read_tokens", "cache_read"),
                        ("cache_write_tokens", "cache_write"),
                    ):
                        v = _deep_get(rec, k_src)
                        if isinstance(v, (int, float)):
                            tokens[k_dst] += int(v)
        except OSError:
            continue
    m["tokens"] = tokens
    return m


def _deep_get(obj, key):
    """在嵌套 dict 中查找某 key 的最后一个数值出现。"""
    found = None
    if isinstance(obj, dict):
        for k, v in obj.items():
            if k == key and isinstance(v, (int, float)):
                found = v
            sub = _deep_get(v, key)
            if sub is not None:
                found = sub
    elif isinstance(obj, list):
        for v in obj:
            sub = _deep_get(v, key)
            if sub is not None:
                found = sub
    return found


if __name__ == "__main__":
    main()
