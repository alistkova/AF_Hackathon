#!/usr/bin/env python3
"""File watcher for automatic incremental updates."""

import threading
import time
from pathlib import Path

from watchdog.events import FileSystemEventHandler
from watchdog.observers import Observer

from .config import DATA_DIR, INDEX_DIR
from .pipeline import update_index


class PDFHandler(FileSystemEventHandler):
    def __init__(self, callback, debounce=2.0):
        self.callback = callback
        self.debounce = debounce
        self.last_event = 0
        self.timer = None

    def on_created(self, event):
        if event.is_directory:
            return
        if Path(event.src_path).suffix.lower() == ".pdf":
            self._trigger()

    def on_modified(self, event):
        if event.is_directory:
            return
        if Path(event.src_path).suffix.lower() == ".pdf":
            self._trigger()

    def _trigger(self):
        now = time.time()
        if now - self.last_event < self.debounce:
            return
        self.last_event = now
        if self.timer:
            self.timer.cancel()
        self.timer = threading.Timer(self.debounce, self.callback)
        self.timer.start()


def watch_directory(
    callback=None,
    data_dir: str | Path = DATA_DIR,
    index_dir: str | Path = INDEX_DIR,
    debounce: float = 2.0,
) -> None:
    """Start watching the data directory for new/modified PDFs."""
    if callback is None:

        def _default_callback() -> None:
            update_index(data_dir=data_dir, index_dir=index_dir)

        callback = _default_callback

    data_path = Path(data_dir)
    data_path.mkdir(parents=True, exist_ok=True)

    event_handler = PDFHandler(callback, debounce)
    observer = Observer()
    observer.schedule(event_handler, str(data_path), recursive=False)
    observer.start()

    print(f"Watching {data_dir} for PDF changes...")
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        observer.stop()
    observer.join()


if __name__ == "__main__":
    watch_directory()
