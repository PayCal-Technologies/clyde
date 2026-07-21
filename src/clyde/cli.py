from __future__ import annotations

import argparse
import json
import shlex
import sys
import time
from collections import Counter
from pathlib import Path

from .book import BookPlan
from .bundler import make_chunks, write_bundle
from .daemon import DEFAULT_HOST, DEFAULT_PORT, rpc, serve, status_url
from .notebooklm import sync_chunks
from .scanner import scan_repo
from .status import ConsoleProgressSink, HTTPProgressSink, TeeProgressSink


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        return args.func(args)
    except Exception as exc:
        print(f"clyde: error: {exc}", file=sys.stderr)
        return 1


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="clyde")
    subparsers = parser.add_subparsers(required=True)

    preview = subparsers.add_parser("preview", help="show what would be bundled")
    add_scan_args(preview)
    preview.add_argument(
        "--show-files",
        type=_non_negative_int,
        default=20,
        metavar="N",
        help="show the first N included files (default: 20, 0 disables)",
    )
    preview.add_argument(
        "--show-skips",
        type=_non_negative_int,
        default=50,
        metavar="N",
        help="show the first N skipped files (default: 50, 0 disables)",
    )
    preview.set_defaults(func=cmd_preview)

    bundle = subparsers.add_parser("bundle", help="write manifest.json and chunks.jsonl")
    add_scan_args(bundle)
    add_book_args(bundle)
    bundle.add_argument(
        "--out",
        type=Path,
        default=Path(".clyde/out"),
        metavar="DIR",
        help="directory for manifest.json and chunks.jsonl (default: .clyde/out)",
    )
    bundle.set_defaults(func=cmd_bundle)

    sync = subparsers.add_parser("sync", help="upload chunks to NotebookLM through MCP stdio")
    add_scan_args(sync)
    add_book_args(sync)
    target = sync.add_mutually_exclusive_group(required=True)
    target.add_argument("--notebook-id")
    target.add_argument("--notebook-url")
    sync.add_argument("--approve-upload", action="store_true")
    sync.add_argument("--mcp-command", default="npx -y notebooklm-mcp@2.0.0")
    sync.add_argument(
        "--mcp-timeout",
        type=_positive_float,
        default=120.0,
        metavar="SECONDS",
        help="seconds to wait for each MCP response (default: 120)",
    )
    sync.add_argument(
        "--heartbeat-interval",
        type=_positive_float,
        default=1.0,
        metavar="SECONDS",
        help="seconds between progress heartbeats during slow MCP calls (default: 1)",
    )
    sync.add_argument(
        "--status-url",
        metavar="URL",
        help="optional Clyde daemon JSON-RPC URL for sync progress events",
    )
    sync.add_argument(
        "--quiet-progress",
        action="store_true",
        help="suppress real-time sync progress lines",
    )
    sync.add_argument(
        "--job-id",
        default="sync",
        help="status daemon job id for this sync (default: sync)",
    )
    sync.set_defaults(func=cmd_sync)

    daemon = subparsers.add_parser("daemon", help="run a localhost JSON-RPC status daemon")
    daemon.add_argument("--host", default=DEFAULT_HOST)
    daemon.add_argument("--port", type=_positive_int, default=DEFAULT_PORT)
    daemon.set_defaults(func=cmd_daemon)

    status = subparsers.add_parser("status", help="read status from a Clyde daemon")
    status.add_argument("--host", default=DEFAULT_HOST)
    status.add_argument("--port", type=_positive_int, default=DEFAULT_PORT)
    status.add_argument("--job-id")
    status.add_argument("--json", action="store_true", help="print raw JSON")
    status.add_argument("--watch", action="store_true", help="poll status until the job finishes")
    status.add_argument("--interval", type=_positive_float, default=1.0)
    status.set_defaults(func=cmd_status)

    book = subparsers.add_parser("book", help="print Clyde's dated NotebookLM book name")
    book.add_argument("subject", nargs="+", help="subject words for the notebook title")
    book.set_defaults(func=cmd_book)
    return parser


