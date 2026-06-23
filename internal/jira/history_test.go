package jira

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDescribeChange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		items []changelogItem
		want  string
	}{
		{name: "empty", items: []changelogItem{}, want: ""},
		{name: "set field", items: []changelogItem{{Field: "status", FromString: "", ToString: "In Progress"}}, want: "status set to In Progress"},
		{name: "remove field", items: []changelogItem{{Field: "assignee", FromString: "Alice", ToString: ""}}, want: "assignee removed"},
		{name: "change field", items: []changelogItem{{Field: "priority", FromString: "High", ToString: "Low"}}, want: "priority: High → Low"},
		{name: "both empty", items: []changelogItem{{Field: "labels", FromString: "", ToString: ""}}, want: "labels changed"},
		{
			name:  "multiple changes",
			items: []changelogItem{
				{Field: "status", FromString: "To Do", ToString: "Done"},
				{Field: "assignee", FromString: "", ToString: "Bob"},
			},
			want: "2 changes: status, assignee",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := assert.New(t)
			is.Equal(tt.want, describeChange(tt.items))
		})
	}
}