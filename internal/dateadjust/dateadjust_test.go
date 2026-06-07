package dateadjust

import (
	"testing"
	"time"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("bad test date %q: %v", s, err)
	}
	return d
}

func TestRewriteClashnodeExample(t *testing.T) {
	now := mustDate(t, "2026-06-09")
	in := "https://node.example.com/uploads/2026/06/0-20260607.txt"
	want := "https://node.example.com/uploads/2026/06/0-20260609.txt"
	if got := Rewrite(in, now); got != want {
		t.Fatalf("Rewrite()=%q want %q", got, want)
	}
}

func TestRewriteShiftsMonthAndDay(t *testing.T) {
	now := mustDate(t, "2026-07-03")
	in := "https://node.example.com/uploads/2026/06/0-20260607.txt"
	want := "https://node.example.com/uploads/2026/07/0-20260703.txt"
	if got := Rewrite(in, now); got != want {
		t.Fatalf("Rewrite()=%q want %q", got, want)
	}
}

func TestRewriteIdempotent(t *testing.T) {
	now := mustDate(t, "2026-06-07")
	in := "https://node.example.com/uploads/2026/06/0-20260607.txt"
	once := Rewrite(in, now)
	if once != in {
		t.Fatalf("rewriting an already-current URL changed it: %q", once)
	}
	if twice := Rewrite(once, now); twice != once {
		t.Fatalf("Rewrite not idempotent: %q vs %q", twice, once)
	}
}

func TestRewriteDashedDate(t *testing.T) {
	now := mustDate(t, "2026-12-25")
	in := "https://example.com/sub/2026-06-07/list.txt"
	want := "https://example.com/sub/2026-12-25/list.txt"
	if got := Rewrite(in, now); got != want {
		t.Fatalf("Rewrite()=%q want %q", got, want)
	}
}

func TestRewriteNoDateUnchanged(t *testing.T) {
	now := mustDate(t, "2026-06-07")
	for _, in := range []string{
		"https://example.com/sub/xingshuo/mihomo.yaml",
		"testdata/mixed-sources.txt",
		"https://example.com/v1/2/3.txt",
	} {
		if got := Rewrite(in, now); got != in {
			t.Fatalf("Rewrite(%q)=%q, expected unchanged", in, got)
		}
	}
}

func TestRewriteLeavesPlaceholderAlone(t *testing.T) {
	now := mustDate(t, "2026-06-07")
	in := "https://example.com/sub/{date}"
	if got := Rewrite(in, now); got != in {
		t.Fatalf("{date} placeholder must be left for expandSources: got %q", got)
	}
}

func TestRewriteDoesNotTouchLongerDigitRun(t *testing.T) {
	now := mustDate(t, "2026-06-09")
	// A 14-digit timestamp contains 20260607 but must not be rewritten.
	in := "https://example.com/x/20260607123456/a.txt"
	if got := Rewrite(in, now); got != in {
		t.Fatalf("longer digit run was modified: %q", got)
	}
}

func TestChanged(t *testing.T) {
	now := mustDate(t, "2026-06-09")
	if !Changed("https://x/2026/06/0-20260607.txt", now) {
		t.Fatal("Changed should be true for an outdated URL")
	}
	if Changed("https://x/no-date.txt", now) {
		t.Fatal("Changed should be false when there is no date")
	}
}
