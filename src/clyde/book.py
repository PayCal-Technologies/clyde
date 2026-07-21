from __future__ import annotations

import re
from dataclasses import dataclass
from datetime import datetime


@dataclass(frozen=True)
class BookPlan:
    subject: str
    timestamp: datetime
    title_override: str | None = None

    @property
    def title(self) -> str:
        if self.title_override:
            return self.title_override
        return f"{self.timestamp.strftime('%Y-%m-%d %H%M')} - {self.subject}"

    @property
    def slug(self) -> str:
        if self.title_override:
            parsed = re.match(
                r"^(?P<date>\d{4})-(?P<month>\d{2})-(?P<day>\d{2}) "
                r"(?P<time>\d{4}) - (?P<subject>.+)$",
                self.title_override,
            )
            if parsed:
                stamp = (
                    f"{parsed.group('date')}{parsed.group('month')}{parsed.group('day')}-"
                    f"{parsed.group('time')}"
                )
                subject = _slug_text(parsed.group("subject"))
                return f"{stamp}-{subject}" if subject else stamp
            return _slug_text(self.title_override)
        stamp = self.timestamp.strftime("%Y%m%d-%H%M")
        subject = _slug_text(self.subject)
        return f"{stamp}-{subject}" if subject else stamp

    @property
    def source_prefix(self) -> str:
        return f"{self.title} :: "

    @classmethod
    def create(cls, subject: str, *, now: datetime | None = None) -> "BookPlan":
        cleaned = " ".join(subject.split())
        if not cleaned:
            raise ValueError("book subject must not be empty")
        timestamp = now or datetime.now().astimezone()
        return cls(subject=cleaned, timestamp=timestamp)

    @classmethod
    def from_title(cls, title: str) -> "BookPlan":
        cleaned = " ".join(title.split())
        if not cleaned:
            raise ValueError("book title must not be empty")
        return cls(subject=cleaned, timestamp=datetime.now().astimezone(), title_override=cleaned)


def _slug_text(value: str) -> str:
    return re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-")
