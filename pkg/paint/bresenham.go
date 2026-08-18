// Package paint implements the Paint Overlay: turning mouse-drag gestures
// into Unicode box-drawing art via Bresenham's line algorithm, resolving
// corners and junctions where strokes meet, and formatting the finished
// art as a comment block in the active file's language.
package paint

// Point is a cell position in the paint canvas's coordinate space (mouse
// column/row).
type Point struct{ X, Y int }

// Line returns every integer grid point from (x0,y0) to (x1,y1) via
// Bresenham's line algorithm.
func Line(x0, y0, x1, y1 int) []Point {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy

	points := []Point{{x0, y0}}
	x, y := x0, y0
	for x != x1 || y != y1 {
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x += sx
		}
		if e2 <= dx {
			err += dx
			y += sy
		}
		points = append(points, Point{x, y})
	}
	return points
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
