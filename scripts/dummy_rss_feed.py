#!/usr/bin/env python3
"""Serve a dummy RSS feed that publishes test items over time.

Examples:
  scripts/dummy_rss_feed.py
  scripts/dummy_rss_feed.py --interval 10 --items-per-interval 2 --port 8765

Add this URL in Relay's feeds UI:
  http://127.0.0.1:8765/rss.xml
"""

from __future__ import annotations

import argparse
import sys
from datetime import UTC, datetime, timedelta
from email.utils import format_datetime
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Iterable
from urllib.parse import urlparse
from xml.etree import ElementTree


CONTENT_NS = "http://purl.org/rss/1.0/modules/content/"
DC_NS = "http://purl.org/dc/elements/1.1/"
YOUTUBE_URL = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"

ElementTree.register_namespace("content", CONTENT_NS)
ElementTree.register_namespace("dc", DC_NS)


class FeedState:
    def __init__(
        self,
        *,
        public_url: str,
        interval_seconds: int,
        items_per_interval: int,
        max_items: int,
    ) -> None:
        self.public_url = public_url.rstrip("/")
        self.interval_seconds = interval_seconds
        self.items_per_interval = items_per_interval
        self.max_items = max_items
        self.started_at = datetime.now(UTC)
        self.seed_count = 2

    def item_count(self, now: datetime) -> int:
        elapsed = max(0, int((now - self.started_at).total_seconds()))
        published_batches = elapsed // self.interval_seconds
        return self.seed_count + published_batches * self.items_per_interval

    def visible_sequences(self, now: datetime) -> range:
        total = self.item_count(now)
        first = max(0, total - self.max_items)
        return range(total - 1, first - 1, -1)

    def published_at(self, sequence: int) -> datetime:
        if sequence < self.seed_count:
            return self.started_at - timedelta(seconds=(self.seed_count - sequence) * 30)
        batch = ((sequence - self.seed_count) // self.items_per_interval) + 1
        return self.started_at + timedelta(seconds=batch * self.interval_seconds)

    def item_kind(self, sequence: int) -> str:
        return "video" if sequence % 2 == 1 else "text"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Serve a local RSS feed with new text and YouTube items over time."
    )
    parser.add_argument("--host", default="127.0.0.1", help="Bind host. Default: 127.0.0.1")
    parser.add_argument("--port", type=int, default=8765, help="Bind port. Default: 8765")
    parser.add_argument(
        "--interval",
        "--rate",
        type=int,
        default=30,
        help="Seconds between publish batches. --rate is an alias. Default: 30",
    )
    parser.add_argument(
        "--items-per-interval",
        type=int,
        default=1,
        help="New items published per interval. Default: 1",
    )
    parser.add_argument(
        "--max-items",
        type=int,
        default=50,
        help="Maximum items retained in the feed response. Default: 50",
    )
    parser.add_argument(
        "--public-url",
        default="",
        help="Public base URL to write into feed links. Default: derived from host and port.",
    )
    return parser.parse_args()


def validate_args(args: argparse.Namespace) -> None:
    if args.interval <= 0:
        raise ValueError("--interval must be greater than 0")
    if args.items_per_interval <= 0:
        raise ValueError("--items-per-interval must be greater than 0")
    if args.max_items < 2:
        raise ValueError("--max-items must be at least 2 so text and video fixtures are visible")
    if args.port < 0 or args.port > 65535:
        raise ValueError("--port must be between 0 and 65535")


def build_public_url(host: str, port: int, explicit_url: str) -> str:
    if explicit_url:
        parsed = urlparse(explicit_url)
        if parsed.scheme not in {"http", "https"} or not parsed.netloc:
            raise ValueError("--public-url must be an absolute http or https URL")
        return explicit_url.rstrip("/")

    public_host = "127.0.0.1" if host in {"", "0.0.0.0", "::"} else host
    if ":" in public_host and not public_host.startswith("["):
        public_host = f"[{public_host}]"
    return f"http://{public_host}:{port}"


def build_feed_xml(state: FeedState, now: datetime) -> bytes:
    rss = ElementTree.Element("rss", {"version": "2.0"})
    channel = ElementTree.SubElement(rss, "channel")
    add_text(channel, "title", "Relay Dummy RSS Feed")
    add_text(channel, "link", state.public_url)
    add_text(
        channel,
        "description",
        (
            "Local test feed that publishes text articles and YouTube video items "
            f"every {state.interval_seconds} seconds."
        ),
    )
    add_text(channel, "lastBuildDate", format_datetime(now, usegmt=True))
    add_text(channel, "generator", "scripts/dummy_rss_feed.py")

    for sequence in state.visible_sequences(now):
        add_item(channel, state, sequence)

    return ElementTree.tostring(rss, encoding="utf-8", xml_declaration=True)


