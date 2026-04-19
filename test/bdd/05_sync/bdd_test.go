package sync_test

import (
	"testing"

	"github.com/pure-golang/budva-claude/test/bdd/shared"
)

func Test05Sync(t *testing.T) { shared.RunEpic(t, "05_sync") }
