package suite

import (
	testsupport "github.com/pure-golang/budva-claude/internal/test/support"
)

const fixturesPath = ".config/stand.json"

// liveStack создаётся один раз для всех сценариев (TDLib не пересоздаётся).
var (
	liveStack    *testsupport.LiveStack
	liveStackErr error
)

// GetOrCreateStack возвращает общий LiveStack, создавая и запуская его при первом вызове.
// После первой ошибки кэширует её и не пытается инициализировать повторно.
func GetOrCreateStack() (*testsupport.LiveStack, error) {
	if liveStack != nil {
		return liveStack, nil
	}
	if liveStackErr != nil {
		return nil, liveStackErr
	}
	stack := testsupport.NewLiveStack(fixturesPath)
	if err := stack.Start(); err != nil {
		liveStackErr = err
		return nil, err
	}
	liveStack = stack
	return stack, nil
}
