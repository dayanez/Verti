package paint

// ---------------- Construction ----------------

// Overlay is the paint mode's state machine: whether the modal is active,
// the in-progress drag (if any), and the canvas being drawn on. It has no
// dependency on bubbletea or any terminal library; the app layer feeds it
// plain (column, row) coordinates from mouse events.
type Overlay struct {
	Active bool
	Canvas *Canvas

	dragging  bool
	dragLastX int
	dragLastY int
}

// NewOverlay returns an inactive overlay with an empty canvas.
func NewOverlay() *Overlay {
	return &Overlay{Canvas: NewCanvas()}
}

// ---------------- Drag lifecycle ----------------

// Toggle flips whether the overlay is active, ending any in-progress drag.
func (o *Overlay) Toggle() {
	o.Active = !o.Active
	o.dragging = false
}

// BeginDrag starts a new stroke at (x, y).
func (o *Overlay) BeginDrag(x, y int) {
	o.dragging = true
	o.dragLastX, o.dragLastY = x, y
	o.Canvas.DrawLine(x, y, x, y)
}

// UpdateDrag extends the current stroke to (x, y), drawing the Bresenham
// segment from the last reported point.
func (o *Overlay) UpdateDrag(x, y int) {
	if !o.dragging {
		return
	}
	o.Canvas.DrawLine(o.dragLastX, o.dragLastY, x, y)
	o.dragLastX, o.dragLastY = x, y
}

// EndDrag finishes the current stroke at (x, y).
func (o *Overlay) EndDrag(x, y int) {
	if !o.dragging {
		return
	}
	o.Canvas.DrawLine(o.dragLastX, o.dragLastY, x, y)
	o.dragging = false
}

// Reset clears the canvas and any in-progress drag without changing
// Active.
func (o *Overlay) Reset() {
	o.Canvas.Clear()
	o.dragging = false
}

// ---------------- Output ----------------

// Finish renders the canvas as a language-appropriate comment block for
// filename, then clears the canvas. It does not change Active; the
// caller decides whether finishing a drawing also exits paint mode.
func (o *Overlay) Finish(filename string) string {
	art := o.Canvas.Render()
	comment := FormatComment(filename, art)
	o.Reset()
	return comment
}
