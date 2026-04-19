package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthorizationState_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state AuthorizationState
		want  string
	}{
		{name: "wait_phone", state: AuthStateWaitPhone, want: "waitPhone"},
		{name: "wait_code", state: AuthStateWaitCode, want: "waitCode"},
		{name: "wait_password", state: AuthStateWaitPassword, want: "waitPassword"},
		{name: "ready", state: AuthStateReady, want: "ready"},
		{name: "closing", state: AuthStateClosing, want: "closing"},
		{name: "closed", state: AuthStateClosed, want: "closed"},
		{name: "unknown", state: AuthorizationState(99), want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := tt.state.String()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}
