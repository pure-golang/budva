//go:build smoke

package smoke

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	tc "github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMain(m *testing.M) {
	if err := os.Chdir(filepath.Join("..", "..")); err != nil {
		panic("failed to chdir to project root: " + err.Error())
	}
	os.Exit(m.Run())
}

type SmokeSuite struct {
	suite.Suite
	stack tc.ComposeStack
	port  string
}

func TestSmoke(t *testing.T) {
	suite.Run(t, new(SmokeSuite))
}

func (s *SmokeSuite) SetupSuite() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	stack, err := tc.NewDockerCompose("test/smoke/testdata/docker-compose.yml")
	s.Require().NoError(err)

	s.stack = stack

	err = stack.
		WaitForService("facade", wait.ForHTTP("/live").WithPort("7070/tcp").WithStartupTimeout(2*time.Minute)).
		WaitForService("engine", wait.ForLog("Engine started").WithStartupTimeout(2*time.Minute)).
		Up(ctx, tc.Wait(true))
	s.Require().NoError(err, "failed to start smoke stack")

	container, err := stack.ServiceContainer(ctx, "facade")
	s.Require().NoError(err)

	port, err := container.MappedPort(ctx, "7070/tcp")
	s.Require().NoError(err)

	s.port = port.Port()
}

func (s *SmokeSuite) TearDownSuite() {
	if s.stack != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.stack.Down(ctx, tc.RemoveOrphans(true))
	}
}

func (s *SmokeSuite) TestHealthcheck() {
	resp := s.get("/healthcheck")
	require.Equal(s.T(), http.StatusOK, resp.StatusCode)
}

func (s *SmokeSuite) TestHealth() {
	resp := s.get("/health")
	require.Equal(s.T(), http.StatusOK, resp.StatusCode)
}

func (s *SmokeSuite) TestLive() {
	resp := s.get("/live")
	require.Equal(s.T(), http.StatusOK, resp.StatusCode)
}

func (s *SmokeSuite) TestReady() {
	resp := s.get("/ready")
	require.Equal(s.T(), http.StatusOK, resp.StatusCode)
}

func (s *SmokeSuite) get(path string) *http.Response {
	s.T().Helper()
	url := fmt.Sprintf("http://localhost:%s%s", s.port, path)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	s.Require().NoError(err, "GET %s failed", path)
	return resp
}
