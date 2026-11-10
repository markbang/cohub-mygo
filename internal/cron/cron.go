// Package cron reads five-field cron expressions and previews their runs.
package cron

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// Spec is a parsed five-field cron expression: minute, hour, day of
// month, month, day of week, as cron-parser reads them.
type Spec struct {
	min, hour, dom, mon, dow [64]bool
	domAny, dowAny           bool
}

var errCron = errors.New("invalid cron expression")

func parseField(s string, lo, hi int, out *[64]bool) error {
	for _, part := range strings.Split(s, ",") {
		step := 1
		if i := strings.Index(part, "/"); i >= 0 {
			n, err := strconv.Atoi(part[i+1:])
			if err != nil || n <= 0 {
				return errCron
			}
			step, part = n, part[:i]
		}
		a, b := lo, hi
		switch {
		case part == "*":
		case strings.Contains(part, "-"):
			x, y, _ := strings.Cut(part, "-")
			var err1, err2 error
			a, err1 = strconv.Atoi(x)
			b, err2 = strconv.Atoi(y)
			if err1 != nil || err2 != nil {
				return errCron
			}
		default:
			n, err := strconv.Atoi(part)
			if err != nil {
				return errCron
			}
			a, b = n, n
			if step > 1 {
				b = hi
			}
		}
		if hi == 7 && b == 7 { // Sunday as 7
			out[0] = true
			b = 6
		}
		if a < lo || b > hi || a > b {
			return errCron
		}
		for v := a; v <= b; v += step {
			out[v] = true
		}
	}
	return nil
}

// Parse reads a five-field expression.
func Parse(expr string) (*Spec, error) {
	f := strings.Fields(expr)
	if len(f) != 5 {
		return nil, errCron
	}
	s := &Spec{domAny: f[2] == "*", dowAny: f[4] == "*"}
	for i, r := range []struct {
		lo, hi int
		out    *[64]bool
	}{{0, 59, &s.min}, {0, 23, &s.hour}, {1, 31, &s.dom}, {1, 12, &s.mon}, {0, 7, &s.dow}} {
		if err := parseField(f[i], r.lo, r.hi, r.out); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Next returns the runs after t, in the zone, as cron counts them: a day
// matches its day of month or of week when both are restricted.
func (s *Spec) Next(t time.Time, n int) []time.Time {
	var out []time.Time
	t = t.Truncate(time.Minute).Add(time.Minute)
	for limit := 0; len(out) < n && limit < 600000; limit++ {
		if !s.mon[int(t.Month())] {
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}
		dom, dow := s.dom[t.Day()], s.dow[int(t.Weekday())]
		day := dom && dow
		switch {
		case s.domAny && s.dowAny:
			day = true
		case s.domAny:
			day = dow
		case s.dowAny:
			day = dom
		default:
			day = dom || dow
		}
		if !day {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
			continue
		}
		if !s.hour[t.Hour()] {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
			continue
		}
		if !s.min[t.Minute()] {
			t = t.Add(time.Minute)
			continue
		}
		out = append(out, t)
		t = t.Add(time.Minute)
	}
	return out
}

// NextRuns previews an expression's next runs in a zone.
func NextRuns(expr, zone string, n int) ([]time.Time, error) {
	loc, err := time.LoadLocation(strings.TrimSpace(zone))
	if err != nil {
		return nil, err
	}
	s, err := Parse(expr)
	if err != nil {
		return nil, err
	}
	return s.Next(time.Now().In(loc), n), nil
}
