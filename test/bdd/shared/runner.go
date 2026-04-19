package shared

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
	"golang.org/x/sys/unix"
)

// chdirOnce гарантирует один os.Chdir в корень репо на процесс: каждый
// per-epic bdd-пакет лежит в test/bdd/<NN_name>/, а LiveStack и feature-пути
// ожидают cwd == корень. Выполняется лениво при первом RunEpic, поэтому
// отдельный TestMain в каждом пакете не нужен.
var chdirOnce sync.Once

func chdirProjectRoot() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to resolve shared/runner.go path via runtime.Caller")
	}
	// file == <repo>/test/bdd/shared/runner.go → поднимаемся на три уровня.
	root := filepath.Join(filepath.Dir(file), "..", "..", "..")
	if err := os.Chdir(root); err != nil {
		panic("failed to chdir to project root: " + err.Error())
	}
}

// acquireTDLibLock захватывает advisory file lock перед инициализацией TDLib.
// TDLib держит эксклюзивный flock на td.binlog, поэтому параллельный запуск
// нескольких per-epic бинарей падает с «Can't lock file». Наш lock выстраивает
// пакеты в очередь на уровне ОС, не требуя -p 1 снаружи.
// Блокируется до освобождения lock другим процессом.
func acquireTDLibLock() (release func(), err error) {
	f, err := os.OpenFile(".tdlib-bdd.lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open tdlib lock file: %w", err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("acquire tdlib lock: %w", err)
	}
	return func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}, nil
}

// RunEpic запускает godog-сюиту для эпика, определяя его имя из пути вызывающего файла.
// Вызывай из bdd_test.go пакета, лежащего в test/bdd/<NN_epic>/.
func RunEpic(t *testing.T) {
	t.Helper()
	chdirOnce.Do(chdirProjectRoot)

	if testing.Short() {
		t.Skip("bdd test")
	}

	_, callerFile, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("BDD: failed to resolve caller path")
	}
	epic := filepath.Base(filepath.Dir(callerFile))

	SetEpicPrefix(strings.SplitN(epic, "_", 2)[0])

	release, err := acquireTDLibLock()
	if err != nil {
		t.Fatalf("BDD lock: %v", err)
	}
	defer release()

	if _, err := GetOrCreateStack(); err != nil {
		t.Fatalf("BDD stack init: %v", err)
	}

	suite := godog.TestSuite{
		Name: "bdd/" + epic,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    featurePaths(epic),
			Output:   colors.Colored(os.Stdout),
			TestingT: t,
			Strict:   true,
		},
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			s := &ScenarioCtx{}
			ctx.Before(func(gctx context.Context, _ *godog.Scenario) (context.Context, error) {
				if err := s.Reset(); err != nil {
					return gctx, err
				}
				return gctx, nil
			})
			ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
				return gctx, nil
			})
			RegisterAllSteps(ctx, s)
		},
	}

	if suite.Run() != 0 {
		t.Fatal("bdd suite failed")
	}
}

// featurePaths возвращает пути к .feature-файлам для godog:
//   - BDD_PATHS (через запятую) — override для локального дебага; принимает
//     как директории, так и конкретные файлы. Сочетайте с `-run Test`,
//     иначе другие пакеты прогонят пустые сюиты.
//   - иначе — директория эпика `test/bdd/<epic>` (godog ищет только *.feature).
func featurePaths(epic string) []string {
	if env := os.Getenv("BDD_PATHS"); env != "" {
		return strings.Split(env, ",")
	}
	return []string{filepath.Join("test", "bdd", epic)}
}
