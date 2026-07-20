from __future__ import annotations

import argparse
import shlex
import sys
from pathlib import Path

from .bundler import make_chunks, write_bundle
from .notebooklm import sync_chunks
from .scanner import scan_repo


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
    preview.set_defaults(func=cmd_preview)

    bundle = subparsers.add_parser("bundle", help="write manifest.json and chunks.jsonl")
    add_scan_args(bundle)
    bundle.add_argument("--out", type=Path, default=Path(".clyde/out"))
    bundle.set_defaults(func=cmd_bundle)

    sync = subparsers.add_parser("sync", help="upload chunks to NotebookLM through MCP stdio")
    add_scan_args(sync)
    sync.add_argument("--notebook-id", required=True)
    sync.add_argument("--approve-upload", action="store_true")
    sync.add_argument("--mcp-command", default="npx -y notebooklm-mcp@2.0.0")
    sync.set_defaults(func=cmd_sync)
    return parser


def add_scan_args(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("repo", type=Path)
    parser.add_argument("--include", action="append", default=[])
    parser.add_argument("--exclude", action="append", default=[])
    parser.add_argument("--max-file-bytes", type=int, default=250_000)
    parser.add_argument("--max-chunk-chars", type=int, default=18_000)


def cmd_preview(args: argparse.Namespace) -> int:
    result = scan_repo(
        args.repo,
        include=args.include,
        exclude=args.exclude,
        max_file_bytes=args.max_file_bytes,
    )
    chunks = make_chunks(result, max_chunk_chars=args.max_chunk_chars)
    _print_summary(result, len(chunks))
    if result.skips:
        print("\nSkipped:")
        for item in result.skips[:50]:
            print(f"  {item.rel_path}: {item.reason}")
        if len(result.skips) > 50:
            print(f"  ... {len(result.skips) - 50} more")
    return 0


def cmd_bundle(args: argparse.Namespace) -> int:
    result = scan_repo(
        args.repo,
        include=args.include,
        exclude=args.exclude,
        max_file_bytes=args.max_file_bytes,
    )
    manifest = write_bundle(result, args.out, max_chunk_chars=args.max_chunk_chars)
    _print_summary(result, manifest["chunk_count"])
    print(f"\nWrote: {args.out / 'manifest.json'}")
    print(f"Wrote: {args.out / 'chunks.jsonl'}")
    return 0


def cmd_sync(args: argparse.Namespace) -> int:
    if not args.approve_upload:
        raise ValueError("sync requires --approve-upload")
    result = scan_repo(
        args.repo,
        include=args.include,
        exclude=args.exclude,
        max_file_bytes=args.max_file_bytes,
    )
    chunks = make_chunks(result, max_chunk_chars=args.max_chunk_chars)
    _print_summary(result, len(chunks))
    command = shlex.split(args.mcp_command)
    count = sync_chunks(chunks, notebook_id=args.notebook_id, command=command)
    print(f"\nUploaded {count} chunks to notebook {args.notebook_id}.")
    return 0


def _print_summary(result, chunk_count: int) -> None:
    print(f"Repo: {result.repo}")
    print(f"Included files: {len(result.files)}")
    print(f"Skipped files: {len(result.skips)}")
    print(f"Total included bytes: {result.total_bytes}")
    print(f"Chunks: {chunk_count}")


if __name__ == "__main__":
    raise SystemExit(main())
