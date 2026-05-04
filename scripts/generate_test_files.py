#!/usr/bin/env python3
"""Generate structured attachment test files for Relay.

Creates a dataset organized by file type with:
- small and large text files
- small and large PDFs
- small/under-limit/over-limit images for all supported image extensions
- tricky filename variants
- multi-file sets that sit just under and just over total upload limits
"""

from __future__ import annotations

import argparse
import shutil
from pathlib import Path


KB = 1024
MB = 1024 * 1024


# App defaults from example.env + frontend store
DEFAULT_TOTAL_UPLOAD_LIMIT = 30 * MB
DEFAULT_IMAGE_LIMIT = 15 * MB
DEFAULT_SINGLE_FILE_LIMIT = 50 * MB

DEFAULT_SMALL_TEXT_SIZE = 4 * KB
DEFAULT_LARGE_TEXT_SIZE = 5 * MB
DEFAULT_SMALL_IMAGE_SIZE = 64 * KB
DEFAULT_SMALL_PDF_SIZE = 128 * KB
DEFAULT_LARGE_PDF_SIZE = 20 * MB


IMAGE_FORMATS = [
    "png",
    "jpg",
    "jpeg",
    "gif",
    "webp",
    "svg",
    "bmp",
    "ico",
    "avif",
    "tiff",
]

TEXT_FORMATS = ["txt", "md", "markdown", "json", "csv", "xml", "yaml", "yml"]


TYPE_DIRECTORIES = {
    "Markdown": ["md", "markdown"],
    "Text": ["txt"],
    "JSON": ["json"],
    "CSV": ["csv"],
    "XML": ["xml"],
    "YAML": ["yaml", "yml"],
    "PDF": ["pdf"],
    "Images": IMAGE_FORMATS,
    "BatchLimits": [],
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Generate attachment test files organized by file type."
    )
    parser.add_argument(
        "--output",
        type=Path,
        default=Path("scripts/test-files"),
        help="Output directory root (default: scripts/test-files).",
    )
    parser.add_argument(
        "--total-upload-limit",
        type=int,
        default=DEFAULT_TOTAL_UPLOAD_LIMIT,
        help=f"Total upload limit in bytes (default: {DEFAULT_TOTAL_UPLOAD_LIMIT}).",
    )
    parser.add_argument(
        "--image-limit",
        type=int,
        default=DEFAULT_IMAGE_LIMIT,
        help=f"Image size limit in bytes (default: {DEFAULT_IMAGE_LIMIT}).",
    )
    parser.add_argument(
        "--single-file-limit",
        type=int,
        default=DEFAULT_SINGLE_FILE_LIMIT,
        help=f"Single file limit in bytes (default: {DEFAULT_SINGLE_FILE_LIMIT}).",
    )
    parser.add_argument(
        "--clean",
        action="store_true",
        help="Delete output directory before generating files.",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print planned output without writing files.",
    )
    return parser.parse_args()


def ensure_dirs(root: Path) -> None:
    for dirname in TYPE_DIRECTORIES:
        (root / dirname).mkdir(parents=True, exist_ok=True)


