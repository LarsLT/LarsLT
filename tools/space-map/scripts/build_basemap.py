"""Turn Natural Earth 110m country outlines into committed SVG path data.

One-off generator. The runtime build never downloads geometry, it just reads
data/basemap.json. Rerun this if the map size or the simplification changes.

    python tools/space-map/scripts/build_basemap.py
"""

import argparse
import json
import math
import sys
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

SOURCE_URL = (
    "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/"
    "geojson/ne_110m_admin_0_countries.geojson"
)

MAP_WIDTH = 1000
MAP_HEIGHT = 500

# Degrees of lat/lon that may be collapsed away. At 1000px wide, one degree of
# longitude is 2.8px, so 0.35 keeps the error around a pixel.
DEFAULT_TOLERANCE = 0.35

# Islands smaller than this in projected pixels are not worth their bytes.
MIN_FEATURE_PX = 1.2

OUT_PATH = Path(__file__).resolve().parent.parent / "data" / "basemap.json"


def fetch_geojson(url: str) -> dict:
    req = urllib.request.Request(url, headers={"User-Agent": "space-map-basemap/1.0"})
    with urllib.request.urlopen(req, timeout=120) as resp:
        return json.load(resp)


def unwrap(ring: list) -> list:
    """Remove the +-180 seam by letting longitude run continuously.

    Natural Earth stores Russia and Fiji with longitudes that jump from 179 to
    -179. Left alone those jumps draw a stripe straight across the map.
    """
    out = [list(ring[0])]
    for lon, lat in ring[1:]:
        prev_lon = out[-1][0]
        while lon - prev_lon > 180:
            lon -= 360
        while prev_lon - lon > 180:
            lon += 360
        out.append([lon, lat])
    return out


def perpendicular_distance(pt, start, end) -> float:
    if start == end:
        return math.hypot(pt[0] - start[0], pt[1] - start[1])
    dx = end[0] - start[0]
    dy = end[1] - start[1]
    n = abs(dy * pt[0] - dx * pt[1] + end[0] * start[1] - end[1] * start[0])
    return n / math.hypot(dx, dy)


def simplify(points: list, tolerance: float) -> list:
    """Douglas-Peucker, iterative so a long coastline cannot blow the stack."""
    if len(points) < 3:
        return points

    keep = [False] * len(points)
    keep[0] = keep[-1] = True
    stack = [(0, len(points) - 1)]

    while stack:
        first, last = stack.pop()
        max_dist = 0.0
        index = first
        for i in range(first + 1, last):
            dist = perpendicular_distance(points[i], points[first], points[last])
            if dist > max_dist:
                max_dist = dist
                index = i
        if max_dist > tolerance:
            keep[index] = True
            stack.append((first, index))
            stack.append((index, last))

    return [p for p, k in zip(points, keep) if k]


def project(lon: float, lat: float) -> tuple:
    x = (lon + 180.0) / 360.0 * MAP_WIDTH
    y = (90.0 - lat) / 180.0 * MAP_HEIGHT
    return x, y


def ring_to_path(ring: list, shift: float) -> str:
    """Project one ring into an SVG subpath, shifted by whole turns of the globe."""
    coords = [project(lon + shift, lat) for lon, lat in ring]

    xs = [c[0] for c in coords]
    ys = [c[1] for c in coords]
    if max(xs) - min(xs) < MIN_FEATURE_PX and max(ys) - min(ys) < MIN_FEATURE_PX:
        return ""
    # Entirely off-canvas after the shift, so the clip would eat it anyway.
    if max(xs) < 0 or min(xs) > MAP_WIDTH:
        return ""

    parts = []
    last = None
    for x, y in coords:
        point = (round(x, 1), round(y, 1))
        if point == last:
            continue
        parts.append(f"{point[0]},{point[1]}")
        last = point

    if len(parts) < 3:
        return ""
    return "M" + "L".join(parts) + "Z"


def rings_of(geometry: dict):
    kind = geometry.get("type")
    if kind == "Polygon":
        yield from geometry["coordinates"]
    elif kind == "MultiPolygon":
        for polygon in geometry["coordinates"]:
            yield from polygon


def build(tolerance: float, source: str) -> dict:
    geo = fetch_geojson(source)
    subpaths = []
    rings_in = 0
    points_in = 0

    for feature in geo["features"]:
        geometry = feature.get("geometry") or {}
        for ring in rings_of(geometry):
            rings_in += 1
            points_in += len(ring)
            unwrapped = unwrap(ring)
            reduced = simplify(unwrapped, tolerance)
            if len(reduced) < 4:
                continue
            lons = [p[0] for p in reduced]
            # A ring that ran past the seam gets drawn again one turn over, so
            # the far side of Russia shows up on the opposite map edge.
            for shift in (-360.0, 0.0, 360.0):
                if max(lons) + shift < -180 or min(lons) + shift > 180:
                    continue
                path = ring_to_path(reduced, shift)
                if path:
                    subpaths.append(path)

    return {
        "generated": datetime.now(timezone.utc).isoformat(timespec="seconds"),
        "source": source,
        "tolerance_deg": tolerance,
        "viewbox": [MAP_WIDTH, MAP_HEIGHT],
        "rings": len(subpaths),
        "land": "".join(subpaths),
        "_stats": {"rings_in": rings_in, "points_in": points_in},
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tolerance", type=float, default=DEFAULT_TOLERANCE)
    parser.add_argument("--source", default=SOURCE_URL)
    parser.add_argument("--out", type=Path, default=OUT_PATH)
    args = parser.parse_args()

    data = build(args.tolerance, args.source)
    stats = data.pop("_stats")
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(data, separators=(",", ":")) + "\n", encoding="utf-8")

    size_kb = args.out.stat().st_size / 1024
    print(
        f"{stats['rings_in']} rings / {stats['points_in']} points in, "
        f"{data['rings']} subpaths out, {size_kb:.1f} KB -> {args.out}"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