def add_item(channel: ElementTree.Element, state: FeedState, sequence: int) -> None:
    item = ElementTree.SubElement(channel, "item")
    published_at = state.published_at(sequence)
    kind = state.item_kind(sequence)

    if kind == "video":
        title = f"Dummy video result #{sequence}"
        link = YOUTUBE_URL
        guid = f"relay-dummy-video-{sequence}"
        description = (
            "A video feed item for exercising media handling in the feeds UI. "
            "It intentionally points at the same stable YouTube example URL."
        )
        content = (
            "<p>This item should be detected as a YouTube video by Relay. "
            "Summary controls should be disabled for it.</p>"
        )
    else:
        title = f"Dummy text result #{sequence}"
        link = f"{state.public_url}/items/text-{sequence}"
        guid = f"relay-dummy-text-{sequence}"
        description = (
            "A text feed item for testing polling, unread counts, summaries, "
            "and normal article rendering."
        )
        content = (
            f"<p>This is dummy text result #{sequence}. It exists so feed polling "
            "can discover a fresh article without relying on an external RSS source.</p>"
        )

    add_text(item, "title", title)
    add_text(item, "link", link)
    add_text(item, "guid", guid)
    add_text(item, f"{{{DC_NS}}}creator", "Relay Test Feed")
    add_text(item, "pubDate", format_datetime(published_at, usegmt=True))
    add_text(item, "description", description)
    add_text(item, f"{{{CONTENT_NS}}}encoded", content)


def add_text(parent: ElementTree.Element, tag: str, value: str) -> None:
    child = ElementTree.SubElement(parent, tag)
    child.text = value


def make_handler(state: FeedState) -> type[BaseHTTPRequestHandler]:
    class DummyRSSHandler(BaseHTTPRequestHandler):
        server_version = "RelayDummyRSS/1.0"

        def do_GET(self) -> None:
            self.handle_request(send_body=True)

        def do_HEAD(self) -> None:
            self.handle_request(send_body=False)

        def handle_request(self, *, send_body: bool) -> None:
            path = urlparse(self.path).path

            if path in {"/", "/rss.xml", "/feed.xml"}:
                body = build_feed_xml(state, datetime.now(UTC))
                self.send_response(200)
                self.send_header("Content-Type", "application/rss+xml; charset=utf-8")
                self.send_header("Content-Length", str(len(body)))
                self.send_header("Cache-Control", "no-store")
                self.end_headers()
                if send_body:
                    self.wfile.write(body)
                return

            if path == "/health":
                body = b"ok\n"
                self.send_response(200)
                self.send_header("Content-Type", "text/plain; charset=utf-8")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                if send_body:
                    self.wfile.write(body)
                return

            if path.startswith("/items/"):
                body = f"Dummy article page for {path}\n".encode("utf-8")
                self.send_response(200)
                self.send_header("Content-Type", "text/plain; charset=utf-8")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                if send_body:
                    self.wfile.write(body)
                return

            self.send_error(404, "not found")

        def log_message(self, fmt: str, *args: object) -> None:
            sys.stderr.write(f"[dummy-rss] {self.address_string()} - {fmt % args}\n")

    return DummyRSSHandler


def print_startup(feed_urls: Iterable[str], state: FeedState) -> None:
    print("[dummy-rss] serving Relay dummy RSS feed")
    for url in feed_urls:
        print(f"[dummy-rss] feed URL: {url}")
    print(
        "[dummy-rss] publishing "
        f"{state.items_per_interval} new item(s) every {state.interval_seconds} second(s)"
    )
    print("[dummy-rss] seeded with one text item and one YouTube video item")
    print("[dummy-rss] press Ctrl+C to stop")


def main() -> int:
    args = parse_args()
    try:
        validate_args(args)
        server = ThreadingHTTPServer((args.host, args.port), BaseHTTPRequestHandler)
        actual_host, actual_port = server.server_address[:2]
        public_url = build_public_url(args.host or actual_host, actual_port, args.public_url)
        state = FeedState(
            public_url=public_url,
            interval_seconds=args.interval,
            items_per_interval=args.items_per_interval,
            max_items=args.max_items,
        )
        server.RequestHandlerClass = make_handler(state)
    except OSError as err:
        print(f"[dummy-rss] failed to start server: {err}", file=sys.stderr)
        return 1
    except ValueError as err:
        print(f"[dummy-rss] invalid option: {err}", file=sys.stderr)
        return 2

    print_startup([f"{public_url}/rss.xml", public_url], state)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n[dummy-rss] stopped")
    finally:
        server.server_close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
