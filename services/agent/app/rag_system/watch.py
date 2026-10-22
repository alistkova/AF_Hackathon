#!/usr/bin/env python3
"""File watcher for automatic incremental updates."""

import time
import threading
from pathlib import Path
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler

from .pipeline import incremental_update
from .config import DATA_DIR


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


def watch_directory(callback=None, debounce=2.0):
    """Start watching the data directory for new/modified PDFs."""
    if callback is None:
        callback = incremental_update

    event_handler = PDFHandler(callback, debounce)
    observer = Observer()
    observer.schedule(event_handler, str(DATA_DIR), recursive=False)
    observer.start()

    print(f"Watching {DATA_DIR} for PDF changes...")
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        observer.stop()
    observer.join()


if __name__ == "__main__":
    watch_directory()
