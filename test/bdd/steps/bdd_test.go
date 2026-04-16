package steps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
)

func TestMain(m *testing.M) {
	if err := os.Chdir(filepath.Join("..", "..", "..")); err != nil {
		panic("failed to chdir to project root: " + err.Error())
	}
	os.Exit(m.Run())
}

func TestBDD(t *testing.T) {
	if testing.Short() {
		t.Skip("bdd test")
	}

	suite := godog.TestSuite{
		Name: "bdd",
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"test/bdd/features"},
			Output:   colors.Colored(os.Stdout),
			TestingT: t,
			Strict:   true,
		},
		ScenarioInitializer: initScenario,
	}

	if suite.Run() != 0 {
		t.Fatal("bdd suite failed")
	}
}
