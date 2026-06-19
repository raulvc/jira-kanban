package ui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/raulvc/jira-kanban/internal/jira"
)

const historyDays = 14

// historyState tracks the history modal.
type historyState struct {
	items    []jira.HistoryItem
	selected int
	scrollY  int
	loading  bool
	err      string
}

func drawHistoryModal(screen tcell.Screen, h *historyState, screenW, screenH int) {
	const padding = 2
	contentW := 80
	contentH := screenH - 8
	if contentH < 10 {
		contentH = 10
	}
	if contentH > 40 {
		contentH = 40
	}

	boxW := contentW + padding*2
	boxH := contentH + padding
	if boxW > screenW-4 {
		boxW = screenW - 4
		contentW = boxW - padding*2
	}
	if boxH > screenH-4 {
		boxH = screenH - 4
	}

	ox := (screenW - boxW) / 2
	oy := (screenH - boxH) / 2

	bgStyle := tcell.StyleDefault.Foreground(colFg).Background(colPanel)
	borderStyle := tcell.StyleDefault.Foreground(colMuted).Background(colPanel)
	titleStyle := tcell.StyleDefault.Foreground(colBlue).Background(colPanel).Bold(true)
	sepStyle := tcell.StyleDefault.Foreground(colMuted).Background(colPanel).Dim(true)
	itemStyle := tcell.StyleDefault.Foreground(colFg).Background(colPanel)
	selStyle := tcell.StyleDefault.Foreground(colFg).Background(colCardSel).Bold(true)
	errStyle := tcell.StyleDefault.Foreground(colRed).Background(colPanel).Bold(true)
	loadingStyle := tcell.StyleDefault.Foreground(colCyan).Background(colPanel).Bold(true)
	hintStyle := tcell.StyleDefault.Foreground(colMuted).Background(colPanel)

	for row := oy; row < oy+boxH; row++ {
		fillRow(screen, ox, row, boxW, bgStyle)
	}
	drawBorder(screen, ox, oy, boxW, boxH, borderStyle)

	cy := oy + 1
	drawText(screen, ox+padding, cy, " Recent Activity ", titleStyle, contentW)
	cy++
	drawSep(screen, ox+padding, cy, contentW, sepStyle)
	cy++

	maxVis := (boxH - 5) / 2
	if maxVis < 1 {
		maxVis = 1
	}

	if h.loading {
		drawText(screen, ox+padding, cy, "  Loading…", loadingStyle, contentW)
		cy++
	} else if h.err != "" {
		drawText(screen, ox+padding, cy, " ✗ "+truncStr(h.err, contentW-3), errStyle, contentW)
		cy++
	} else if len(h.items) == 0 {
		drawText(screen, ox+padding, cy, "  No recent activity", hintStyle, contentW)
		cy++
	} else {
		// Scroll to keep selected item visible
		if h.scrollY > h.selected {
			h.scrollY = h.selected
		}
		if h.selected >= h.scrollY+maxVis {
			h.scrollY = h.selected - maxVis + 1
		}

		for i := h.scrollY; i < len(h.items) && i < h.scrollY+maxVis; i++ {
			it := h.items[i]
			style := itemStyle
			if i == h.selected {
				style = selStyle
			}

			fillRow(screen, ox+1, cy, boxW-2, style)

			// Status icon
			icon := statusIcon(it.Status)
			iconStyle := statusColorHistory(it.Status)
			if i == h.selected {
				iconStyle = iconStyle.Background(colCardSel).Bold(true)
			}

			screen.SetContent(ox+padding, cy, ' ', nil, style)
			drawText(screen, ox+padding+1, cy, icon+" ", iconStyle, 4)

			// Key
			keyStyle := tcell.StyleDefault.Foreground(colCyan).Background(colPanel)
			if i == h.selected {
				keyStyle = tcell.StyleDefault.Foreground(colCyan).Background(colCardSel).Bold(true)
			}
			drawText(screen, ox+padding+4, cy, it.Key, keyStyle, 12)

			// Summary (truncated)
			sumW := contentW - 22
			if it.Epic != "" {
				sumW -= 10
			}
			if sumW < 10 {
				sumW = 10
			}
			drawText(screen, ox+padding+17, cy, truncStr(it.Summary, sumW), style, sumW)

			// Epic badge (right-aligned)
			if it.Epic != "" {
				epicCol := epicColor(it.Epic)
				epicBadge := " " + truncStr(it.Epic, 8) + " "
				epicBadgeStyle := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(epicCol).Bold(true)
				if i == h.selected {
					epicBadgeStyle = tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(epicCol).Bold(true)
				}
				epicX := ox + padding + contentW - len([]rune(epicBadge)) - 1
				drawText(screen, epicX, cy, epicBadge, epicBadgeStyle, len([]rune(epicBadge))+1)
			}
			cy++

			// Change description line
			fillRow(screen, ox+1, cy, boxW-2, style)
			descStyle := tcell.StyleDefault.Foreground(colMuted).Background(colPanel)
			if i == h.selected {
				descStyle = tcell.StyleDefault.Foreground(colMuted).Background(colCardSel)
			}
			var desc string
			if it.ChangeDesc != "" && !it.Updated.IsZero() {
				desc = it.ChangeDesc + " · " + relativeTime(it.Updated)
			} else if it.ChangeDesc != "" {
				desc = it.ChangeDesc
			} else if !it.Updated.IsZero() {
				desc = relativeTime(it.Updated)
			}
			if desc != "" {
				drawText(screen, ox+padding+5, cy, truncStr(desc, contentW-6), descStyle, contentW-6)
			}
			cy++
		}

		// Scroll indicator
		if len(h.items) > maxVis {
			totalScroll := max(1, len(h.items)-maxVis)
			barH := max(1, maxVis*2*maxVis/len(h.items))
			barTop := oy + 3 + h.scrollY*(maxVis*2-barH)/totalScroll
			for i := range maxVis * 2 {
				if barTop+i >= oy+3 && barTop+i < oy+3+maxVis*2 {
					screen.SetContent(ox+padding+contentW, oy+3+i, '▐', nil, tcell.StyleDefault.Foreground(colMuted).Background(colPanel))
				}
			}
		}
	}

	// Hint bar at bottom
	hintY := oy + boxH - 2
	drawSep(screen, ox+padding, hintY-1, contentW, sepStyle)
	fillRow(screen, ox+1, hintY, boxW-2, bgStyle)
	drawText(screen, ox+padding, hintY, " Esc/q close • Enter open • ↑/↓ scroll", hintStyle, contentW)
}

// statusIcon returns a visual icon for a status name.
func statusIcon(status string) string {
	switch {
	case status == "Done" || status == "Closed":
		return "✓"
	case status == "In Progress" || status == "In Review":
		return "●"
	case status == "To Do" || status == "Open" || status == "Backlog":
		return "○"
	default:
		return "◆"
	}
}

// statusColorHistory returns a color for a status icon in the history view.
func statusColorHistory(status string) tcell.Style {
	switch {
	case status == "Done" || status == "Closed":
		return tcell.StyleDefault.Foreground(colGreen).Background(colPanel)
	case status == "In Progress" || status == "In Review":
		return tcell.StyleDefault.Foreground(colBlue).Background(colPanel)
	case status == "To Do" || status == "Open" || status == "Backlog":
		return tcell.StyleDefault.Foreground(colMuted).Background(colPanel)
	default:
		return tcell.StyleDefault.Foreground(colOrange).Background(colPanel)
	}
}

// relativeTime returns a human-readable relative time string.
func relativeTime(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	if d < 48*time.Hour {
		return "yesterday"
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}