package term_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/infra/term"
)

// errReader эмулирует io.Reader, возвращающий ошибку на первом чтении.
type errReader struct {
	err error
}

func (e *errReader) Read(_ []byte) (int, error) {
	return 0, e.err
}

func TestNew(t *testing.T) {
	t.Parallel()

	// Arrange
	var out bytes.Buffer
	in := strings.NewReader("")

	// Act
	r := term.New(in, &out, 0)

	// Assert
	require.NotNil(t, r)
}

func TestRepo_ReadLine(t *testing.T) {
	t.Parallel()

	sentinelErr := errors.New("boom")

	tests := []struct {
		name    string
		reader  io.Reader
		want    string
		wantErr error
	}{
		{
			name:   "single_line",
			reader: strings.NewReader("hello\n"),
			want:   "hello",
		},
		{
			name:   "trims_leading_and_trailing_whitespace",
			reader: strings.NewReader("  padded value  \n"),
			want:   "padded value",
		},
		{
			name:   "trims_tabs_and_crlf",
			reader: strings.NewReader("\tvalue\r\n"),
			want:   "value",
		},
		{
			name:   "empty_line_returns_empty_string",
			reader: strings.NewReader("\n"),
			want:   "",
		},
		{
			name:   "reads_first_line_only",
			reader: strings.NewReader("first\nsecond\n"),
			want:   "first",
		},
		{
			name:    "empty_input_returns_eof",
			reader:  strings.NewReader(""),
			wantErr: io.EOF,
		},
		{
			name:    "reader_error_is_propagated",
			reader:  &errReader{err: sentinelErr},
			wantErr: sentinelErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var out bytes.Buffer
			r := term.New(tt.reader, &out, 0)

			// Act
			got, err := r.ReadLine()

			// Assert
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRepo_ReadLine_SequentialCalls(t *testing.T) {
	t.Parallel()

	// Arrange
	var out bytes.Buffer
	r := term.New(strings.NewReader("alpha\nbeta\ngamma\n"), &out, 0)

	// Act + Assert
	for _, want := range []string{"alpha", "beta", "gamma"} {
		got, err := r.ReadLine()
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}

	// после исчерпания — io.EOF
	got, err := r.ReadLine()
	require.ErrorIs(t, err, io.EOF)
	assert.Empty(t, got)
}

func TestRepo_Println(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []any
		want string
	}{
		{
			name: "single_string",
			args: []any{"hello"},
			want: "hello\n",
		},
		{
			name: "multiple_args_space_separated",
			args: []any{"foo", "bar", 42},
			want: "foo bar 42\n",
		},
		{
			name: "no_args_prints_newline",
			args: nil,
			want: "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var out bytes.Buffer
			r := term.New(strings.NewReader(""), &out, 0)

			// Act
			r.Println(tt.args...)

			// Assert
			assert.Equal(t, tt.want, out.String())
		})
	}
}

func TestRepo_Printf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format string
		args   []any
		want   string
	}{
		{
			name:   "plain_string",
			format: "hello",
			want:   "hello",
		},
		{
			name:   "formatted_with_args",
			format: "%s=%d\n",
			args:   []any{"answer", 42},
			want:   "answer=42\n",
		},
		{
			name:   "percent_literal",
			format: "100%%",
			want:   "100%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var out bytes.Buffer
			r := term.New(strings.NewReader(""), &out, 0)

			// Act
			r.Printf(tt.format, tt.args...)

			// Assert
			assert.Equal(t, tt.want, out.String())
		})
	}
}

// TestRepo_ReadPassword_InvalidFD проверяет error path, когда fd не является tty.
// Happy-path требует настоящего терминала и покрывается integration-тестами.
func TestRepo_ReadPassword_InvalidFD(t *testing.T) {
	t.Parallel()

	// Arrange
	// fd = -1 гарантированно невалиден и вернёт ошибку из term.ReadPassword.
	var out bytes.Buffer
	r := term.New(strings.NewReader(""), &out, -1)

	// Act
	got, err := r.ReadPassword()

	// Assert
	require.Error(t, err)
	assert.Empty(t, got)
}
