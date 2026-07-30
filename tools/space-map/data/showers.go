package data

import "time"

// Shower is one annual meteor shower. Dates are month and day: the orbits that
// feed them are fixed, so the calendar barely moves from year to year.
type Shower struct {
	Name       string
	Start      MonthDay
	Peak       MonthDay
	End        MonthDay
	RadiantDec float64 // declination of the radiant, degrees
	ZHR        int     // meteors an hour at the peak, under a perfect sky
}

// MonthDay is a day in the year with no year attached.
type MonthDay struct {
	Month time.Month
	Day   int
}

// Showers is the IMO's working list of major annual showers.
var Showers = []Shower{
	{"Quadrantids", MonthDay{time.December, 28}, MonthDay{time.January, 3}, MonthDay{time.January, 12}, 49.5, 110},
	{"Lyrids", MonthDay{time.April, 16}, MonthDay{time.April, 22}, MonthDay{time.April, 25}, 33.3, 18},
	{"Eta Aquariids", MonthDay{time.April, 19}, MonthDay{time.May, 6}, MonthDay{time.May, 28}, -1.1, 50},
	{"Delta Aquariids", MonthDay{time.July, 12}, MonthDay{time.July, 30}, MonthDay{time.August, 23}, -16.4, 25},
	{"Perseids", MonthDay{time.July, 17}, MonthDay{time.August, 12}, MonthDay{time.August, 24}, 58.0, 100},
	{"Orionids", MonthDay{time.October, 2}, MonthDay{time.October, 21}, MonthDay{time.November, 7}, 15.8, 20},
	{"Southern Taurids", MonthDay{time.September, 10}, MonthDay{time.November, 5}, MonthDay{time.November, 20}, 15.0, 5},
	{"Leonids", MonthDay{time.November, 6}, MonthDay{time.November, 17}, MonthDay{time.November, 30}, 21.6, 15},
	{"Geminids", MonthDay{time.December, 4}, MonthDay{time.December, 14}, MonthDay{time.December, 17}, 32.3, 150},
	{"Ursids", MonthDay{time.December, 17}, MonthDay{time.December, 22}, MonthDay{time.December, 26}, 75.3, 10},
}

// StrongestShower returns the busiest shower running today, and whether any is.
func StrongestShower(now time.Time) (Shower, bool) {
	var best Shower
	found := false
	for _, s := range Showers {
		if !s.ActiveOn(now) {
			continue
		}
		if !found || s.ZHR > best.ZHR {
			best, found = s, true
		}
	}
	return best, found
}

// ActiveOn reports whether the shower is running. The Quadrantids straddle new
// year, so the window is tested the way a clock face is, wrapping round.
func (s Shower) ActiveOn(now time.Time) bool {
	day := dayIndex(MonthDay{now.Month(), now.Day()})
	start, end := dayIndex(s.Start), dayIndex(s.End)
	if start <= end {
		return day >= start && day <= end
	}
	return day >= start || day <= end
}

// PeakIn is how many days remain until the peak, negative once it has passed.
func (s Shower) PeakIn(now time.Time) int {
	days := dayIndex(s.Peak) - dayIndex(MonthDay{now.Month(), now.Day()})
	switch {
	case days > 183:
		days -= 366
	case days < -183:
		days += 366
	}
	return days
}

// dayIndex numbers a month and day within a leap year, so 29 February has a
// slot and no date collides with another.
func dayIndex(md MonthDay) int {
	return time.Date(2024, md.Month, md.Day, 0, 0, 0, 0, time.UTC).YearDay()
}