def byte_pattern(length: int) -> bytes:
    block = b"RELAY_TEST_DATA_0123456789"
    repeats = (length // len(block)) + 1
    return (block * repeats)[:length]


def write_sized_binary(path: Path, size: int, prefix: bytes) -> None:
    if size < len(prefix):
        raise ValueError(f"Cannot write {path.name}: size {size} < header {len(prefix)}")
    payload_size = size - len(prefix)
    with path.open("wb") as f:
        f.write(prefix)
        if payload_size > 0:
            f.write(byte_pattern(payload_size))


def write_sized_text(path: Path, size: int, template: str) -> None:
    content = []
    current = 0
    while current < size:
        line = template.format(i=len(content))
        current += len(line.encode("utf-8"))
        content.append(line)
    joined = "".join(content).encode("utf-8")[:size]
    with path.open("wb") as f:
        f.write(joined)


def image_header(ext: str) -> bytes:
    headers = {
        "png": b"\x89PNG\r\n\x1a\n",
        "jpg": b"\xff\xd8\xff\xe0" + b"JFIF\x00",
        "jpeg": b"\xff\xd8\xff\xe0" + b"JFIF\x00",
        "gif": b"GIF89a",
        "webp": b"RIFF\x00\x00\x00\x00WEBP",
        "svg": b'<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>',
        "bmp": b"BM",
        "ico": b"\x00\x00\x01\x00",
        "avif": b"\x00\x00\x00 ftypavif",
        "tiff": b"II*\x00",
    }
    return headers[ext]


def write_small_and_large_text_files(root: Path, small: int, large: int) -> list[Path]:
    created: list[Path] = []
    for ext in TEXT_FORMATS:
        dirname = ext_to_dirname(ext)
        folder = root / dirname

        small_name = f"small.{ext}"
        large_name = f"large.{ext}"
        tricky_a = f"Report FINAL (v2).{ext}"
        tricky_b = f"UPPERCASE..NAME.{ext.upper()}"
        tricky_c = f"spaces   inside.{ext}"

        samples = [
            (small_name, small),
            (large_name, large),
            (tricky_a, small),
            (tricky_b, small),
            (tricky_c, small),
        ]

        for filename, size in samples:
            path = folder / filename
            template = text_template_for_extension(ext)
            write_sized_text(path, size, template)
            created.append(path)
    return created


def write_pdf_files(root: Path, small: int, large: int) -> list[Path]:
    created: list[Path] = []
    folder = root / "PDF"
    pdf_prefix = (
        b"%PDF-1.4\n"
        b"1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n"
        b"2 0 obj<</Type/Pages/Count 1/Kids[3 0 R]>>endobj\n"
        b"3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 300 144]>>endobj\n"
        b"trailer<</Root 1 0 R>>\n%%EOF\n"
    )
    for name, size in [
        ("small.pdf", small),
        ("large.pdf", large),
        ("Report FINAL (v2).pdf", small),
        ("UPPERCASE..NAME.PDF", small),
    ]:
        path = folder / name
        write_sized_binary(path, size, pdf_prefix)
        created.append(path)
    return created


def write_image_files(root: Path, small: int, image_limit: int) -> list[Path]:
    created: list[Path] = []
    folder = root / "Images"
    under = max(image_limit - 128, small + 1)
    over = image_limit + 128

    for ext in IMAGE_FORMATS:
        ext_dir = folder / ext.upper()
        ext_dir.mkdir(parents=True, exist_ok=True)
        header = image_header(ext)
        for name, size in [
            (f"small.{ext}", small),
            (f"edge-under-image-limit.{ext}", under),
            (f"edge-over-image-limit.{ext}", over),
            (f"Report FINAL (v2).{ext}", small),
            (f"UPPERCASE..NAME.{ext.upper()}", small),
        ]:
            path = ext_dir / name
            write_sized_binary(path, size, header)
            created.append(path)
    return created


def write_batch_limit_sets(root: Path, total_limit: int) -> list[Path]:
    created: list[Path] = []
    folder = root / "BatchLimits"

    # Three files that sum to (limit - 64 bytes)
    under_sizes = [total_limit // 3, total_limit // 3, total_limit // 3 - 64]
    # Three files that sum to (limit + 64 bytes)
    over_sizes = [total_limit // 3, total_limit // 3, total_limit // 3 + 64]

    for i, size in enumerate(under_sizes, start=1):
        path = folder / f"under-total-limit-{i}.txt"
        write_sized_text(path, size, "under-limit sample line {i}\n")
        created.append(path)
    for i, size in enumerate(over_sizes, start=1):
        path = folder / f"over-total-limit-{i}.txt"
        write_sized_text(path, size, "over-limit sample line {i}\n")
        created.append(path)

    return created


def write_single_file_limit_case(root: Path, single_limit: int) -> Path:
    folder = root / "Text"
    path = folder / "edge-over-single-file-limit.txt"
    write_sized_text(path, single_limit + 128, "single-file-limit edge case {i}\n")
    return path


def ext_to_dirname(ext: str) -> str:
    for dirname, exts in TYPE_DIRECTORIES.items():
        if ext in exts:
            return dirname
    raise ValueError(f"No directory mapping for extension: {ext}")


def text_template_for_extension(ext: str) -> str:
    if ext in {"md", "markdown"}:
        return "# Relay test markdown {i}\n\n- item A\n- item B\n\n"
    if ext == "json":
        return '{{"row": {i}, "name": "sample", "active": true}}\n'
    if ext == "csv":
        return "id,name,value\n{i},sample,42\n"
    if ext == "xml":
        return "<row><id>{i}</id><name>sample</name><value>42</value></row>\n"
    if ext in {"yaml", "yml"}:
        return "row: {i}\nname: sample\nenabled: true\n---\n"
    return "relay plain text sample line {i}\n"


def format_bytes(value: int) -> str:
    if value >= MB:
        return f"{value / MB:.2f}MB"
    if value >= KB:
        return f"{value / KB:.2f}KB"
    return f"{value}B"


def print_summary(files: list[Path], root: Path) -> None:
    total_bytes = sum(p.stat().st_size for p in files if p.exists())
    print(f"Created {len(files)} files in: {root}")
    print(f"Total bytes: {total_bytes} ({format_bytes(total_bytes)})")
    for dirname in TYPE_DIRECTORIES:
        sub = root / dirname
        if not sub.exists():
            continue
        count = sum(1 for p in sub.rglob("*") if p.is_file())
        print(f"  - {dirname}: {count} files")


def main() -> None:
    args = parse_args()
    root = args.output

    if args.clean and root.exists():
        shutil.rmtree(root)

    if args.dry_run:
        print("Dry run: no files written.")
        print(f"Output root: {root}")
        print(f"Total limit: {args.total_upload_limit} ({format_bytes(args.total_upload_limit)})")
        print(f"Image limit: {args.image_limit} ({format_bytes(args.image_limit)})")
        print(
            f"Single-file limit: {args.single_file_limit} "
            f"({format_bytes(args.single_file_limit)})"
        )
        print("Directories:")
        for dirname in TYPE_DIRECTORIES:
            print(f"  - {root / dirname}")
        return

    ensure_dirs(root)
    created: list[Path] = []
    created.extend(
        write_small_and_large_text_files(
            root=root, small=DEFAULT_SMALL_TEXT_SIZE, large=DEFAULT_LARGE_TEXT_SIZE
        )
    )
    created.extend(write_pdf_files(root=root, small=DEFAULT_SMALL_PDF_SIZE, large=DEFAULT_LARGE_PDF_SIZE))
    created.extend(
        write_image_files(
            root=root,
            small=DEFAULT_SMALL_IMAGE_SIZE,
            image_limit=args.image_limit,
        )
    )
    created.extend(write_batch_limit_sets(root=root, total_limit=args.total_upload_limit))
    created.append(write_single_file_limit_case(root=root, single_limit=args.single_file_limit))
    print_summary(created, root)


if __name__ == "__main__":
    main()
