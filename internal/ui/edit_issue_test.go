package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEditIssueState_IsSubtask(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		parentKey  string
		parentEpic bool
		want       bool
	}{
		{name: "no parent", parentKey: "", parentEpic: false, want: false},
		{name: "parent is epic", parentKey: "EPIC-1", parentEpic: true, want: false},
		{name: "parent is story", parentKey: "STORY-1", parentEpic: false, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := assert.New(t)
			e := &editIssueState{
				formState: formState{parentKey: tt.parentKey, parentEpic: tt.parentEpic},
			}
			is.Equal(tt.want, e.isSubtask())
		})
	}
}

func TestEditIssueState_SkipField(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		parentKey  string
		parentEpic bool
		field      issueField
		want       bool
	}{
		{name: "labels always skipped", parentKey: "", parentEpic: false, field: ifLabels, want: true},
		{name: "type always skipped", parentKey: "", parentEpic: false, field: ifType, want: true},
		{name: "summary not skipped", parentKey: "", parentEpic: false, field: ifSummary, want: false},
		{name: "epic not skipped for normal issue", parentKey: "", parentEpic: false, field: ifEpic, want: false},
		{name: "epic not skipped when parent is epic", parentKey: "EPIC-1", parentEpic: true, field: ifEpic, want: false},
		{name: "epic skipped for subtask", parentKey: "STORY-1", parentEpic: false, field: ifEpic, want: true},
		{name: "description not skipped", parentKey: "", parentEpic: false, field: ifDescription, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := assert.New(t)
			e := &editIssueState{
				formState: formState{parentKey: tt.parentKey, parentEpic: tt.parentEpic},
			}
			is.Equal(tt.want, e.skipField(tt.field))
		})
	}
}