package sources

import (
	"testing"
	"time"
)

func TestKpSkipsReadingsThatAreNotNumbers(t *testing.T) {
	now := time.Date(2026, 7, 30, 16, 0, 0, 0, time.UTC)
	stamps := `"datetime":["2026-07-30T06:00:00Z","2026-07-30T09:00:00Z","2026-07-30T12:00:00Z","2026-07-30T15:00:00Z"]`

	cases := []struct {
		name string
		body string
		want float64
		at   string
	}{
		{"the newest value wins", `{"Kp":[1.0,2.0,3.0,4.667],` + stamps + `}`, 4.667, "2026-07-30T15:00:00Z"},
		{"a null hour is not a quiet one", `{"Kp":[1.0,2.0,3.333,null],` + stamps + `}`, 3.333, "2026-07-30T12:00:00Z"},
		{"nothing outside the index scale", `{"Kp":[1.0,2.0,5.0,99.0],` + stamps + `}`, 5.0, "2026-07-30T12:00:00Z"},
		{"nor below it", `{"Kp":[1.0,2.0,5.0,-1.0],` + stamps + `}`, 5.0, "2026-07-30T12:00:00Z"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kp, err := parseKp([]byte(tc.body), now)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if kp.Kp != tc.want {
				t.Errorf("Kp %v, want %v", kp.Kp, tc.want)
			}
			if at := kp.At.Format(time.RFC3339); at != tc.at {
				t.Errorf("reading is from %s, want %s", at, tc.at)
			}
		})
	}
}

func TestKpRefusesAFeedItCannotRead(t *testing.T) {
	now := time.Date(2026, 7, 30, 16, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		body string
	}{
		{"throttled", `{"detail":"Request was throttled."}`},
		{"empty", ``},
		{"no readings", `{"Kp":[],"datetime":[]}`},
		{"all null", `{"Kp":[null,null],"datetime":["2026-07-30T12:00:00Z","2026-07-30T15:00:00Z"]}`},
		{"one timestamp short", `{"Kp":[1.0,2.0],"datetime":["2026-07-30T15:00:00Z"]}`},
		{"stale", `{"Kp":[2.0],"datetime":["2026-07-28T15:00:00Z"]}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if kp, err := parseKp([]byte(tc.body), now); err == nil {
				t.Fatalf("got Kp %v, want an error", kp.Kp)
			}
		})
	}
}
