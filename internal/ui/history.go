package ui

import (
	"fmt"
	"log/slog"
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

type historyLayout struct {
	ox, boxW, boxH int
	contentW        int
	padding         int
	maxVis          int
	itemStyle       tcell.Style
	selStyle        tcell.Style
	sepStyle        tcell.Style
	bgStyle         tcell.Style
}

func drawHistoryModal(screen tcell.Screen, h *historyState, screenW, screenH int) {
	const padding = 2
	contentW := 80
	contentH := clampHistoryHeight(screenH - 8)

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
	maxVis := max((boxH-5)/2, 1)

	l := historyLayout{
		ox: ox, boxW: boxW, boxH: boxH,
		contentW: contentW, padding: padding, maxVis: maxVis,
		bgStyle:   tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel),
		itemStyle: tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel),
		selStyle:  tcell.StyleDefault.Foreground(T().Fg).Background(T().CardSel).Bold(true),
		sepStyle:  tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel).Dim(true),
	}

	for row := oy; row < oy+boxH; row++ {
		fillRow(screen, ox, row, boxW, l.bgStyle)
	}
	drawBorder(screen, ox, oy, boxW, boxH, tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel))

	cy := oy + 1
	drawText(screen, ox+padding, cy, " Recent Activity ", tcell.StyleDefault.Foreground(T().Blue).Background(T().Panel).Bold(true), contentW)
	cy += 2
	drawSep(screen, ox+padding, cy-1, contentW, l.sepStyle)

	if h.loading {
		drawText(screen, ox+padding, cy, "  Loading…", tcell.StyleDefault.Foreground(T().Cyan).Background(T().Panel).Bold(true), contentW)
	} else if h.err != "" {
		drawText(screen, ox+padding, cy, " ✗ "+truncStr(h.err, contentW-3), tcell.StyleDefault.Foreground(T().Red).Background(T().Panel).Bold(true), contentW)
	} else if len(h.items) == 0 {
		drawText(screen, ox+padding, cy, "  No recent activity", tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel), contentW)
	} else {
		drawHistoryItems(screen, h, &l, cy)
		drawHistoryScrollbar(screen, h, &l, oy)
	}

	hintY := oy + boxH - 2
	drawSep(screen, ox+padding, hintY-1, contentW, l.sepStyle)
	fillRow(screen, ox+1, hintY, boxW-2, l.bgStyle)
	drawText(screen, ox+padding, hintY, " Esc/q close • Enter open • ↑/↓ scroll", tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel), contentW)
}

func clampHistoryHeight(h int) int {
	return max(10, min(40, h))
}

func drawHistoryItems(screen tcell.Screen, h *historyState, l *historyLayout, startCy int) {
	if h.scrollY > h.selected {
		h.scrollY = h.selected
	}
	if h.selected >= h.scrollY+l.maxVis {
		h.scrollY = h.selected - l.maxVis + 1
	}

	cy := startCy
	for i := h.scrollY; i < len(h.items) && i < h.scrollY+l.maxVis; i++ {
		it := h.items[i]
		isSel := i == h.selected
		style := l.itemStyle
		if isSel {
			style = l.selStyle
		}

		fillRow(screen, l.ox+1, cy, l.boxW-2, style)

		icon := statusIcon(it.Status)
		iconStyle := statusColorHistory(it.Status)
		if isSel {
			iconStyle = iconStyle.Background(T().CardSel).Bold(true)
		}

		screen.SetContent(l.ox+l.padding, cy, ' ', nil, style)
		drawText(screen, l.ox+l.padding+1, cy, icon+" ", iconStyle, 4)

		keyStyle := tcell.StyleDefault.Foreground(T().Cyan).Background(T().Panel)
		if isSel {
			keyStyle = tcell.StyleDefault.Foreground(T().Cyan).Background(T().CardSel).Bold(true)
		}
		drawText(screen, l.ox+l.padding+4, cy, it.Key, keyStyle, 12)

		sumW := l.contentW - 22
		if it.Epic != "" {
			sumW -= 10
		}
		sumW = max(10, sumW)
		drawText(screen, l.ox+l.padding+17, cy, truncStr(it.Summary, sumW), style, sumW)

		if it.Epic != "" {
			epicCol := epicColor(it.Epic)
			epicBadge := " " + truncStr(it.Epic, 8) + " "
			epicBadgeStyle := tcell.StyleDefault.Foreground(T().BadgeFg).Background(epicCol).Bold(true)
			epicX := l.ox + l.padding + l.contentW - len([]rune(epicBadge)) - 1
			drawText(screen, epicX, cy, epicBadge, epicBadgeStyle, len([]rune(epicBadge))+1)
		}
		cy++

		fillRow(screen, l.ox+1, cy, l.boxW-2, style)
		descStyle := tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel)
		if isSel {
			descStyle = tcell.StyleDefault.Foreground(T().Muted).Background(T().CardSel)
		}
		desc := historyChangeDesc(it)
		if desc != "" {
			drawText(screen, l.ox+l.padding+5, cy, truncStr(desc, l.contentW-6), descStyle, l.contentW-6)
		}
		cy++
	}
}

