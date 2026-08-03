package clyde

import (
	"testing"
	"time"
)

func TestBookPlan(t *testing.T) {
	plan, err := NewBookPlan("  Clyde self feedback  ", time.Date(2026, 7, 21, 14, 35, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Title() != "2026-07-21 1435 - Clyde self feedback" {
		t.Fatalf("unexpected title: %s", plan.Title())
	}
	if plan.Slug() != "20260721-1435-clyde-self-feedback" {
		t.Fatalf("unexpected slug: %s", plan.Slug())
	}
}

func TestBookPlanFromTitle(t *testing.T) {
	plan, err := BookPlanFromTitle("2026-07-21 1030 - Clyde self feedback")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Slug() != "20260721-1030-clyde-self-feedback" {
		t.Fatalf("unexpected slug: %s", plan.Slug())
	}
}
