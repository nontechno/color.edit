package main

import (
	"strings"
	"unicode"

	"color.edit/spec"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type declarativePreview struct {
	*tview.Box
	document       *assignmentDocument
	specification  spec.Specification
	flickerPath    string
	flickerAlias   string
	flickerOn      bool
	candidatePath  string
	candidateAlias string
	candidateColor tcell.Color
	candidateOn    bool
	selectColor    func(string)
	lastClickX     int
	lastClickY     int
	lastClickPaths []string
	lastClickIndex int
	haveLastClick  bool
}

func newDeclarativePreview(document *assignmentDocument) *declarativePreview {
	p := &declarativePreview{
		Box:      tview.NewBox().SetBorder(true).SetTitle(" Live preview "),
		document: document, specification: document.specification,
	}
	p.applyChrome()
	return p
}

func (p *declarativePreview) applyChrome() {
	style := p.specification.Editor
	p.SetBackgroundColor(p.document.color(style.Background, tcell.ColorBlack)).
		SetBorderColor(p.document.color(style.Border, tcell.ColorWhite)).
		SetTitleColor(p.document.color(style.Title, tcell.ColorAqua))
}

// setFlicker blinks the elements bound to one color path.
func (p *declarativePreview) setFlicker(path string, on bool) {
	p.flickerPath, p.flickerAlias, p.flickerOn = path, "", on
}

// setAliasFlicker blinks every element whose color comes from an alias.
func (p *declarativePreview) setAliasFlicker(alias string, on bool) {
	p.flickerPath, p.flickerAlias, p.flickerOn = "", alias, on
}

// setCandidate previews a color for one color path.
func (p *declarativePreview) setCandidate(path string, color tcell.Color, on bool) {
	p.candidatePath, p.candidateAlias, p.candidateColor, p.candidateOn = path, "", color, on
}

// setAliasCandidate previews a new color for an alias, which changes every
// element whose color comes from it.
func (p *declarativePreview) setAliasCandidate(alias string, color tcell.Color, on bool) {
	p.candidatePath, p.candidateAlias, p.candidateColor, p.candidateOn = "", alias, color, on
}

func (p *declarativePreview) setColorSelectionHandler(handler func(string)) {
	p.selectColor = handler
}

func (p *declarativePreview) color(path string) tcell.Color {
	configured, alias := p.document.colorAndAlias(path)
	if p.candidateOn && ((p.candidatePath != "" && p.candidatePath == path) ||
		(p.candidateAlias != "" && alias == p.candidateAlias)) {
		return p.candidateColor
	}
	flickering := p.flickerOn && ((p.flickerPath != "" && p.flickerPath == path) ||
		(p.flickerAlias != "" && alias == p.flickerAlias))
	if !flickering || configured.Hex() < 0 {
		return configured
	}
	return tcell.NewHexColor(configured.Hex() ^ 0xffffff)
}

func (p *declarativePreview) Draw(screen tcell.Screen) {
	p.applyChrome()
	p.Box.Draw(screen)
	x, y, width, height := p.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}
	background := p.color(p.specification.Canvas.Background)
	fillRect(screen, x, y, width, height, ' ', tcell.StyleDefault.Background(background))
	for _, element := range p.specification.Elements {
		ex, ey, ew, eh := p.scaled(element, x, y, width, height)
		if ew <= 0 || eh <= 0 {
			continue
		}
		foreground := p.color(element.Foreground)
		if element.Foreground == "" {
			foreground = p.document.color(p.specification.Editor.Text, tcell.ColorWhite)
		}
		elementBackground := background
		if element.Background != "" {
			elementBackground = p.color(element.Background)
		}
		cellStyle := tcell.StyleDefault.Foreground(foreground).Background(elementBackground)
		switch element.Kind {
		case "fill":
			character := firstRune(element.Character, ' ')
			fillRect(screen, ex, ey, ew, eh, character, cellStyle)
		case "frame":
			drawFrame(screen, ex, ey, ew, eh, cellStyle)
		case "text":
			if element.Fill {
				fillRect(screen, ex, ey, ew, eh, ' ', cellStyle)
			}
			align := tview.AlignLeft
			switch element.Align {
			case "center":
				align = tview.AlignCenter
			case "right":
				align = tview.AlignRight
			}
			lines := strings.Split(element.Text, "\n")
			for offset, line := range lines {
				if offset >= eh {
					break
				}
				tview.Print(screen, tview.Escape(line), ex, ey+offset, ew, align, foreground)
				// tview.Print preserves the existing background written above.
			}
		}
	}
}

func (p *declarativePreview) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return p.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, _ func(tview.Primitive)) (bool, tview.Primitive) {
		x, y := event.Position()
		if !p.InRect(x, y) {
			return false, nil
		}
		switch action {
		case tview.MouseLeftDown:
			// Consume the press without moving keyboard focus away from the color
			// list. The corresponding click performs the selection.
			return true, nil
		case tview.MouseLeftClick, tview.MouseLeftDoubleClick:
			path, ok := p.nextColorPathAt(x, y)
			if ok && p.selectColor != nil {
				p.selectColor(path)
			}
			return true, nil
		default:
			return false, nil
		}
	})
}

