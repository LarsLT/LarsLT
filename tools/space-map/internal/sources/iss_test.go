package sources

import (
	"testing"
	"time"
)

// tleEpoch is the epoch on goodTLE, day 211.15218593 of 2026.
var tleEpoch = time.Date(2026, 7, 30, 3, 39, 8, 0, time.UTC)

func TestElementsGoOffAfterThreeDays(t *testing.T) {
	cases := []struct {
		name   string
		now    time.Time
		wantOK bool
	}{
		{"fresh off the wire", tleEpoch.Add(2 * time.Hour), true},
		{"a day of drift is drawable", tleEpoch.Add(24 * time.Hour), true},
		{"just inside the cap", tleEpoch.Add(maxTLEAge - time.Hour), true},
		{"past the cap a reboost could have moved it", tleEpoch.Add(maxTLEAge + time.Hour), false},
		{"a week of cache is nowhere near the station", tleEpoch.Add(7 * 24 * time.Hour), false},
		{"elements from the future are a clock fault", tleEpoch.Add(-2 * time.Hour), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseTLE([]byte(goodTLE), tc.now)
			if got := err == nil; got != tc.wantOK {
				t.Fatalf("usable = %v, want %v (err %v)", got, tc.wantOK, err)
			}
		})
	}
}

func TestValidTLERejectsCelestrakProse(t *testing.T) {
	for _, body := range []string{"No GP data found", "", "{}"} {
		if err := validTLE([]byte(body)); err == nil {
			t.Errorf("%q passed for an element set", body)
		}
	}
}
