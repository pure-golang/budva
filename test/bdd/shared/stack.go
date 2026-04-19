package shared

import (
	testsupport "github.com/pure-golang/budva-claude/internal/test/support"
)

const fixturesPath = ".config/stand.json"

// sharedStack создаётся один раз для всех сценариев (TDLib не пересоздаётся).
var (
	sharedStack    *testsupport.LiveStack
	sharedStackErr error
)

// GetOrCreateStack возвращает общий LiveStack, создавая и запуская его при первом вызове.
// После первой ошибки кэширует её и не пытается инициализировать повторно.
func GetOrCreateStack() (*testsupport.LiveStack, error) {
	if sharedStack != nil {
		return sharedStack, nil
	}
	if sharedStackErr != nil {
		return nil, sharedStackErr
	}
	stack := testsupport.NewLiveStack(fixturesPath)
	if err := stack.Start(); err != nil {
		sharedStackErr = err
		return nil, err
	}
	sharedStack = stack
	return stack, nil
}
