package jira

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseJiraError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantPrefix string
	}{
		{name: "error messages", statusCode: 400, body: `{"errorMessages":["Field is required"],"errors":{}}`, wantPrefix: "HTTP 400: Field is required"},
		{name: "field errors", statusCode: 400, body: `{"errorMessages":[],"errors":{"summary":"is required"}}`, wantPrefix: "HTTP 400: summary: is required"},
		{name: "combined errors", statusCode: 400, body: `{"errorMessages":["Bad request"],"errors":{"field":"msg"}}`, wantPrefix: "HTTP 400:"},
		{name: "non-JSON body", statusCode: 500, body: "internal server error", wantPrefix: "HTTP 500: internal server error"},
		{name: "empty body", statusCode: 403, body: "", wantPrefix: "HTTP 403:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			must := require.New(t)
			is := assert.New(t)
			err := parseJiraError(tt.statusCode, []byte(tt.body))
			must.NotNil(err, "expected error")
			is.Contains(err.Error(), tt.wantPrefix)
		})
	}
}