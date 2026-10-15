package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"release build through ldflags", "1.0.0", "1.0.0"},
		{"go install with a tag", "v1.0.0", "1.0.0"},
		{
			"go build in a checkout with changes",
			"v1.0.1-0.20261015120000-abcdef123456+dirty",
			"1.0.1-0.20261015120000-abcdef123456+dirty",
		},
		{"build without VCS data", "(devel)", "dev"},
		{"no build info", "", "dev"},
		{"v before a letter stays", "vnext", "vnext"},
		{"default version", "dev", "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, normalizeVersion(tt.in))
		})
	}
}

func TestBuildVersion(t *testing.T) {
	t.Parallel()

	got := buildVersion()
	assert.NotEmpty(t, got)
	assert.NotEqual(t, "(devel)", got)
	assert.NotEqual(t, "v", got[:1])
}
