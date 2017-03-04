package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSSMKey(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{"ssm://prod.secret", "prod.secret"},
		{"production", ""},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			out := ssmKey(tt.in)
			assert.Equal(t, tt.out, out)
		})
	}
}
