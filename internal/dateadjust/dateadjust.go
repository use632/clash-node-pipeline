// Package dateadjust rewrites date-like substrings inside subscription URLs to
// the current date. Many free subscription links embed the publish date in the
// path or file name (for example
// https://node.example.com/uploads/2026/06/0-20260607.txt where "2026/06" and
// "20260607" are dates). Once a user saves such a link, the program can shift
// every embedded date to "today" on the next run, so the link keeps working
// without manual editing.
//
// This is distinct from the explicit "{date}" placeholder mechanism: it works
// on real, already-dated URLs by auto-detecting the date pattern.
package dateadjust

import (
	"regexp"
	"strings"
	"time"
)

// rule matches one date shape. The regexp matches only the date core (no
// surrounding boundary characters); apply() checks digit boundaries manually so
// a date is never rewritten when it is part of a longer number.
type rule struct {
	re    *regexp.Regexp
	build func(now time.Time, sub []string) string
}

// rules are tried in order, most specific (longest) first, so a full Y-M-D is
// consumed before a Y-M prefix of it can be. Years are constrained to 20xx and
// months/days to valid ranges to avoid rewriting unrelated numbers.
var rules = []rule{
	// 1) YYYY<sep>MM<sep>DD with separators / - _ . (separators preserved).
	{
		re: regexp.MustCompile(`(20[0-9]{2})([/_.-])(0[1-9]|1[0-2])([/_.-])(0[1-9]|[12][0-9]|3[01])`),
		build: func(now time.Time, s []string) string {
			return now.Format("2006") + s[2] + now.Format("01") + s[4] + now.Format("02")
		},
	},
	// 2) YYYYMMDD compact (exactly 8 digits).
	{
		re: regexp.MustCompile(`20[0-9]{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12][0-9]|3[01])`),
		build: func(now time.Time, _ []string) string {
			return now.Format("20060102")
		},
	},
	// 3) YYYY<sep>MM year-month only (separator preserved).
	{
		re: regexp.MustCompile(`(20[0-9]{2})([/_.-])(0[1-9]|1[0-2])`),
		build: func(now time.Time, s []string) string {
			return now.Format("2006") + s[2] + now.Format("01")
		},
	},
}

// Rewrite returns raw with every detected date substring shifted to now. If no
// date pattern is found the input is returned unchanged. The operation is
// idempotent: rewriting an already-current URL yields the same URL.
func Rewrite(raw string, now time.Time) string {
	for _, rl := range rules {
		raw = rl.apply(raw, now)
	}
	return raw
}

// Changed reports whether Rewrite would modify raw for the given date.
func Changed(raw string, now time.Time) bool {
	return Rewrite(raw, now) != raw
}

func (rl rule) apply(s string, now time.Time) string {
	locs := rl.re.FindAllStringSubmatchIndex(s, -1)
	if len(locs) == 0 {
		return s
	}
	var b strings.Builder
	last := 0
	for _, loc := range locs {
		start, end := loc[0], loc[1]
		// Skip if the match sits inside a longer run of digits, so e.g. the
		// "20260607" in a 14-digit timestamp is left alone.
		if start > 0 && isDigit(s[start-1]) {
			continue
		}
		if end < len(s) && isDigit(s[end]) {
			continue
		}
		sub := make([]string, len(loc)/2)
		for i := range sub {
			if loc[2*i] >= 0 {
				sub[i] = s[loc[2*i]:loc[2*i+1]]
			}
		}
		b.WriteString(s[last:start])
		b.WriteString(rl.build(now, sub))
		last = end
	}
	b.WriteString(s[last:])
	return b.String()
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }
