package term

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"golang.org/x/term"
)

// Repo реализует терминальный ввод-вывод.
type Repo struct {
	scanner *bufio.Scanner
	writer  io.Writer
	fd      int
}

// New создаёт новый экземпляр терминального репозитория.
func New(reader io.Reader, writer io.Writer, fd int) *Repo {
	return &Repo{
		scanner: bufio.NewScanner(reader),
		writer:  writer,
		fd:      fd,
	}
}

// ReadLine читает строку из stdin.
func (r *Repo) ReadLine() (string, error) {
	if r.scanner.Scan() {
		return strings.TrimSpace(r.scanner.Text()), nil
	}
	if err := r.scanner.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}

// ReadPassword читает пароль из stdin со скрытым вводом.
func (r *Repo) ReadPassword() (string, error) {
	password, err := term.ReadPassword(r.fd)
	if err != nil {
		return "", err
	}
	return string(password), nil
}

// Println выводит строку в stdout.
func (r *Repo) Println(a ...any) {
	fmt.Fprintln(r.writer, a...)
}

// Printf выводит форматированную строку в stdout.
func (r *Repo) Printf(format string, a ...any) {
	fmt.Fprintf(r.writer, format, a...)
}