def add_scan_args(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("repo", type=Path, metavar="REPO", help="repository directory to scan")
    parser.add_argument(
        "--include",
        action="append",
        default=[],
        type=_glob_pattern,
        metavar="GLOB",
        help="only include paths matching this glob; repeatable",
    )
    parser.add_argument(
        "--exclude",
        action="append",
        default=[],
        type=_glob_pattern,
        metavar="GLOB",
        help="skip paths matching this glob in addition to Clyde defaults; repeatable",
    )
    parser.add_argument(
        "--max-file-bytes",
        type=_positive_int,
        default=250_000,
        metavar="BYTES",
        help="skip files larger than this many bytes (default: 250000)",
    )
    parser.add_argument(
        "--max-chunk-chars",
        type=_positive_int,
        default=18_000,
        metavar="CHARS",
        help="split uploaded source text at this many characters (default: 18000)",
    )


def add_book_args(parser: argparse.ArgumentParser) -> None:
    group = parser.add_mutually_exclusive_group()
    group.add_argument(
        "--subject",
        metavar="TEXT",
        help="generate a dated NotebookLM book title for this subject",
    )
    group.add_argument(
        "--book-title",
        metavar="TEXT",
        help="use an exact NotebookLM book title created earlier",
    )


def cmd_preview(args: argparse.Namespace) -> int:
    result = scan_repo(
        args.repo,
        include=args.include,
        exclude=args.exclude,
        max_file_bytes=args.max_file_bytes,
    )
    chunks = make_chunks(result, max_chunk_chars=args.max_chunk_chars)
    _print_summary(result, len(chunks), args)
    if result.files and args.show_files:
        print(f"\nIncluded files (first {min(args.show_files, len(result.files))}):")
        for item in result.files[: args.show_files]:
            print(f"  {item.rel_path} ({_format_bytes(item.size)})")
        if len(result.files) > args.show_files:
            print(f"  ... {len(result.files) - args.show_files} more")
    if result.skips and args.show_skips:
        print("\nSkipped:")
        for item in result.skips[: args.show_skips]:
            print(f"  {item.rel_path}: {item.reason}")
        if len(result.skips) > args.show_skips:
            print(f"  ... {len(result.skips) - args.show_skips} more")
    if not result.files:
        print("\nNo files matched. Check --include/--exclude or the repo path.")
    return 0


def cmd_bundle(args: argparse.Namespace) -> int:
    plan = _book_plan(args)
    result = scan_repo(
        args.repo,
        include=args.include,
        exclude=args.exclude,
        max_file_bytes=args.max_file_bytes,
    )
    if args.out.exists() and not args.out.is_dir():
        raise ValueError(f"--out must be a directory, not a file: {args.out}")
    manifest = write_bundle(
        result,
        args.out,
        max_chunk_chars=args.max_chunk_chars,
        book_title=plan.title if plan else None,
        book_slug=plan.slug if plan else None,
    )
    _print_summary(result, manifest["chunk_count"], args)
    if plan:
        _print_book_plan(plan)
    print(f"\nWrote: {args.out / 'manifest.json'}")
    print(f"Wrote: {args.out / 'chunks.jsonl'}")
    print("Review manifest.json before running sync.")
    return 0


def cmd_sync(args: argparse.Namespace) -> int:
    if not args.approve_upload:
        raise ValueError("sync requires --approve-upload")
    plan = _book_plan(args)
    result = scan_repo(
        args.repo,
        include=args.include,
        exclude=args.exclude,
        max_file_bytes=args.max_file_bytes,
    )
    chunks = make_chunks(
        result,
        max_chunk_chars=args.max_chunk_chars,
        book_title=plan.title if plan else None,
    )
    _print_summary(result, len(chunks), args)
    if plan:
        _print_book_plan(plan)
    command = shlex.split(args.mcp_command)
    count = sync_chunks(
        chunks,
        notebook_id=args.notebook_id,
        notebook_url=args.notebook_url,
        command=command,
        request_timeout=args.mcp_timeout,
        heartbeat_interval=args.heartbeat_interval,
        progress=_progress_sink(args),
        job_id=args.job_id,
        title_prefix=plan.source_prefix if plan else "",
    )
    target = args.notebook_id or args.notebook_url
    print(f"\nUploaded {count} chunks to notebook {target}.")
    return 0


def cmd_daemon(args: argparse.Namespace) -> int:
    serve(args.host, args.port)
    return 0


def cmd_status(args: argparse.Namespace) -> int:
    url = status_url(args)
    seen = None
    while True:
        result = rpc(url, "status.get", {"job_id": args.job_id} if args.job_id else {})
        if args.json:
            rendered = json.dumps(result, indent=2, sort_keys=True)
        else:
            rendered = _format_status(result)
        if rendered != seen:
            print(rendered)
            seen = rendered
        if not args.watch or _is_terminal_status(result):
            return 0
        time.sleep(args.interval)


def cmd_book(args: argparse.Namespace) -> int:
    plan = BookPlan.create(" ".join(args.subject))
    _print_book_plan(plan)
    return 0


def _book_plan(args: argparse.Namespace) -> BookPlan | None:
    subject = getattr(args, "subject", None)
    book_title = getattr(args, "book_title", None)
    if book_title:
        return BookPlan.from_title(book_title)
    return BookPlan.create(subject) if subject else None


def _print_book_plan(plan: BookPlan) -> None:
    print(f"Book title: {plan.title}")
    print(f"Book slug: {plan.slug}")


def _progress_sink(args: argparse.Namespace):
    sinks = []
    if not args.quiet_progress:
        sinks.append(ConsoleProgressSink())
    if args.status_url:
        sinks.append(HTTPProgressSink(args.status_url))
    if not sinks:
        return None
    return TeeProgressSink(sinks, ignore_errors=True)


def _print_summary(result, chunk_count: int, args: argparse.Namespace) -> None:
    print(f"Repo: {result.repo}")
    print(f"Included files: {len(result.files)}")
    print(f"Skipped files: {len(result.skips)}")
    print(f"Total included bytes: {result.total_bytes} ({_format_bytes(result.total_bytes)})")
    print(f"Chunks: {chunk_count}")
    print(f"Max file size: {args.max_file_bytes} bytes ({_format_bytes(args.max_file_bytes)})")
    print(f"Max chunk size: {args.max_chunk_chars} chars")
    if args.include:
        print(f"Include globs: {', '.join(args.include)}")
    if args.exclude:
        print(f"Extra exclude globs: {', '.join(args.exclude)}")
    if result.skips:
        reasons = Counter(item.reason for item in result.skips)
        print("Skip reasons:")
        for reason, count in reasons.most_common():
            print(f"  {reason}: {count}")


def _positive_int(value: str) -> int:
    try:
        parsed = int(value, 10)
    except ValueError as exc:
        raise argparse.ArgumentTypeError("must be an integer") from exc
    if parsed <= 0:
        raise argparse.ArgumentTypeError("must be greater than 0")
    return parsed


def _positive_float(value: str) -> float:
    try:
        parsed = float(value)
    except ValueError as exc:
        raise argparse.ArgumentTypeError("must be a number") from exc
    if parsed <= 0:
        raise argparse.ArgumentTypeError("must be greater than 0")
    return parsed


def _non_negative_int(value: str) -> int:
    try:
        parsed = int(value, 10)
    except ValueError as exc:
        raise argparse.ArgumentTypeError("must be an integer") from exc
    if parsed < 0:
        raise argparse.ArgumentTypeError("must be 0 or greater")
    return parsed


def _glob_pattern(value: str) -> str:
    if not value.strip():
        raise argparse.ArgumentTypeError("must not be empty")
    return value


def _format_bytes(size: int) -> str:
    units = ["B", "KiB", "MiB", "GiB"]
    amount = float(size)
    for unit in units:
        if amount < 1024 or unit == units[-1]:
            if unit == "B":
                return f"{size} {unit}"
            return f"{amount:.1f} {unit}"
        amount /= 1024


def _format_status(result: dict) -> str:
    if "job" in result:
        job = result["job"]
        if job is None:
            return "No matching job."
        return _format_job(job)
    jobs = result.get("jobs", [])
    if not jobs:
        return "No jobs."
    return "\n".join(_format_job(job) for job in jobs)


def _format_job(job: dict) -> str:
    total = job.get("total") or 0
    done = job.get("done") or 0
    progress = f"{done}/{total}" if total else str(done)
    line = f"{job.get('job_id')}: {job.get('phase')} {progress} - {job.get('message')}"
    if job.get("error"):
        line += f" ({job['error']})"
    return line


def _is_terminal_status(result: dict) -> bool:
    jobs = [result.get("job")] if "job" in result else result.get("jobs", [])
    jobs = [job for job in jobs if job]
    return bool(jobs) and all(job.get("phase") in {"complete", "failed"} for job in jobs)


if __name__ == "__main__":
    raise SystemExit(main())