// nextColorPathAt returns the visible foreground first and the final painted
// background second. Repeated clicks at the same cell cycle through the unique
// applicable paths.
func (p *declarativePreview) nextColorPathAt(pointX, pointY int) (string, bool) {
	paths := p.colorPathsAt(pointX, pointY)
	if len(paths) == 0 {
		p.haveLastClick = false
		return "", false
	}
	if p.haveLastClick && pointX == p.lastClickX && pointY == p.lastClickY && equalPaths(paths, p.lastClickPaths) {
		p.lastClickIndex = (p.lastClickIndex + 1) % len(paths)
	} else {
		p.lastClickX, p.lastClickY = pointX, pointY
		p.lastClickPaths = append(p.lastClickPaths[:0], paths...)
		p.lastClickIndex = 0
		p.haveLastClick = true
	}
	return paths[p.lastClickIndex], true
}

func (p *declarativePreview) colorPathsAt(pointX, pointY int) []string {
	x, y, width, height := p.GetInnerRect()
	if pointX < x || pointX >= x+width || pointY < y || pointY >= y+height {
		return nil
	}
	foreground := ""
	background := p.specification.Canvas.Background
	for _, element := range p.specification.Elements {
		ex, ey, ew, eh := p.scaled(element, x, y, width, height)
		if ew <= 0 || eh <= 0 || pointX < ex || pointX >= ex+ew || pointY < ey || pointY >= ey+eh {
			continue
		}
		elementBackground := element.Background
		if elementBackground == "" {
			elementBackground = p.specification.Canvas.Background
		}
		elementForeground := element.Foreground
		if elementForeground == "" {
			elementForeground = p.specification.Editor.Text
		}
		switch element.Kind {
		case "fill":
			background = elementBackground
			foreground = ""
			if character := firstRune(element.Character, ' '); !unicode.IsSpace(character) {
				foreground = elementForeground
			}
		case "frame":
			if ew >= 2 && eh >= 2 && (pointX == ex || pointX == ex+ew-1 || pointY == ey || pointY == ey+eh-1) {
				background = elementBackground
				foreground = elementForeground
			}
		case "text":
			if element.Fill {
				background = elementBackground
				foreground = ""
			}
			if textPaintsCell(element, pointX, pointY, ex, ey, ew, eh) {
				foreground = elementForeground
			}
		}
	}
	paths := make([]string, 0, 2)
	for _, path := range []string{foreground, background} {
		if path == "" || (len(paths) > 0 && paths[0] == path) {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

func textPaintsCell(element spec.Element, pointX, pointY, x, y, width, height int) bool {
	row := pointY - y
	lines := strings.Split(element.Text, "\n")
	if row < 0 || row >= height || row >= len(lines) || lines[row] == "" {
		return false
	}
	lineWidth := min(width, tview.TaggedStringWidth(tview.Escape(lines[row])))
	if lineWidth <= 0 {
		return false
	}
	start := x
	switch element.Align {
	case "center":
		start += (width - lineWidth) / 2
	case "right":
		start += width - lineWidth
	}
	return pointX >= start && pointX < start+lineWidth
}

func equalPaths(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (p *declarativePreview) scaled(element spec.Element, x, y, width, height int) (int, int, int, int) {
	canvas := p.specification.Canvas
	if canvas.Resize == "clip" {
		left, top := x+element.X, y+element.Y
		if left >= x+width || top >= y+height {
			return left, top, 0, 0
		}
		return left, top, min(element.Width, x+width-left), min(element.Height, y+height-top)
	}
	left := x + element.X*width/canvas.Width
	top := y + element.Y*height/canvas.Height
	right := x + (element.X+element.Width)*width/canvas.Width
	bottom := y + (element.Y+element.Height)*height/canvas.Height
	// A one-cell dimension represents terminal chrome such as a header row,
	// status line, separator, or scrollbar. Its position scales, but making the
	// stroke two cells thick when the preview grows misrepresents the source UI.
	// Right-edge cells remain anchored to the scaled edge; proportional rounding
	// would otherwise pull a right-frame scrollbar one cell into the content.
	if element.Width == 1 {
		if element.X+element.Width == canvas.Width {
			right = x + width
			left = right - 1
		} else {
			right = left + 1
		}
	}
	if element.Height == 1 {
		bottom = top + 1
	}
	if right <= left {
		right = left + 1
	}
	if bottom <= top {
		bottom = top + 1
	}
	right = min(right, x+width)
	bottom = min(bottom, y+height)
	return left, top, right - left, bottom - top
}

func firstRune(value string, fallback rune) rune {
	for _, character := range value {
		return character
	}
	return fallback
}

func fillRect(screen tcell.Screen, x, y, width, height int, character rune, style tcell.Style) {
	for row := 0; row < height; row++ {
		for column := 0; column < width; column++ {
			screen.SetContent(x+column, y+row, character, nil, style)
		}
	}
}

func drawFrame(screen tcell.Screen, x, y, width, height int, style tcell.Style) {
	if width < 2 || height < 2 {
		return
	}
	for column := 1; column < width-1; column++ {
		screen.SetContent(x+column, y, '─', nil, style)
		screen.SetContent(x+column, y+height-1, '─', nil, style)
	}
	for row := 1; row < height-1; row++ {
		screen.SetContent(x, y+row, '│', nil, style)
		screen.SetContent(x+width-1, y+row, '│', nil, style)
	}
	screen.SetContent(x, y, '┌', nil, style)
	screen.SetContent(x+width-1, y, '┐', nil, style)
	screen.SetContent(x, y+height-1, '└', nil, style)
	screen.SetContent(x+width-1, y+height-1, '┘', nil, style)
}
