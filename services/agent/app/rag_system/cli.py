#!/usr/bin/env python3
"""CLI for the RAG system."""

import argparse
import json

from .config import DATA_DIR, INDEX_DIR
from .pipeline import build_index, index_stats, list_chunks, query_index, search_chunks, update_index


def main() -> None:
    parser = argparse.ArgumentParser(description="RAG system for Russian news PDFs")
    subparsers = parser.add_subparsers(dest="command", required=True)

    common = argparse.ArgumentParser(add_help=False)
    common.add_argument("--data-dir", default=str(DATA_DIR), help="Directory with PDF files")
    common.add_argument("--index-dir", default=str(INDEX_DIR), help="Directory for the RAG index")

    build_parser = subparsers.add_parser(
        "build", parents=[common], help="Build or rebuild the full index"
    )
    build_parser.add_argument(
        "--force", action="store_true", help="Force rebuild even if index exists"
    )

    subparsers.add_parser(
        "update", parents=[common], help="Incrementally update index with new PDFs"
    )

    query_parser = subparsers.add_parser("query", parents=[common], help="Query the RAG system")
    query_parser.add_argument("question", type=str, help="Question to ask")
    query_parser.add_argument(
        "--model",
        default="qwen",
        choices=["gpt-6-astra", "gpt-6-astra", "qwen", "deepseek", "qwen_vl", "kimi"],
        help="LLM model to use",
    )
    query_parser.add_argument("--k", type=int, default=5, help="Number of chunks to retrieve")
    query_parser.add_argument("--temp", type=float, default=0.3, help="Temperature")
    query_parser.add_argument("--stream", action="store_true", help="Stream the response")
    query_parser.add_argument("--company-id", type=str, help="Filter by company ID")
    query_parser.add_argument("--sector", type=str, help="Filter by sector")
    query_parser.add_argument("--after", type=str, help="Filter by published_after (YYYY-MM-DD)")
    query_parser.add_argument("--before", type=str, help="Filter by published_before (YYYY-MM-DD)")

    search_parser = subparsers.add_parser("search", parents=[common], help="Search chunks with filters (no LLM)")
    search_parser.add_argument("query", type=str, help="Search query")
    search_parser.add_argument("--k", type=int, default=10, help="Number of chunks to retrieve")
    search_parser.add_argument("--company-id", type=str, help="Filter by company ID")
    search_parser.add_argument("--sector", type=str, help="Filter by sector")
    search_parser.add_argument("--after", type=str, help="Filter by published_after (YYYY-MM-DD)")
    search_parser.add_argument("--before", type=str, help="Filter by published_before (YYYY-MM-DD)")

    list_parser = subparsers.add_parser("list-chunks", parents=[common], help="List chunks by metadata only")
    list_parser.add_argument("--k", type=int, default=100, help="Number of chunks to retrieve")
    list_parser.add_argument("--company-id", type=str, help="Filter by company ID")
    list_parser.add_argument("--sector", type=str, help="Filter by sector")
    list_parser.add_argument("--after", type=str, help="Filter by published_after (YYYY-MM-DD)")
    list_parser.add_argument("--before", type=str, help="Filter by published_before (YYYY-MM-DD)")

    subparsers.add_parser("stats", parents=[common], help="Show system statistics")
    subparsers.add_parser("list", parents=[common], help="List processed files")

    watch_parser = subparsers.add_parser(
        "watch", parents=[common], help="Watch data directory for new PDFs"
    )
    watch_parser.add_argument("--debounce", type=float, default=2.0, help="Debounce seconds")

    args = parser.parse_args()

    if args.command == "build":
        result = build_index(
            data_dir=args.data_dir,
            index_dir=args.index_dir,
            force_rebuild=args.force,
        )
        print(json.dumps(result, ensure_ascii=False, indent=2))
    elif args.command == "update":
        result = update_index(data_dir=args.data_dir, index_dir=args.index_dir)
        print(json.dumps(result, ensure_ascii=False, indent=2))
    elif args.command == "query":
        if args.stream:
            for chunk in query_index(
                args.question,
                index_dir=args.index_dir,
                model=args.model,
                k=args.k,
                temperature=args.temp,
                stream=True,
                company_id=args.company_id,
                sector=args.sector,
                published_after=args.after,
                published_before=args.before,
            ):
                print(chunk, end="", flush=True)
            print()
        else:
            result = query_index(
                args.question,
                index_dir=args.index_dir,
                model=args.model,
                k=args.k,
                temperature=args.temp,
                company_id=args.company_id,
                sector=args.sector,
                published_after=args.after,
                published_before=args.before,
            )
            print(json.dumps(result, ensure_ascii=False, indent=2))
    elif args.command == "search":
        result = search_chunks(
            args.query,
            index_dir=args.index_dir,
            k=args.k,
            company_id=args.company_id,
            sector=args.sector,
            published_after=args.after,
            published_before=args.before,
        )
        print(json.dumps(result, ensure_ascii=False, indent=2))
    elif args.command == "list-chunks":
        result = list_chunks(
            index_dir=args.index_dir,
            k=args.k,
            company_id=args.company_id,
            sector=args.sector,
            published_after=args.after,
            published_before=args.before,
        )
        print(json.dumps(result, ensure_ascii=False, indent=2))
    elif args.command == "stats":
        stats = index_stats(index_dir=args.index_dir)
        print(json.dumps(stats, ensure_ascii=False, indent=2))
    elif args.command == "list":
        stats = index_stats(index_dir=args.index_dir)
        for fname, info in stats.get("files", {}).items():
            print(f"{fname}: {info['pages']} pages, processed {info['processed_at']}")
    elif args.command == "watch":
        from .watch import watch_directory

        watch_directory(
            data_dir=args.data_dir,
            index_dir=args.index_dir,
            debounce=args.debounce,
        )


if __name__ == "__main__":
    main()
