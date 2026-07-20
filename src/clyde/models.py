from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path


@dataclass(frozen=True)
class FileRecord:
    path: Path
    rel_path: str
    size: int
    sha256: str
    text: str


@dataclass(frozen=True)
class SkipRecord:
    rel_path: str
    reason: str


@dataclass(frozen=True)
class ChunkRecord:
    rel_path: str
    sha256: str
    index: int
    total: int
    text: str


@dataclass
class ScanResult:
    repo: Path
    files: list[FileRecord] = field(default_factory=list)
    skips: list[SkipRecord] = field(default_factory=list)

    @property
    def total_bytes(self) -> int:
        return sum(item.size for item in self.files)
