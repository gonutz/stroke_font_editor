package main

import (
	"fmt"
	"github.com/gonutz/prototype/draw"
)

func main() {
	const (
		idle = iota
		waitingForChar
	)
	mode := idle

	var letter rune
	var shape strokes

	shape = strokes{
		stroke{typ: dot, x1: 0.1, y1: 0.2},
		stroke{typ: line, x1: 0.1, y1: 0.1, x2: 0.9, y2: 0.9},
		stroke{typ: curve, x1: 0.15, y1: 0.2, x2: 0.9, y2: 0.5, x3: 0.95, y3: 0.8},
	}

	const windowW, windowH = 1000, 800
	check(draw.RunWindow("Stroke Font Editor", windowW, windowH, func(window draw.Window) {
		if window.WasKeyPressed(draw.KeyEscape) {
			window.Close()
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

		if button("Change Letter", windowW-buttonW-10, 50) {
			mode = waitingForChar
			return
		}
		window.DrawText(
			"Letter: "+str(letter)+" ("+string(letter)+")",
			windowW-buttonW-10, 10,
			draw.White,
		)

		// draw the current letter
		const canvasMin, canvasSize = 10, windowH - 20
		toScreen := func(t float64) int {
			return int(canvasMin + canvasSize*t + 0.5)
		}

		// clear background
		window.FillRect(canvasMin, canvasMin, canvasSize, canvasSize, draw.White)

		// draw base line
		window.DrawLine(
			canvasMin,
			toScreen(2.0/3.0),
			canvasMin+canvasSize,
			toScreen(2.0/3.0),
			draw.Red,
		)

		// draw draggable control points
		for _, stroke := range shape {
			p := func(x, y float64) {
				sx, sy := toScreen(x), toScreen(y)
				window.FillRect(sx-3, sy-3, 7, 7, draw.RGB(1, 0.8, 0.8))
				window.DrawRect(sx-4, sy-4, 9, 9, draw.RGB(1, 0.5, 0.5))
			}
			p(stroke.x1, stroke.y1)
			if stroke.typ != dot {
				p(stroke.x2, stroke.y2)
			}
			if stroke.typ == curve {
				p(stroke.x3, stroke.y3)
			}
		}

		// draw letter
		for _, stroke := range shape {
			switch stroke.typ {
			case dot:
				window.DrawPoint(
					toScreen(stroke.x1), toScreen(stroke.y1),
					draw.Black,
				)
			case line:
				window.DrawLine(
					toScreen(stroke.x1), toScreen(stroke.y1),
					toScreen(stroke.x2), toScreen(stroke.y2),
					draw.Black,
				)
			case curve:
				interp := func(t float64) (sx, sy int) {
					tt := 1.0 - t
					x := tt*tt*stroke.x1 + 2*tt*t*stroke.x2 + t*t*stroke.x3
					y := tt*tt*stroke.y1 + 2*tt*t*stroke.y2 + t*t*stroke.y3
					return toScreen(x), toScreen(y)
				}
				lastX, lastY := interp(0)
				const step = 0.01
				curT := step
				for {
					t := curT
					if t > 1 {
						t = 1
					}
					x, y := interp(t)
					window.DrawLine(lastX, lastY, x, y, draw.Black)
					lastX, lastY = x, y
					if curT >= 1 {
						break
					}
					curT += step
				}
			default:
				panic("wat? " + stroke.typ)
			}
		}
	}))
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func str(x interface{}) string {
	return fmt.Sprint(x)
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
