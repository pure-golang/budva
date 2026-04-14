package smoke

import (
	"os/exec"
	"testing"
)

func TestEngineBinaryBuilds(t *testing.T) {
	t.Parallel()

	// Act
	cmd := exec.Command("go", "build", "-o", "/dev/null", "../../cmd/engine")
	output, err := cmd.CombinedOutput()

	// Assert
	if err != nil {
		t.Fatalf("engine binary build failed: %v\n%s", err, output)
	}
}

func TestFacadeBinaryBuilds(t *testing.T) {
	t.Parallel()

	// Act
	cmd := exec.Command("go", "build", "-o", "/dev/null", "../../cmd/facade")
	output, err := cmd.CombinedOutput()

	// Assert
	if err != nil {
		t.Fatalf("facade binary build failed: %v\n%s", err, output)
	}
}
