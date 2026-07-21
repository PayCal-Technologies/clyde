from datetime import datetime, timezone

import pytest

from clyde.book import BookPlan


def test_book_plan_uses_date_hour_minute_subject() -> None:
    plan = BookPlan.create(
        "  Clyde self feedback  ",
        now=datetime(2026, 7, 21, 14, 35, tzinfo=timezone.utc),
    )

    assert plan.title == "2026-07-21 1435 - Clyde self feedback"
    assert plan.slug == "20260721-1435-clyde-self-feedback"
    assert plan.source_prefix == "2026-07-21 1435 - Clyde self feedback :: "


def test_book_plan_rejects_empty_subject() -> None:
    with pytest.raises(ValueError, match="subject"):
        BookPlan.create("   ")


def test_book_plan_can_use_exact_existing_title() -> None:
    plan = BookPlan.from_title("2026-07-21 1030 - Clyde self feedback")

    assert plan.title == "2026-07-21 1030 - Clyde self feedback"
    assert plan.source_prefix == "2026-07-21 1030 - Clyde self feedback :: "
    assert plan.slug == "20260721-1030-clyde-self-feedback"
