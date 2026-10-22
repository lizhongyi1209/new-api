#!/usr/bin/env python3
"""Record one user's task lifecycle and usage-log metadata without request bodies."""

import argparse
import fcntl
import json
import os
import signal
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path


def query_rows(sql):
    result = subprocess.run(
        ["docker", "exec", "postgres", "psql", "-U", "root", "-d", "new-api", "-X", "-q", "-At", "-c", sql],
        capture_output=True,
        text=True,
        timeout=20,
        check=True,
    )
    return [json.loads(line) for line in result.stdout.splitlines() if line]


def record(output, event, **fields):
    entry = {"at": datetime.now(timezone.utc).isoformat(), "event": event, **fields}
    output.write(json.dumps(entry, ensure_ascii=False, separators=(",", ":")) + "\n")
    output.flush()


def save_state(path, state):
    temporary_path = path.with_suffix(path.suffix + ".tmp")
    temporary_path.write_text(json.dumps(state, ensure_ascii=False), encoding="utf-8")
    os.replace(temporary_path, path)


def task_projection():
    return (
        "json_build_object("
        "'id',id,'task_id',task_id,'platform',platform,'status',status,"
        "'fail_reason',left(coalesce(fail_reason,''),400),'channel_id',channel_id,"
        "'submit_time',submit_time,'finish_time',finish_time,"
        "'model',properties::jsonb->>'origin_model_name',"
        "'request_id',private_data::jsonb->>'request_id',"
        "'client_request_bytes',private_data::jsonb->'generate_image_timing'->>'client_request_bytes',"
        "'upstream_status',private_data::jsonb->'generate_image_timing'->>'upstream_status')"
    )


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--user-id", type=int, required=True)
    parser.add_argument("--interval", type=int, default=20)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if args.user_id <= 0 or args.interval < 5 or not args.output.is_absolute():
        parser.error("user ID must be positive, interval at least 5, and output path absolute")

    os.umask(0o077)
    lock_path = Path(str(args.output) + ".lock")
    state_path = Path(str(args.output) + ".state")
    with lock_path.open("w", encoding="utf-8") as lock_file:
        try:
            fcntl.flock(lock_file, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            print("monitor already running", file=sys.stderr)
            return 1

        if state_path.exists():
            state = json.loads(state_path.read_text(encoding="utf-8"))
            if state.get("user_id") != args.user_id:
                print("monitor state belongs to a different user", file=sys.stderr)
                return 1
        else:
            baseline = query_rows(
                "SELECT json_build_object('task_id',coalesce(max(id),0)) "
                f"FROM tasks WHERE user_id={args.user_id}"
            )[0]
            log_baseline = query_rows(
                "SELECT json_build_object('log_id',coalesce(max(id),0)) "
                f"FROM logs WHERE user_id={args.user_id}"
            )[0]
            active = query_rows(
                f"SELECT {task_projection()} FROM tasks WHERE user_id={args.user_id} "
                "AND status NOT IN ('SUCCESS','FAILURE') ORDER BY id"
            )
            state = {
                "user_id": args.user_id,
                "last_task_id": baseline["task_id"],
                "last_log_id": log_baseline["log_id"],
                "active": {str(row["id"]): row for row in active},
            }
            save_state(state_path, state)

        stopping = False

        def stop(_signal, _frame):
            nonlocal stopping
            stopping = True

        signal.signal(signal.SIGTERM, stop)
        signal.signal(signal.SIGINT, stop)

        with args.output.open("a", encoding="utf-8") as output:
            record(output, "started", user_id=args.user_id, interval_seconds=args.interval,
                   active_tasks=len(state["active"]), last_task_id=state["last_task_id"])
            last_heartbeat = time.monotonic()
            while not stopping:
                active_ids = ",".join(state["active"].keys())
                active_clause = f" OR id IN ({active_ids})" if active_ids else ""
                task_sql = (
                    f"SELECT {task_projection()} FROM tasks WHERE user_id={args.user_id} "
                    f"AND (id>{state['last_task_id']}{active_clause}) ORDER BY id"
                )
                log_sql = (
                    "SELECT json_build_object('id',id,'type',type,'quota',quota,"
                    "'model',model_name,'channel_id',channel_id,'created_at',created_at) "
                    f"FROM logs WHERE user_id={args.user_id} AND id>{state['last_log_id']} ORDER BY id"
                )
                try:
                    tasks = query_rows(task_sql)
                    usage_logs = query_rows(log_sql)
                except (subprocess.SubprocessError, ValueError) as exc:
                    record(output, "poll_error", error=str(exc)[:200])
                    time.sleep(args.interval)
                    continue

                for row in tasks:
                    task_id = str(row["id"])
                    previous = state["active"].get(task_id)
                    if previous is None or row != previous:
                        record(output, "task_new" if previous is None else "task_updated", **row)
                    state["last_task_id"] = max(state["last_task_id"], row["id"])
                    if row["status"] in ("SUCCESS", "FAILURE"):
                        state["active"].pop(task_id, None)
                    else:
                        state["active"][task_id] = row

                for row in usage_logs:
                    record(output, "usage_log", **row)
                    state["last_log_id"] = max(state["last_log_id"], row["id"])

                save_state(state_path, state)
                if time.monotonic() - last_heartbeat >= 3600:
                    record(output, "heartbeat", active_tasks=len(state["active"]))
                    last_heartbeat = time.monotonic()
                time.sleep(args.interval)

            record(output, "stopped", user_id=args.user_id)
    return 0


if __name__ == "__main__":
    sys.exit(main())
