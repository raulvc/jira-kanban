package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEditIssueState_IsSubtask(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		isSub bool
		want  bool
	}{
		{name: "not a subtask", isSub: false, want: false},
		{name: "is a subtask", isSub: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := assert.New(t)
			e := &editIssueState{isSub: tt.isSub}
			is.Equal(tt.want, e.isSubtask())
		})
	}
}

func TestEditIssueState_SkipField(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		isSub  bool
		field  issueField
		want   bool
	}{
		{name: "labels always skipped", isSub: false, field: ifLabels, want: true},
		{name: "type always skipped", isSub: false, field: ifType, want: true},
		{name: "summary not skipped", isSub: false, field: ifSummary, want: false},
		{name: "epic not skipped for normal issue", isSub: false, field: ifEpic, want: false},
		{name: "epic skipped for subtask", isSub: true, field: ifEpic, want: true},
		{name: "description not skipped", isSub: false, field: ifDescription, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := assert.New(t)
			e := &editIssueState{isSub: tt.isSub}
			is.Equal(tt.want, e.skipField(tt.field))
		})
	}
}
