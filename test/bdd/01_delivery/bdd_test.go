package delivery_test

import (
	"testing"

	"github.com/pure-golang/budva-claude/test/bdd/shared"
)

func Test01Delivery(t *testing.T) { shared.RunEpic(t, "01_delivery") }
