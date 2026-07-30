package astro

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

// Physical constants for the propagator, in kilometres and seconds.
const (
	mu           = 398600.4418   // Earth's gravitational parameter
	earthRadius  = 6378.137      // equatorial radius
	j2           = 1.08262668e-3 // oblateness, what makes the orbit plane drift
	keplerRounds = 12
)

// TLE is a two-line element set, reduced to the orbit it describes.
type TLE struct {
	Name          string
	Epoch         time.Time
	Inclination   float64 // radians
	RAAN          float64 // radians, at epoch
	Eccentricity  float64
	ArgPerigee    float64 // radians, at epoch
	MeanAnomaly   float64 // radians, at epoch
	MeanMotion    float64 // radians per second
	MeanMotionDot float64 // drag, radians per second squared

	semiMajor   float64
	raanRate    float64 // radians per second
	perigeeRate float64
}

// ParseTLE reads the two numbered lines, with or without a name above them.
func ParseTLE(text string) (*TLE, error) {
	var name, line1, line2 string
	for raw := range strings.SplitSeq(text, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "1 ") && len(line) >= 64:
			line1 = line
		case strings.HasPrefix(line, "2 ") && len(line) >= 63:
			line2 = line
		case line != "" && name == "":
			name = line
		}
	}
	if line1 == "" || line2 == "" {
		return nil, fmt.Errorf("no two-line element set found")
	}

	epoch, err := parseEpoch(line1[18:32])
	if err != nil {
		return nil, err
	}

	t := &TLE{Name: name, Epoch: epoch}
	fields := []struct {
		dst  *float64
		text string
	}{
		{&t.Inclination, line2[8:16]},
		{&t.RAAN, line2[17:25]},
		{&t.ArgPerigee, line2[34:42]},
		{&t.MeanAnomaly, line2[43:51]},
	}
	for _, f := range fields {
		v, err := strconv.ParseFloat(strings.TrimSpace(f.text), 64)
		if err != nil {
			return nil, fmt.Errorf("parse element %q: %w", strings.TrimSpace(f.text), err)
		}
		*f.dst = v * deg
	}

	// Eccentricity is written without its leading decimal point.
	ecc, err := strconv.ParseFloat("0."+strings.TrimSpace(line2[26:33]), 64)
	if err != nil {
		return nil, fmt.Errorf("parse eccentricity: %w", err)
	}
	t.Eccentricity = ecc

	revsPerDay, err := strconv.ParseFloat(strings.TrimSpace(line2[52:63]), 64)
	if err != nil {
		return nil, fmt.Errorf("parse mean motion: %w", err)
	}
	if revsPerDay <= 0 {
		return nil, fmt.Errorf("mean motion %g is not positive", revsPerDay)
	}
	t.MeanMotion = revsPerDay * 2 * math.Pi / 86400

	// Drag, written as half the first derivative of mean motion. Dropping it
	// leaves a week-old element set a couple of hundred kilometres behind.
	nDot, err := strconv.ParseFloat(strings.TrimSpace(line1[33:43]), 64)
	if err != nil {
		return nil, fmt.Errorf("parse mean motion rate: %w", err)
	}
	t.MeanMotionDot = nDot * 2 * math.Pi / (86400 * 86400)

	t.derive()
	return t, nil
}

// parseEpoch reads the TLE's own date format: two digits of year, then the day
// of that year with a fraction. Years run 57-99 for the 1900s, 00-56 for 2000s.
func parseEpoch(field string) (time.Time, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(field), 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse epoch %q: %w", field, err)
	}
	year := int(value / 1000)
	if year < 57 {
		year += 2000
	} else {
		year += 1900
	}
	dayOfYear := value - float64(int(value/1000))*1000

	start := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	return start.Add(time.Duration((dayOfYear - 1) * 86400 * float64(time.Second))), nil
}

// derive fills in the orbit size and the two drift rates that matter over a
// pass: the plane swinging round, and perigee walking within it.
func (t *TLE) derive() {
	t.semiMajor = math.Cbrt(mu / (t.MeanMotion * t.MeanMotion))
	semiLatus := t.semiMajor * (1 - t.Eccentricity*t.Eccentricity)
	factor := -1.5 * j2 * math.Pow(earthRadius/semiLatus, 2) * t.MeanMotion
	t.raanRate = factor * math.Cos(t.Inclination)
	t.perigeeRate = -factor * (2 - 2.5*math.Pow(math.Sin(t.Inclination), 2))
}

// Period is one trip round the Earth.
func (t *TLE) Period() time.Duration {
	return time.Duration(2 * math.Pi / t.MeanMotion * float64(time.Second))
}

// Age is how stale the elements are. They drift slowly, so a cached set stays
// usable for days.
func (t *TLE) Age(now time.Time) time.Duration { return now.Sub(t.Epoch) }

// SubPoint is the spot on the ground the satellite is directly above. Kepler
// plus J2 and drag, twenty kilometres off a live fix half a day past epoch.
func (t *TLE) SubPoint(at time.Time) geo.Point {
	dt := at.Sub(t.Epoch).Seconds()

	meanAnomaly := t.MeanAnomaly + t.MeanMotion*dt + t.MeanMotionDot*dt*dt
	eccentric := solveKepler(meanAnomaly, t.Eccentricity)
	trueAnomaly := 2 * math.Atan2(
		math.Sqrt(1+t.Eccentricity)*math.Sin(eccentric/2),
		math.Sqrt(1-t.Eccentricity)*math.Cos(eccentric/2),
	)

	raan := t.RAAN + t.raanRate*dt
	argument := t.ArgPerigee + t.perigeeRate*dt + trueAnomaly

	// Orbital plane into Earth-centred inertial coordinates. The radius cancels
	// out of the direction, so only the angles are needed.
	sinArg, cosArg := math.Sincos(argument)
	sinRaan, cosRaan := math.Sincos(raan)
	cosInc := math.Cos(t.Inclination)
	x := cosArg*cosRaan - sinArg*sinRaan*cosInc
	y := cosArg*sinRaan + sinArg*cosRaan*cosInc
	z := sinArg * math.Sin(t.Inclination)

	return geo.Point{
		Lat: math.Asin(z) * rad,
		Lon: geo.WrapLon(math.Atan2(y, x)*rad - GreenwichAngle(at)),
	}
}

// solveKepler turns mean anomaly into eccentric anomaly. Newton converges in a
// handful of steps at the eccentricities real satellites fly.
func solveKepler(meanAnomaly, eccentricity float64) float64 {
	e := meanAnomaly
	for range keplerRounds {
		e -= (e - eccentricity*math.Sin(e) - meanAnomaly) / (1 - eccentricity*math.Cos(e))
	}
	return e
}

// GroundTrack samples the path under the satellite from before to after now.
func (t *TLE) GroundTrack(now time.Time, span, step time.Duration) ([]geo.Point, []time.Time) {
	var points []geo.Point
	var times []time.Time
	for offset := -span; offset <= span; offset += step {
		at := now.Add(offset)
		points = append(points, t.SubPoint(at))
		times = append(times, at)
	}
	return points, times
}
