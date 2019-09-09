package main

import (
	"fmt"
	"github.com/gonutz/prototype/draw"
	"math"
)

func main() {
	const (
		idle = iota
		waitingForChar
	)
	mode := idle

	const (
		keyHideControlPoints = draw.KeyTab
		keyHideBaseLine      = draw.KeyTab
		keyHideFrame         = draw.KeyTab
	)

	type penShape string
	const (
		rectangular penShape = "rect"
		circular    penShape = "round"
	)

	pen := circular
	penSize := 1
	const penSizeChangeTimeOut = 4
	penSizeChangeTime := 0

	var (
		letter     rune
		shape      strokes
		curX, curY *float64
		curMouseDx int
		curMouseDy int
	)

	shape = strokes{
		stroke{typ: dot, x1: 0.1, y1: 0.2},
		stroke{typ: line, x1: 0.1, y1: 0.1, x2: 0.9, y2: 0.9},
		stroke{typ: curve, x1: 0.15, y1: 0.2, x2: 0.9, y2: 0.5, x3: 0.95, y3: 0.8},
	}

	const windowW, windowH = 960, 800
	check(draw.RunWindow("Stroke Font Editor", windowW, windowH, func(window draw.Window) {
		if window.WasKeyPressed(draw.KeyEscape) {
			window.Close()
		}

		if !window.IsMouseDown(draw.LeftButton) {
			curX, curY = nil, nil
		}

		drawDot := window.FillEllipse
		if pen == rectangular {
			drawDot = window.FillRect
		}

		penSizeChangeTime--
		if penSizeChangeTime < 0 {
			penSizeChangeTime = -1
		}

		const buttonW, buttonH = 150, 30
		button := func(text string, x, y int) bool {
			w, h := buttonW, buttonH
			mx, my := window.MousePosition()
			color := draw.White
			contains := func(xx, yy int) bool {
				return xx >= x && yy >= y && xx < x+w && yy < y+h
			}
			if contains(mx, my) {
				color = draw.LightGray
			}
			window.FillRect(x, y, w, h, color)
			tw, th := window.GetTextSize(text)
			window.DrawText(text, x+(w-tw)/2, y+(h-th)/2, draw.Black)
			for _, c := range window.Clicks() {
				if c.Button == draw.LeftButton && contains(c.X, c.Y) {
					return true
				}
			}
			return false
		}

		if mode == waitingForChar {
			window.DrawText("Enter the new letter", 100, 100, draw.White)
			s := window.Characters()
			if len(s) > 0 {
				mode = idle
				for _, r := range s {
					letter = r
					break
				}
			}
			return
		}

		if button("Change Letter", windowW-buttonW-10, 40) {
			mode = waitingForChar
			return
		}
		window.DrawText(
			"Letter: "+fmt.Sprint(letter)+" ("+string(letter)+")",
			windowW-buttonW-10, 10,
			draw.White,
		)
		if button("New Dot", windowW-buttonW-10, 140) {
			shape = append(shape, stroke{typ: dot, x1: 0, y1: 0})
		}
		if button("New Line", windowW-buttonW-10, 185) {
			shape = append(shape, stroke{typ: line, x1: 0, y1: 0, x2: 0.1, y2: 0})
		}
		if button("New Curve", windowW-buttonW-10, 230) {
			shape = append(shape, stroke{typ: curve,
				x1: 0, y1: 0,
				x2: 0.1, y2: 0.1,
				x3: 0.2, y3: 0,
			})
		}

		if pen == rectangular {
			if button("Round Pen", windowW-buttonW-10, 345) {
				pen = circular
			}
		} else {
			if button("Rect Pen", windowW-buttonW-10, 345) {
				pen = rectangular
			}
		}

		// draw pen size controls
		{
			window.DrawText(
				fmt.Sprintf("Pen Size %d", penSize),
				windowW-buttonW-10+buttonH, 390,
				draw.White,
			)
			top := 420
			{
				x := windowW - buttonW - 10
				y := top
				tw, th := window.GetTextSize("-")
				mx, my := window.MousePosition()
				fill := draw.White
				if mx >= x && my >= y && mx < x+buttonH && my < y+buttonH {
					fill = draw.LightGray
					if window.IsMouseDown(draw.LeftButton) && penSizeChangeTime < 0 {
						penSize--
						if penSize < 1 {
							penSize = 1
						}
						penSizeChangeTime = penSizeChangeTimeOut
					}
				}
				window.FillRect(x, y, buttonH, buttonH, fill)
				window.DrawText("-", x+(buttonH-tw)/2, y+(buttonH-th)/2, draw.Black)
			}
			{
				x := windowW - 10 - buttonH
				y := top
				tw, th := window.GetTextSize("+")
				mx, my := window.MousePosition()
				fill := draw.White
				if mx >= x && my >= y && mx < x+buttonH && my < y+buttonH {
					fill = draw.LightGray
					if window.IsMouseDown(draw.LeftButton) && penSizeChangeTime < 0 {
						penSize++
						penSizeChangeTime = penSizeChangeTimeOut
					}
				}
				window.FillRect(x, y, buttonH, buttonH, fill)
				window.DrawText("+", x+(buttonH-tw)/2, y+(buttonH-th)/2, draw.Black)
			}
			{
				y := top + (buttonH-penSize)/2
				if penSize > buttonH {
					y = top
				}
				left := windowW - buttonW - 10 + buttonH
				right := windowW - 10 - buttonH
				x := left + (right-left-penSize)/2
				drawDot(x, y, penSize, penSize, draw.White)
			}
		}

		// draw the current letter
		const canvasMin, canvasSize = 10, windowH - 20
		toScreen := func(t float64) int {
			return int(canvasMin + canvasSize*t + 0.5)
		}
		fromScreen := func(s int) float64 {
			d := s - canvasMin
			if d < 0 {
				d = 0
			}
			if d > canvasSize-1 {
				d = canvasSize - 1
			}
			return float64(d) / (canvasSize - 1)
		}

		// clear background
		window.FillRect(canvasMin, canvasMin, canvasSize, canvasSize, draw.White)

		if curX != nil && curY != nil {
			mx, my := window.MousePosition()
			sx, sy := mx-curMouseDx, my-curMouseDy
			*curX, *curY = fromScreen(sx), fromScreen(sy)
		}

		// draw draggable control points
		for i := range shape {
			stroke := &shape[i]
			p := func(px, py *float64) {
				x, y := *px, *py
				m := penSize + 10
				sx, sy := toScreen(x), toScreen(y)
				fill, outline := draw.RGB(1, 0.8, 0.8), draw.RGB(1, 0.5, 0.5)
				mx, my := window.MousePosition()
				contains := func(x, y int) bool {
					return x >= sx-m-1 && y >= sy-m-1 &&
						x < sx-m+2+2*m && y < sy-m+2+2*m
				}
				if contains(mx, my) {
					fill, outline = draw.RGB(0.8, 1, 0.8), draw.RGB(0.5, 1, 0.5)
					// if nothing is selected and the mouse was clicked on this
					// control point, make it current
					if curX == nil && curY == nil &&
						window.IsMouseDown(draw.LeftButton) {
						for _, c := range window.Clicks() {
							if c.Button == draw.LeftButton && contains(c.X, c.Y) {
								curX, curY = px, py
								curMouseDx = mx - sx
								curMouseDy = my - sy
							}
						}
					}
				}
				if !window.IsKeyDown(keyHideControlPoints) {
					window.FillRect(sx-m, sy-m, 1+2*m, 1+2*m, fill)
					window.DrawRect(sx-m-1, sy-m-1, 3+2*m, 3+2*m, outline)
				}
			}
			p(&stroke.x1, &stroke.y1)
			if stroke.typ != dot {
				p(&stroke.x2, &stroke.y2)
			}
			if stroke.typ == curve {
				p(&stroke.x3, &stroke.y3)
			}
		}

		// draw base line
		if !window.IsKeyDown(keyHideBaseLine) {
			window.DrawLine(
				canvasMin,
				toScreen(2.0/3.0),
				canvasMin+canvasSize,
				toScreen(2.0/3.0),
				draw.Purple,
			)
		}

		// draw letter
		for _, stroke := range shape {
			switch stroke.typ {
			case dot:
				x, y := toScreen(stroke.x1), toScreen(stroke.y1)
				drawDot(x-penSize/2, y-penSize/2, penSize, penSize, draw.Black)
			case line:
				step := 1.0 / (canvasSize * math.Hypot(stroke.x1-stroke.x2, stroke.y1-stroke.y2))
				curT := 0.0
				for {
					t := curT
					curT += step
					if t > 1 {
						t = 1
					}
					x := toScreen(stroke.x1*t + (1-t)*stroke.x2)
					y := toScreen(stroke.y1*t + (1-t)*stroke.y2)
					drawDot(x-penSize/2, y-penSize/2, penSize, penSize, draw.Black)
					if curT >= 1 {
						break
					}
				}
			case curve:
				interp := func(t float64) (x, y float64) {
					tt := 1.0 - t
					x = tt*tt*stroke.x1 + 2*tt*t*stroke.x2 + t*t*stroke.x3
					y = tt*tt*stroke.y1 + 2*tt*t*stroke.y2 + t*t*stroke.y3
					return
				}
				step := 0.5 / (canvasSize * math.Hypot(stroke.x1-stroke.x3, stroke.y1-stroke.y3))
				curT := 0.0
				for {
					t := curT
					curT += step
					if t > 1 {
						t = 1
					}
					x, y := interp(t)
					sx := toScreen(x)
					sy := toScreen(y)
					drawDot(sx-penSize/2, sy-penSize/2, penSize, penSize, draw.Black)
					if curT >= 1 {
						break
					}
				}
			default:
				panic("wat stroke type? " + stroke.typ)
			}
		}

		if !window.IsKeyDown(keyHideFrame) {
			window.DrawRect(
				canvasMin-1,
				canvasMin-1,
				canvasSize+2,
				canvasSize+2,
				draw.Purple,
			)
		}
	}))
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

type strokes []stroke

type stroke struct {
	typ    strokeType
	x1, y1 float64
	x2, y2 float64
	x3, y3 float64
}

type strokeType string

const (
	dot   strokeType = "dot"
	line  strokeType = "line"
	curve strokeType = "curve"
)