func historyChangeDesc(it jira.HistoryItem) string {
	if it.ChangeDesc != "" && !it.Updated.IsZero() {
		return it.ChangeDesc + " · " + relativeTime(it.Updated)
	}
	if it.ChangeDesc != "" {
		return it.ChangeDesc
	}
	if !it.Updated.IsZero() {
		return relativeTime(it.Updated)
	}
	return ""
}

func drawHistoryScrollbar(screen tcell.Screen, h *historyState, l *historyLayout, oy int) {
	if len(h.items) <= l.maxVis {
		return
	}
	totalScroll := max(1, len(h.items)-l.maxVis)
	barH := max(1, l.maxVis*2*l.maxVis/len(h.items))
	barTop := oy + 3 + h.scrollY*(l.maxVis*2-barH)/totalScroll
	scrollStyle := tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel)
	for i := range l.maxVis * 2 {
		if barTop+i >= oy+3 && barTop+i < oy+3+l.maxVis*2 {
			screen.SetContent(l.ox+l.padding+l.contentW, oy+3+i, '▐', nil, scrollStyle)
		}
	}
}

// statusIcon returns a visual icon for a status name.
func statusIcon(status string) string {
	switch status {
	case "Done", "Closed":
		return "✓"
	case "In Progress", "In Review":
		return "●"
	case "To Do", "Open", "Backlog":
		return "○"
	default:
		return "◆"
	}
}

// statusColorHistory returns a color for a status icon in the history view.
func statusColorHistory(status string) tcell.Style {
	switch status {
	case "Done", "Closed":
		return tcell.StyleDefault.Foreground(T().Green).Background(T().Panel)
	case "In Progress", "In Review":
		return tcell.StyleDefault.Foreground(T().Blue).Background(T().Panel)
	case "To Do", "Open", "Backlog":
		return tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel)
	default:
		return tcell.StyleDefault.Foreground(T().Orange).Background(T().Panel)
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
// ── history ────────────────────────────────────────────────────────────────

func openHistory(ctx *appContext) {
	h := &historyState{loading: true}
	ctx.state.history = h

	go func() {
		items, err := ctx.client.SearchRecentActivity(ctx.state.accountID, historyDays)
		ctx.app.QueueUpdateDraw(func() {
			if ctx.state.history != h {
				return
			}
			h.loading = false
			if err != nil {
				h.err = err.Error(); slog.Error("history load failed", "error", err)
				return
			}
			h.items = items
		})
	}()
}

func handleHistoryInput(ctx *appContext, event *tcell.EventKey) *tcell.EventKey {
	h := ctx.state.history
	switch event.Key() {
	case tcell.KeyEscape:
		ctx.state.history = nil
		return nil
	case tcell.KeyRune:
		if event.Rune() == 'q' {
			ctx.state.history = nil
			return nil
		}
	case tcell.KeyUp:
		if h.selected > 0 {
			h.selected--
		}
		return nil
	case tcell.KeyDown:
		if h.selected < len(h.items)-1 {
			h.selected++
		}
		return nil
	case tcell.KeyPgUp:
		h.selected = max(0, h.selected-10)
		return nil
	case tcell.KeyPgDn:
		h.selected = min(len(h.items)-1, h.selected+10)
		return nil
	case tcell.KeyHome:
		h.selected = 0
		return nil
	case tcell.KeyEnd:
		h.selected = max(0, len(h.items)-1)
		return nil
	case tcell.KeyEnter:
		if h.selected >= 0 && h.selected < len(h.items) {
			key := h.items[h.selected].Key
			summary := h.items[h.selected].Summary
			ctx.state.history = nil
			ctx.state.detail = &detailState{
				card: jira.Card{
					Key:     key,
					Summary: summary,
				},
				loading: true,
			}
			go func() {
				full, err := ctx.client.GetIssue(key)
				ctx.app.QueueUpdateDraw(func() {
					d := ctx.state.detail
					if d == nil || d.card.Key != key {
						return
					}
					if err != nil {
						d.loading = false
						d.err = err.Error(); slog.Error("detail load failed", "key", d.card.Key, "error", err)
						return
					}
					d.card = full
					d.loading = false
				})
			}()
		}
		return nil
	case tcell.KeyCtrlC:
		ctx.app.Stop()
		return nil
	}
	return nil
}
