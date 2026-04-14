package integration

import (
	"testing"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/test/support"
)

type integrationSuite struct {
	*support.Stack
}

func setupSuite(tb testing.TB) *integrationSuite {
	tb.Helper()

	stack, err := support.NewStack()
	if err != nil {
		tb.Fatalf("failed to create stack: %v", err)
	}

	return &integrationSuite{Stack: stack}
}

func (s *integrationSuite) makeRuleSet(sendCopy bool) *domain.RuleSet {
	return s.MakeRuleSet(sendCopy, nil)
}
