package cron

import (
	"testing"
	"time"
)

func TestCronNextRuns(t *testing.T) {
	s, err := Parse("0 9 * * 1-5")
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC) // a Saturday
	got := s.Next(from, 2)
	if len(got) != 2 || got[0] != time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC) || got[1] != time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC) {
		t.Fatalf("next %v", got)
	}
	if _, err := Parse("0 9 * *"); err == nil {
		t.Fatal("four fields parse")
	}
	if got := mustCron(t, "*/15 * * * *").Next(from, 3); got[2] != from.Add(45*time.Minute) {
		t.Fatalf("steps %v", got)
	}
}

func mustCron(t *testing.T, e string) *Spec {
	s, err := Parse(e)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
