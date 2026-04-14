package support

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/pure-golang/budva-claude/internal/repo/state"
)

// BadgerContainer управляет testcontainer с BadgerDB HTTP-сервером.
type BadgerContainer struct {
	container testcontainers.Container
	baseURL   string
	tmpDir    string
}

// StartBadgerContainer собирает бинарник на хосте и запускает testcontainer.
func StartBadgerContainer(ctx context.Context) (*BadgerContainer, error) {
	_, thisFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	serverDir := filepath.Join(projectRoot, "test", "support", "badgerserver")

	// Собираем бинарник для linux/amd64
	tmpDir, err := os.MkdirTemp("", "badgerserver-build-*")
	if err != nil {
		return nil, err
	}
	binaryPath := filepath.Join(tmpDir, "badgerserver")

	cmd := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, "./test/support/badgerserver")
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
	if output, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("build badgerserver: %w\n%s", err, output)
	}

	// Копируем бинарник рядом с Dockerfile
	dockerBinary := filepath.Join(serverDir, "badgerserver")
	if err := copyFile(binaryPath, dockerBinary); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("copy binary: %w", err)
	}

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    serverDir,
			Dockerfile: "Dockerfile",
		},
		ExposedPorts: []string{"8080/tcp"},
		WaitingFor:   wait.ForHTTP("/health").WithPort("8080/tcp").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		os.Remove(dockerBinary)
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("start badger container: %w", err)
	}

	// Cleanup бинарника
	os.Remove(dockerBinary)
	os.RemoveAll(tmpDir)

	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("get container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "8080/tcp")
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("get container port: %w", err)
	}

	return &BadgerContainer{
		container: container,
		baseURL:   fmt.Sprintf("http://%s:%s", host, port.Port()),
	}, nil
}

// NewKVStore создаёт KVStore-клиент, подключённый к контейнеру.
func (bc *BadgerContainer) NewKVStore() state.KVStore {
	return NewRemoteKV(bc.baseURL)
}

// Stop останавливает контейнер.
func (bc *BadgerContainer) Stop(ctx context.Context) {
	if bc.container != nil {
		bc.container.Terminate(ctx)
	}
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755)
}
