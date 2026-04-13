package steps

import (
	"os"
	"testing"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
)

func TestBDD(t *testing.T) {
	if testing.Short() {
		t.Skip("bdd test")
	}

	suite := godog.TestSuite{
		Name: "bdd",
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"../features"},
			Output:   colors.Colored(os.Stdout),
			TestingT: t,
			Strict:   true,
			Tags:     "~@pending",
		},
		ScenarioInitializer: initScenario,
	}

	if suite.Run() != 0 {
		t.Fatal("bdd suite failed")
	}
}
