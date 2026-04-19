//go:build bdd

package steps

import (
	"os"
	"path/filepath"
	"strings"
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

	// Проверяем TDLib auth до запуска сценариев — fail-fast с понятным сообщением
	if _, err := getOrCreateStack(); err != nil {
		t.Fatalf("BDD stack init: %v", err)
	}

	paths, err := resolveFeaturePaths()
	if err != nil {
		t.Fatalf("resolve feature paths: %v", err)
	}
	suite := godog.TestSuite{
		Name: "bdd",
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    paths,
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

// resolveFeaturePaths возвращает список путей для godog:
//   - BDD_PATHS (через запятую) — override для локального дебага;
//   - иначе — подпапки test/bdd/features (каждая = feature-область).
func resolveFeaturePaths() ([]string, error) {
	if env := os.Getenv("BDD_PATHS"); env != "" {
		return strings.Split(env, ","), nil
	}
	const root = "test/bdd/features"
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			paths = append(paths, filepath.Join(root, e.Name()))
		}
	}
	if len(paths) == 0 {
		return []string{root}, nil
	}
	return paths, nil
}
