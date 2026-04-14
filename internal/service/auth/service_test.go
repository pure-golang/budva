package auth

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/domain"
)

func TestNew(t *testing.T) {
	t.Parallel()
	svc := New()

	assert.Equal(t, domain.AuthorizationState(0), svc.State())
	assert.NotNil(t, svc.InputChan())
}

func TestSetStateAndState(t *testing.T) {
	t.Parallel()
	svc := New()

	svc.SetState(domain.AuthStateWaitPhone, nil)
	assert.Equal(t, domain.AuthStateWaitPhone, svc.State())

	svc.SetState(domain.AuthStateReady, nil)
	assert.Equal(t, domain.AuthStateReady, svc.State())
}

func TestSubscribeReceivesStateChanges(t *testing.T) {
	t.Parallel()
	svc := New()

	var received []domain.AuthorizationState
	var mu sync.Mutex

	svc.Subscribe(func(state domain.AuthorizationState, _ any) {
		mu.Lock()
		received = append(received, state)
		mu.Unlock()
	})

	svc.SetState(domain.AuthStateWaitPhone, nil)
	svc.SetState(domain.AuthStateWaitCode, nil)
	svc.SetState(domain.AuthStateReady, nil)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []domain.AuthorizationState{
		domain.AuthStateWaitPhone,
		domain.AuthStateWaitCode,
		domain.AuthStateReady,
	}, received)
}

func TestSubscribeReceivesExtra(t *testing.T) {
	t.Parallel()
	svc := New()

	var gotExtra any
	svc.Subscribe(func(_ domain.AuthorizationState, extra any) {
		gotExtra = extra
	})

	hint := &domain.WaitPasswordState{PasswordHint: "pet name"}
	svc.SetState(domain.AuthStateWaitPassword, hint)

	require.NotNil(t, gotExtra)
	ws, ok := gotExtra.(*domain.WaitPasswordState)
	require.True(t, ok)
	assert.Equal(t, "pet name", ws.PasswordHint)
}

func TestMultipleSubscribers(t *testing.T) {
	t.Parallel()
	svc := New()

	var count1, count2 int
	svc.Subscribe(func(_ domain.AuthorizationState, _ any) { count1++ })
	svc.Subscribe(func(_ domain.AuthorizationState, _ any) { count2++ })

	svc.SetState(domain.AuthStateReady, nil)

	assert.Equal(t, 1, count1)
	assert.Equal(t, 1, count2)
}

func TestInputChanSend(t *testing.T) {
	t.Parallel()
	svc := New()

	done := make(chan string, 1)
	go func() {
		done <- svc.ReadInput()
	}()

	svc.InputChan() <- "phone123"

	select {
	case got := <-done:
		assert.Equal(t, "phone123", got)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for input")
	}
}

func TestReadInput(t *testing.T) {
	t.Parallel()
	svc := New()

	go func() {
		svc.InputChan() <- "code456"
	}()

	done := make(chan string, 1)
	go func() {
		done <- svc.ReadInput()
	}()

	select {
	case got := <-done:
		assert.Equal(t, "code456", got)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ReadInput")
	}
}

func TestConcurrentStateAccess(t *testing.T) {
	t.Parallel()
	svc := New()

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.SetState(domain.AuthorizationState(i%3), nil)
			_ = svc.State()
		}()
	}
	wg.Wait()
}
