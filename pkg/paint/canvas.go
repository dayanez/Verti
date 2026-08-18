package paint

import "strings"

// Canvas accumulates strokes drawn via DrawLine and resolves them into
// Unicode box-drawing characters: orthogonal cells are joined into proper
// corners/junctions (┌┐└┘├┤┬┴┼) by inspecting each cell's four immediate
// neighbors, and diagonal steps render as ╱ or ╲.
type Canvas struct {
	cells map[Point]struct{}
	diag  map[Point]rune
}

// NewCanvas returns an empty canvas.
func NewCanvas() *Canvas {
	return &Canvas{cells: make(map[Point]struct{}), diag: make(map[Point]rune)}
}

// DrawLine walks the Bresenham path from (x0,y0) to (x1,y1) and marks each
// step's cell, classifying each step as horizontal, vertical, or diagonal
// based on its movement relative to the previous point.
func (c *Canvas) DrawLine(x0, y0, x1, y1 int) {
	pts := Line(x0, y0, x1, y1)
	c.cells[pts[0]] = struct{}{}
	for i := 1; i < len(pts); i++ {
		prev, p := pts[i-1], pts[i]
		dx, dy := p.X-prev.X, p.Y-prev.Y
		if dx != 0 && dy != 0 {
			r := '╲'
			if (dx > 0) != (dy > 0) {
				r = '╱'
			}
			c.diag[prev] = r
			c.diag[p] = r
			continue
		}
		c.cells[p] = struct{}{}
	}
}

func (c *Canvas) hasCell(x, y int) bool {
	_, ok := c.cells[Point{x, y}]
	return ok
}

// junctionRune picks the box-drawing character for a cell given which of
// its four cardinal neighbors are also part of the drawing.
func junctionRune(up, down, left, right bool) rune {
	switch {
	case up && down && left && right:
		return '┼'
	case up && down && left:
		return '┤'
	case up && down && right:
		return '├'
	case up && left && right:
		return '┴'
	case down && left && right:
		return '┬'
	case up && left:
		return '┘'
	case up && right:
		return '└'
	case down && left:
		return '┐'
	case down && right:
		return '┌'
	case up && down:
		return '│'
	case left && right:
		return '─'
	case up || down:
		return '│'
	default:
		return '─'
	}
}

// Resolve returns the final rune for every drawn cell.
func (c *Canvas) Resolve() map[Point]rune {
	out := make(map[Point]rune, len(c.cells)+len(c.diag))
	for p := range c.cells {
		out[p] = junctionRune(c.hasCell(p.X, p.Y-1), c.hasCell(p.X, p.Y+1), c.hasCell(p.X-1, p.Y), c.hasCell(p.X+1, p.Y))
	}
	for p, r := range c.diag {
		out[p] = r
	}
	return out
}

// Bounds returns the smallest rectangle containing every drawn cell. ok is
// false for an empty canvas.
func (c *Canvas) Bounds() (minX, minY, maxX, maxY int, ok bool) {
	first := true
	consider := func(p Point) {
		if first {
			minX, maxX, minY, maxY = p.X, p.X, p.Y, p.Y
			first = false
			return
		}
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	for p := range c.cells {
		consider(p)
	}
	for p := range c.diag {
		consider(p)
	}
	return minX, minY, maxX, maxY, !first
}

// Render flattens the canvas into rows of text, one rune per cell, spaces
// where nothing was drawn.
func (c *Canvas) Render() []string {
	minX, minY, maxX, maxY, ok := c.Bounds()
	if !ok {
		return nil
	}
	resolved := c.Resolve()
	rows := make([]string, maxY-minY+1)
	for y := minY; y <= maxY; y++ {
		var sb strings.Builder
		for x := minX; x <= maxX; x++ {
			if r, ok := resolved[Point{x, y}]; ok {
				sb.WriteRune(r)
			} else {
				sb.WriteRune(' ')
			}
		}
		rows[y-minY] = sb.String()
	}
	return rows
}

// Empty reports whether anything has been drawn.
func (c *Canvas) Empty() bool { return len(c.cells) == 0 && len(c.diag) == 0 }

// Clear resets the canvas to empty.
func (c *Canvas) Clear() {
	c.cells = make(map[Point]struct{})
	c.diag = make(map[Point]rune)
}
