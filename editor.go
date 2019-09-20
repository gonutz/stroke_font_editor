package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/gonutz/prototype/draw"
)

func main() {
	const (
		idle = iota
		waitingForChar
		copyingChar
	)
	mode := idle

	hideControlPoints := false
	hideBaseLine := false
	hideFrame := false
	hideGrid := false

	type penShape int
	const (
		rectangular penShape = iota
		circular
	)

	pen := circular
	penSize := 1
	const penSizeChangeTimeOut = 4
	penSizeChangeTime := 0

	const baseLine = 2.0 / 3.0
	gridSize := 0.1
	useGrid := true

	var (
		curLetter           rune
		shape               strokes
		allLetters          = make(map[rune]strokes)
		curX, curY          *float64
		curMouseDx          int
		curMouseDy          int
		mouseInDeletionArea bool
	)

	settingsPath := filepath.Join(os.Getenv("APPDATA"), "stroke_font_editor.set")
	defer func() {
		saveAppSettings(appSettings{
			Letter:            curLetter,
			PenShape:          int(pen),
			PenSize:           penSize,
			UseGrid:           useGrid,
			GridSize:          gridSize,
			HideControlPoints: hideControlPoints,
			HideBaseLine:      hideBaseLine,
			HideFrame:         hideFrame,
			HideGrid:          hideGrid,
		}, settingsPath)
	}()
	if s, err := loadAppSettings(settingsPath); err == nil {
		curLetter = s.Letter
		pen = penShape(s.PenShape)
		penSize = s.PenSize
		useGrid = s.UseGrid
		gridSize = s.GridSize
		hideControlPoints = s.HideControlPoints
		hideBaseLine = s.HideBaseLine
		hideFrame = s.HideFrame
		hideGrid = s.HideGrid
	}

	lastPath := filepath.Join(os.Getenv("APPDATA"), "stroke_font_editor.stf")
	if l, err := importFile(lastPath); err == nil {
		allLetters = make(map[rune]strokes)
		for i := range l {
			allLetters[l[i].r] = l[i].shape
		}
		shape = make(strokes, len(allLetters[curLetter]))
		copy(shape, allLetters[curLetter])
	}
	defer func() {
		allLetters[curLetter] = shape
		var l letters
		for r, s := range allLetters {
			l = append(l, letter{r: r, shape: s})
		}
		exportFile(l, lastPath)
	}()

	const windowW, windowH = 960, 800
	check(draw.RunWindow("Stroke Font Editor", windowW, windowH, func(window draw.Window) {
		if window.WasKeyPressed(draw.KeyEscape) {
			window.Close()
		}

		if window.WasKeyPressed(draw.KeyE) &&
			(window.IsKeyDown(draw.KeyLeftControl) ||
				window.IsKeyDown(draw.KeyRightControl)) {
			allLetters[curLetter] = shape
			var l letters
			for r, s := range allLetters {
				l = append(l, letter{r: r, shape: s})
			}
			exportFile(l, "font.stf")
		}

		if !window.IsMouseDown(draw.LeftButton) {
			if mouseInDeletionArea && curX != nil && curY != nil {
				for s := range shape {
					if curX == &shape[s].x1 ||
						curX == &shape[s].x2 ||
						curX == &shape[s].x3 {
						copy(shape[s:], shape[s+1:])
						shape = shape[:len(shape)-1]
						break
					}
				}
			}
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
					allLetters[curLetter] = make(strokes, len(shape))
					copy(allLetters[curLetter], shape)

					curLetter = r

					shape = make(strokes, len(allLetters[curLetter]))
					copy(shape, allLetters[curLetter])
					break
				}
			}
			return
		}

		if mode == copyingChar {
			window.DrawText("Enter the letter to copy", 100, 100, draw.White)
			s := window.Characters()
			if len(s) > 0 {
				mode = idle
				for _, r := range s {
					orig := allLetters[r]
					allLetters[curLetter] = make(strokes, len(orig))
					copy(allLetters[curLetter], orig)
					shape = make(strokes, len(allLetters[curLetter]))
					copy(shape, allLetters[curLetter])
					break
				}
			}
			return
		}

		if button("Change Letter", windowW-buttonW-10, 40) ||
			window.WasKeyPressed(draw.KeyF2) {
			mode = waitingForChar
			return
		}
		if button("Copy Letter", windowW-buttonW-10, 80) {
			mode = copyingChar
			return
		}
		window.DrawText(
			"Letter: "+fmt.Sprint(curLetter)+" ("+string(curLetter)+")",
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

		if window.WasKeyPressed(draw.KeyTab) {
			hideControlPoints = !hideControlPoints
			hideBaseLine = !hideBaseLine
			hideFrame = !hideFrame
			hideGrid = !hideGrid
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

		if window.WasKeyPressed(draw.KeyG) {
			useGrid = !useGrid
		}
		if window.WasKeyPressed(draw.KeyNumAdd) {
			n := int(1.0/gridSize + 0.5)
			gridSize = 1.0 / float64(n+1)
		}
		if window.WasKeyPressed(draw.KeyNumSubtract) {
			n := int(1.0/gridSize + 0.5)
			if n-1 > 0 {
				gridSize = 1.0 / float64(n-1)
			}
		}
		if window.WasKeyPressed(draw.KeyNumMultiply) {
			gridSize /= 2
		}
		if window.WasKeyPressed(draw.KeyNumDivide) {
			gridSize *= 2
			if gridSize > 1 {
				gridSize = 1
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

		// deletion area
		{
			x, y := windowW-10-buttonW, windowH-10-buttonW
			window.FillRect(x, y, buttonW, buttonW, draw.DarkRed)
			text := "Drag here\nto delete\nsomething"
			tw, th := window.GetTextSize(text)
			window.DrawText(text, x+(buttonW-tw)/2, y+(buttonW-th)/2, draw.White)
			mx, my := window.MousePosition()
			mouseInDeletionArea = mx >= x && my >= y && mx < x+buttonW && my < y+buttonW
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

		alignWithGrid := func(x float64) float64 {
			return float64(int(x/gridSize+0.5)) * gridSize
		}

		if curX != nil && curY != nil {
			mx, my := window.MousePosition()
			sx, sy := mx-curMouseDx, my-curMouseDy
			x, y := fromScreen(sx), fromScreen(sy)
			if useGrid {
				x, y = alignWithGrid(x), alignWithGrid(y)
			}
			*curX, *curY = x, y
		}

		// draw grid
		if useGrid && !hideGrid {
			gridColor := draw.RGB(0.9, 0.9, 1)
			for x := gridSize; x < 1.0-gridSize/2; x += gridSize {
				sx := toScreen(x)
				window.DrawLine(sx, canvasMin, sx, canvasMin+canvasSize, gridColor)
			}
			for y := gridSize; y < 1.0-gridSize/2; y += gridSize {
				sy := toScreen(y)
				window.DrawLine(canvasMin, sy, canvasMin+canvasSize, sy, gridColor)
			}
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
				if !hideControlPoints {
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
		if !hideBaseLine {
			window.DrawLine(
				canvasMin,
				toScreen(baseLine),
				canvasMin+canvasSize,
				toScreen(baseLine),
				draw.Purple,
			)
		}

		// draw letter
		for i, stroke := range shape {
			switch stroke.typ {
			case dot:
				x, y := toScreen(stroke.x1), toScreen(stroke.y1)
				drawDot(x-penSize/2, y-penSize/2, penSize, penSize, draw.Black)
				if !hideControlPoints {
					window.DrawText(strconv.Itoa(i), x, y, draw.DarkGreen)
				}
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
				if !hideControlPoints {
					x := toScreen((stroke.x1 + stroke.x2) / 2)
					y := toScreen((stroke.y1 + stroke.y2) / 2)
					window.DrawText(strconv.Itoa(i), x, y, draw.DarkGreen)
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
				if !hideControlPoints {
					x := toScreen((stroke.x1 + stroke.x2) / 2)
					y := toScreen((stroke.y1 + stroke.y2) / 2)
					window.DrawText(strconv.Itoa(i), x, y, draw.DarkGreen)
				}
			default:
				panic("unknown stroke type")
			}
		}

		if !hideFrame {
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

type appSettings struct {
	Letter            rune
	PenShape          int
	PenSize           int
	UseGrid           bool
	GridSize          float64
	HideControlPoints bool
	HideBaseLine      bool
	HideFrame         bool
	HideGrid          bool
}

func saveAppSettings(s appSettings, path string) error {
	data, err := json.Marshal(&s)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, data, 0666)
}

func loadAppSettings(path string) (appSettings, error) {
	var s appSettings
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(data, &s)
	return s, err
}

const exportFileVersion = 1

// exportFile's file format (little-endian encoding is used):
//
// 4 byte     ASCII "STRK" or 1263686739 as integer
// 4 byte     file version
// uint32     length of the following table in bytes
// 	letter table, list of entries:
// uint32     unicode character
// uint32     offset into the data section where the shape is defined
// uint32     number of strokes for this character
// 	data section, list of shapes:
// 6 float32  x1, y1, x2, y2, x3, y3:
//            these describe a bezier curve from x1,y1 to x3,y3 with control
//            point x2,y2.
//            If all points are the same, they describe a single dot.
//            If the last two points are the same, points 1 and 2 describe a
//            straight line.
func exportFile(list letters, path string) error {
	list = simplify(list)

	var buf bytes.Buffer
	w := &buf
	enc := binary.LittleEndian

	// magic number and file version
	w.WriteString("STRK")
	binary.Write(w, enc, uint32(exportFileVersion))

	// table with offsets for letter shapes
	binary.Write(w, enc, uint32(3*4*len(list))) // table length in bytes
	sort.Sort(list)
	var offset uint32
	for _, l := range list {
		binary.Write(w, enc, uint32(l.r))
		binary.Write(w, enc, offset)
		binary.Write(w, enc, uint32(len(l.shape)))
		offset += uint32(3 * 2 * 4 * len(l.shape))
	}

	// shapes back to back as described in the above table
	for _, l := range list {
		for _, s := range l.shape {
			switch s.typ {
			case dot:
				binary.Write(w, enc, float32(s.x1))
				binary.Write(w, enc, float32(s.y1))
				binary.Write(w, enc, float32(s.x1))
				binary.Write(w, enc, float32(s.y1))
				binary.Write(w, enc, float32(s.x1))
				binary.Write(w, enc, float32(s.y1))
			case line:
				binary.Write(w, enc, float32(s.x1))
				binary.Write(w, enc, float32(s.y1))
				binary.Write(w, enc, float32(s.x2))
				binary.Write(w, enc, float32(s.y2))
				binary.Write(w, enc, float32(s.x2))
				binary.Write(w, enc, float32(s.y2))
			case curve:
				binary.Write(w, enc, float32(s.x1))
				binary.Write(w, enc, float32(s.y1))
				binary.Write(w, enc, float32(s.x2))
				binary.Write(w, enc, float32(s.y2))
				binary.Write(w, enc, float32(s.x3))
				binary.Write(w, enc, float32(s.y3))
			default:
				panic("unknown stroke type")
			}
		}
	}

	return ioutil.WriteFile(path, buf.Bytes(), 0666)
}

func simplify(list letters) letters {
	var out letters
	for _, l := range list {
		if len(l.shape) > 0 {
			out = append(out, letter{r: l.r, shape: linearize(l.shape)})
		}
	}
	return out
}

// linearize reorders strokes in a way that allows the stroke to continue for as
// long as possible. An 'S' for example is made up of multiple bezier curves.
// combineStrokes will order them in a way that they start at one end and go all
// the way to the other end in one go.
func linearize(shape strokes) strokes {
	var nodes []*node
	var edges []*edge

	addNode := func(p [2]float64) *node {
		for i := range nodes {
			if p == nodes[i].pos {
				return nodes[i]
			}
		}
		nodes = append(nodes, &node{pos: p})
		return nodes[len(nodes)-1]
	}

	for i, s := range shape {
		edges = append(edges, &edge{
			name: i,
			a:    addNode(s.start()),
			b:    addNode(s.end()),
		})
	}

	// connect nodes to all their edges
	for _, e := range edges {
		if !containsEdge(e.a.edges, e) {
			e.a.edges = append(e.a.edges, e)
		}
		if !containsEdge(e.b.edges, e) {
			e.b.edges = append(e.b.edges, e)
		}
	}

	sort.Sort(byY(nodes))
	sort.Stable(byX(nodes))
	sort.Stable(byEdgeCount(nodes))

	var walkLongest func(n *node) []*edge
	walkLongest = func(n *node) []*edge {
		var paths [][]*edge
		for _, e := range n.edges {
			if !e.visited {
				e.visited = true
				if n.pos != e.a.pos {
					paths = append(paths, append([]*edge{e}, walkLongest(e.a)...))
				} else {
					paths = append(paths, append([]*edge{e}, walkLongest(e.b)...))
				}
				e.visited = false
			}
		}
		var longest []*edge
		maxLen := 0
		for _, path := range paths {
			if len(path) > maxLen {
				maxLen = len(path)
				longest = path
			}
		}
		return longest
	}

	var path []*edge
	for _, e := range edges {
		if !e.visited {
			e.visited = true

			aPath := walkLongest(e.a)
			for _, e := range aPath {
				e.visited = true
			}

			bPath := walkLongest(e.b)
			for _, e := range bPath {
				e.visited = true
			}

			for i := len(aPath) - 1; i >= 0; i-- {
				path = append(path, aPath[i])
			}
			path = append(path, e)
			for i := range bPath {
				path = append(path, bPath[i])
			}
		}
	}

	ordered := make(strokes, len(shape))
	for i, e := range path {
		ordered[i] = shape[e.name]
	}

	flipToFit := func(a, b *stroke, flipA, flipB bool) (aFlipped, bFlipped bool) {
		if a.end() == b.start() {
			// this is what we want
			return false, false
		} else if a.end() == b.end() && flipB {
			b.flip()
			return false, true
		} else if a.start() == b.start() && flipA {
			a.flip()
			return true, false
		} else if a.start() == b.end() && flipA && flipB {
			a.flip()
			b.flip()
			return true, true
		}
		return false, false
	}
	if len(ordered) > 1 {
		_, flippedB := flipToFit(&ordered[0], &ordered[1], true, true)
		for i := 1; i < len(ordered); i++ {
			_, flippedB = flipToFit(&ordered[i-1], &ordered[i], !flippedB, true)
		}
	}

	return ordered
}

type edge struct {
	name    int
	a, b    *node
	visited bool
}

type node struct {
	pos   [2]float64
	edges []*edge
}

type byX []*node

func (x byX) Len() int           { return len(x) }
func (x byX) Less(i, j int) bool { return x[i].pos[0] < x[j].pos[0] }
func (x byX) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

type byY []*node

func (x byY) Len() int           { return len(x) }
func (x byY) Less(i, j int) bool { return x[i].pos[1] < x[j].pos[1] }
func (x byY) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

type byEdgeCount []*node

func (x byEdgeCount) Len() int           { return len(x) }
func (x byEdgeCount) Less(i, j int) bool { return len(x[i].edges) < len(x[j].edges) }
func (x byEdgeCount) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

func containsEdge(edges []*edge, e *edge) bool {
	for i := range edges {
		if edges[i] == e {
			return true
		}
	}
	return false
}

func importFile(path string) (letters, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	r := &errReader{Reader: bytes.NewReader(data)}
	enc := binary.LittleEndian

	// check file magic and version
	var magic [4]byte
	binary.Read(r, enc, &magic)
	if string(magic[:]) != "STRK" {
		return nil, errors.New("STRK expected as magic number at file start")
	}

	var version uint32
	binary.Read(r, enc, &version)
	if version != exportFileVersion {
		return nil, errors.New("wrong file version")
	}

	// read character-to-stroke-offset table
	var headerSize uint32
	binary.Read(r, enc, &headerSize)
	type entry struct {
		Char   uint32
		Offset uint32
		N      uint32
	}
	table := make([]entry, headerSize/12)
	binary.Read(r, enc, &table)

	// read shapes
	var list letters
	for _, e := range table {
		shape := make(strokes, e.N)
		for i := range shape {
			var x1, y1, x2, y2, x3, y3 float32
			binary.Read(r, enc, &x1)
			binary.Read(r, enc, &y1)
			binary.Read(r, enc, &x2)
			binary.Read(r, enc, &y2)
			binary.Read(r, enc, &x3)
			binary.Read(r, enc, &y3)

			shape[i].typ = curve
			if x1 == x2 && y1 == y2 &&
				x1 == x3 && y1 == y3 {
				shape[i].typ = dot
			} else if x2 == x3 && y2 == y3 {
				shape[i].typ = line
			}
			shape[i].x1 = float64(x1)
			shape[i].y1 = float64(y1)
			shape[i].x2 = float64(x2)
			shape[i].y2 = float64(y2)
			shape[i].x3 = float64(x3)
			shape[i].y3 = float64(y3)
		}
		list = append(list, letter{
			r:     rune(e.Char),
			shape: shape,
		})
	}

	if r.err != nil && r.err != io.EOF {
		return nil, err
	}
	return list, nil
}

type errReader struct {
	io.Reader
	err error
}

func (r *errReader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	n, err = r.Reader.Read(p)
	r.err = err
	return
}

type letters []letter

type letter struct {
	r     rune
	shape strokes
}

func (x letters) Len() int           { return len(x) }
func (x letters) Less(i, j int) bool { return x[i].r < x[j].r }
func (x letters) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

type strokes []stroke

type stroke struct {
	typ    strokeType
	x1, y1 float64
	x2, y2 float64
	x3, y3 float64
}

type strokeType byte

const (
	dot strokeType = iota
	line
	curve
)

func (s *stroke) start() [2]float64 {
	return [2]float64{s.x1, s.y1}
}

func (s *stroke) end() [2]float64 {
	switch s.typ {
	case dot:
		return [2]float64{s.x1, s.y1}
	case line:
		return [2]float64{s.x2, s.y2}
	case curve:
		return [2]float64{s.x3, s.y3}
	default:
		panic("unknown stroke type")
	}
}

func (s *stroke) flip() {
	switch s.typ {
	case dot:
		return
	case line:
		s.x1, s.x2 = s.x2, s.x1
		s.y1, s.y2 = s.y2, s.y1
	case curve:
		s.x1, s.x3 = s.x3, s.x1
		s.y1, s.y3 = s.y3, s.y1
	default:
		panic("unknown stroke type")
	}
}
