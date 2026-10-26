import argparse
import json
import statistics
from pathlib import Path
from typing import Any

from app.config import get_settings
from app.metrics.recorder import _FILENAME

_P99 = 0.99


def _load(path: Path) -> list[dict[str, Any]]:
    if not path.exists():
        return []
    entries: list[dict[str, Any]] = []
    with open(path, encoding="utf-8") as handle:
        for line in handle:
            line = line.strip()
            if not line:
                continue
            try:
                entries.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    return entries


def _pct(values: list[float], pct: float) -> float:
    if not values:
        return float("nan")
    ordered = sorted(values)
    index = min(len(ordered) - 1, int(round(pct * (len(ordered) - 1))))
    return ordered[index]


def _group_key(entry: dict[str, Any], group: str | None) -> tuple[str, ...]:
    if not group:
        return ("all",)
    parts = group.split(",")
    return tuple(str(entry.get(p.strip(), "unknown")) for p in parts if p.strip())


def summarize(path: Path, group: str | None) -> dict[str, Any]:
    entries = _load(path)
    buckets: dict[tuple[str, ...], list[dict[str, Any]]] = {}
    for entry in entries:
        buckets.setdefault(_group_key(entry, group), []).append(entry)

    result: dict[str, Any] = {"total_entries": len(entries), "file": str(path)}
    rows: list[dict[str, Any]] = []
    for key in sorted(buckets):
        bucket = buckets[key]
        e2el = [float(e.get("e2el_s", 0)) for e in bucket]
        output_tput = [float(e.get("output_tput", 0)) for e in bucket]
        total_tput = [float(e.get("total_tput", 0)) for e in bucket]
        output_tokens = [float(e.get("output_tokens", 0)) for e in bucket]
        ok_count = sum(1 for e in bucket if e.get("ok") is not False)
        rows.append(
            {
                "group": key,
                "count": len(bucket),
                "ok_count": ok_count,
                "e2el_mean_s": statistics.mean(e2el) if e2el else float("nan"),
                "e2el_median_s": statistics.median(e2el) if e2el else float("nan"),
                "e2el_p99_s": _pct(e2el, _P99),
                "output_tput_mean": statistics.mean(output_tput) if output_tput else float("nan"),
                "total_tput_mean": statistics.mean(total_tput) if total_tput else float("nan"),
                "output_tokens_total": sum(output_tokens),
            }
        )
    result["groups"] = rows
    return result


def main() -> None:
    parser = argparse.ArgumentParser(description="Summarize collected LLM metrics from JSONL")
    parser.add_argument(
        "--file",
        default=None,
        help=f"path to metrics file (default: <metrics_dir>/{_FILENAME})",
    )
    parser.add_argument(
        "--group",
        default=None,
        help="comma-separated fields to group by, e.g. source,model",
    )
    args = parser.parse_args()

    path = Path(args.file) if args.file else Path(get_settings().metrics_dir) / _FILENAME
    result = summarize(path, args.group)
    print(json.dumps(result, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
