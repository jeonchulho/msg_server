#!/usr/bin/env python3

import json
import sys
from pathlib import Path


def usage() -> None:
    print("usage: render_k6_summary_md.py <summary.json> <chat|msa>", file=sys.stderr)


def get_metric(metrics: dict, name: str, key: str, default: str = "n/a"):
    return metrics.get(name, {}).get("values", {}).get(key, default)


def fmt_number(value):
    if isinstance(value, (int, float)):
        return f"{value:.3f}" if isinstance(value, float) else str(value)
    return str(value)


def main() -> int:
    if len(sys.argv) != 3:
        usage()
        return 1

    summary_path = Path(sys.argv[1])
    profile = sys.argv[2].strip().lower()

    if profile not in {"chat", "msa"}:
        usage()
        return 1

    if not summary_path.exists():
        print(f"summary file not found: {summary_path}", file=sys.stderr)
        return 1

    with summary_path.open("r", encoding="utf-8") as file:
        data = json.load(file)

    metrics = data.get("metrics", {})

    requests = get_metric(metrics, "http_reqs", "count")
    failed_rate = get_metric(metrics, "http_req_failed", "rate")
    http_p95 = get_metric(metrics, "http_req_duration", "p(95)")
    http_p99 = get_metric(metrics, "http_req_duration", "p(99)")
    http_avg = get_metric(metrics, "http_req_duration", "avg")

    title = "k6 Chat Hotpath Summary" if profile == "chat" else "k6 MSA Hotpath Summary"

    lines = [
        f"## {title}",
        "",
        f"- requests: {fmt_number(requests)}",
        f"- failed_rate: {fmt_number(failed_rate)}",
        f"- http_p95_ms: {fmt_number(http_p95)}",
        f"- http_p99_ms: {fmt_number(http_p99)}",
        f"- http_avg_ms: {fmt_number(http_avg)}",
    ]

    if profile == "msa":
        chat_p95 = get_metric(metrics, "msa_chat_create_message_ms", "p(95)")
        session_p95 = get_metric(metrics, "msa_session_update_status_ms", "p(95)")
        tenanthub_p95 = get_metric(metrics, "msa_tenanthub_list_ms", "p(95)")
        lines.extend(
            [
                f"- chat_p95_ms: {fmt_number(chat_p95)}",
                f"- session_p95_ms: {fmt_number(session_p95)}",
                f"- tenanthub_p95_ms: {fmt_number(tenanthub_p95)}",
            ]
        )

    print("\n".join(lines))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
