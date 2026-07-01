package ui

import (
	"fmt"

	"github.com/raulvc/jira-kanban/internal/jira"
)

// ── actions ─────────────────────────────────────────────────────────────────

func openTransitionModal(ctx *appContext) {
	var card *jira.Card
	if d := ctx.state.detail; d != nil {
		card = &d.card
	} else {
		card = ctx.state.selectedCard()
	}
	if card == nil {
		return
	}
	ctx.state.statusMsg = " Loading transitions…"
	ctx.app.ForceDraw()
	go func() {
		transitions, err := ctx.client.GetTransitions(card.Key)
		ctx.app.QueueUpdateDraw(func() {
			ctx.state.statusMsg = ""
			if err != nil {
				ctx.state.statusMsg = fmt.Sprintf(" Error: %s", err.Error())
				return
			}
			if len(transitions) == 0 {
				ctx.state.statusMsg = " No transitions available"
				return
			}
			ctx.state.modal = &modalState{
				issueKey:       card.Key,
				allTransitions: transitions,
			}
		})
	}()
}

func executeTransition(ctx *appContext) {
	m := ctx.state.modal
	t := m.selectedTransition()
	if t == nil {
		return
	}
	issueKey := m.issueKey
	transitionID := t.ID
	transitionName := t.Name
	toStatus := t.ToStatus
	toStatusID := t.ToStatusID

	ctx.state.modal = nil
	ctx.state.moveIssueToStatus(issueKey, toStatus)
	if d := ctx.state.detail; d != nil && d.card.Key == issueKey {
		d.card.Status = toStatus
	}
	ctx.state.statusMsg = fmt.Sprintf(" Transitioning %s → %s…", issueKey, transitionName)
	ctx.app.ForceDraw()

	go func() {
		err := ctx.client.DoTransition(issueKey, transitionID)
		if err != nil {
			ctx.app.QueueUpdateDraw(func() {
				ctx.state.statusMsg = fmt.Sprintf(" Error: %s", err.Error())
			})
			startSync(ctx)
			return
		}
		// Persist the status change to cache so the next sync doesn't revert it.
		ctx.client.UpdateCachedStatus(ctx.boardID, issueKey, toStatusID, toStatus)
		ctx.app.QueueUpdateDraw(func() {
			ctx.state.statusMsg = fmt.Sprintf(" %s → %s", issueKey, transitionName)
		})
		startSync(ctx)
	}()
}
