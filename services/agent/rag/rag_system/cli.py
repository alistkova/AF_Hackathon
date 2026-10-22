#!/usr/bin/env python3
"""CLI for the RAG system."""
import sys
import argparse
import json
from pathlib import Path

from .pipeline import build_index, incremental_update, query, get_stats


def main():
    parser = argparse.ArgumentParser(description="RAG system for Russian news PDFs")
    subparsers = parser.add_subparsers(dest="command", required=True)

    # Build index
    build_parser = subparsers.add_parser("build", help="Build or rebuild the full index")
    build_parser.add_argument("--force", action="store_true", help="Force rebuild even if index exists")

    # Incremental update
    subparsers.add_parser("update", help="Incrementally update index with new PDFs")

    # Query
    query_parser = subparsers.add_parser("query", help="Query the RAG system")
    query_parser.add_argument("question", type=str, help="Question to ask")
    query_parser.add_argument("--model", default="qwen", choices=["gemma", "gpt-5.6-sol", "qwen", "deepseek", "qwen_vl", "kimi"], help="LLM model to use")
    query_parser.add_argument("--k", type=int, default=5, help="Number of chunks to retrieve")
    query_parser.add_argument("--temp", type=float, default=0.3, help="Temperature")
    query_parser.add_argument("--stream", action="store_true", help="Stream the response")

    # Stats
    subparsers.add_parser("stats", help="Show system statistics")

    # List files
    subparsers.add_parser("list", help="List processed files")

    args = parser.parse_args()

    if args.command == "build":
        result = build_index(force_rebuild=args.force)
        print(json.dumps(result, ensure_ascii=False, indent=2))

    elif args.command == "update":
        result = incremental_update()
        print(json.dumps(result, ensure_ascii=False, indent=2))

    elif args.command == "query":
        if args.stream:
            for chunk in query(args.question, args.model, args.k, args.temp, stream=True):
                print(chunk, end="", flush=True)
            print()
        else:
            result = query(args.question, args.model, args.k, args.temp, stream=False)
            print(json.dumps(result, ensure_ascii=False, indent=2))

    elif args.command == "stats":
        stats = get_stats()
        print(json.dumps(stats, ensure_ascii=False, indent=2))

    elif args.command == "list":
        stats = get_stats()
        for fname, info in stats.get("files", {}).items():
            print(f"{fname}: {info['pages']} pages, processed {info['processed_at']}")


if __name__ == "__main__":
    main()
