package telegram

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/repo/telegram/mocks"
)

// --- parseFloodWait ---

func TestParseFloodWait(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want time.Duration
	}{
		{
			name: "typical_flood_wait",
			in:   "Too Many Requests: retry after 5",
			want: 5*time.Second + 500*time.Millisecond,
		},
		{
			name: "large_value",
			in:   "retry after 120",
			want: 120*time.Second + 500*time.Millisecond,
		},
		{
			name: "no_match_returns_zero",
			in:   "some other error",
			want: 0,
		},
		{
			name: "zero_seconds_returns_zero",
			in:   "retry after 0",
			want: 0,
		},
		{
			name: "empty_string",
			in:   "",
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := parseFloodWait(tt.in)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- isRelevantUpdate ---

func TestIsRelevantUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		typ  client.Type
		want bool
	}{
		{
			name: "new_message_is_relevant",
			typ:  &client.UpdateNewMessage{},
			want: true,
		},
		{
			name: "send_succeeded_is_relevant",
			typ:  &client.UpdateMessageSendSucceeded{},
			want: true,
		},
		{
			name: "message_edited_is_relevant",
			typ:  &client.UpdateMessageEdited{},
			want: true,
		},
		{
			name: "permanent_delete_is_relevant",
			typ:  &client.UpdateDeleteMessages{IsPermanent: true},
			want: true,
		},
		{
			name: "non_permanent_delete_ignored",
			typ:  &client.UpdateDeleteMessages{IsPermanent: false},
			want: false,
		},
		{
			name: "unrelated_update_ignored",
			typ:  &client.UpdateUser{},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := isRelevantUpdate(tt.typ)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- mapTDLibState ---

func TestMapTDLibState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		state        client.AuthorizationState
		wantRelevant bool
		wantState    domain.AuthorizationState
		wantExtra    any
	}{
		{
			name:         "wait_phone",
			state:        &client.AuthorizationStateWaitPhoneNumber{},
			wantRelevant: true,
			wantState:    domain.AuthStateWaitPhone,
		},
		{
			name:         "wait_code",
			state:        &client.AuthorizationStateWaitCode{},
			wantRelevant: true,
			wantState:    domain.AuthStateWaitCode,
		},
		{
			name:         "wait_password_carries_hint",
			state:        &client.AuthorizationStateWaitPassword{PasswordHint: "hint"},
			wantRelevant: true,
			wantState:    domain.AuthStateWaitPassword,
			wantExtra:    &domain.WaitPasswordState{PasswordHint: "hint"},
		},
		{
			name:         "ready",
			state:        &client.AuthorizationStateReady{},
			wantRelevant: true,
			wantState:    domain.AuthStateReady,
		},
		{
			name:         "closed",
			state:        &client.AuthorizationStateClosed{},
			wantRelevant: true,
			wantState:    domain.AuthStateClosed,
		},
		{
			name:         "wait_tdlib_parameters_skipped",
			state:        &client.AuthorizationStateWaitTdlibParameters{},
			wantRelevant: false,
		},
		{
			name:         "closing_skipped",
			state:        &client.AuthorizationStateClosing{},
			wantRelevant: false,
		},
		{
			name:         "logging_out_skipped",
			state:        &client.AuthorizationStateLoggingOut{},
			wantRelevant: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			event, relevant := mapTDLibState(tt.state)

			// Assert
			assert.Equal(t, tt.wantRelevant, relevant)
			if !tt.wantRelevant {
				return
			}
			assert.Equal(t, tt.wantState, event.State)
			if tt.wantExtra != nil {
				assert.Equal(t, tt.wantExtra, event.Extra)
			} else {
				assert.Nil(t, event.Extra)
			}
		})
	}
}

// --- New и аксессоры ---

func TestNew_InitializesChannels(t *testing.T) {
	t.Parallel()

	// Arrange / Act
	r := New(config.TelegramConfig{})

	// Assert: каналы готовы до Start().
	require.NotNil(t, r)
	assert.NotNil(t, r.Updates())
	assert.NotNil(t, r.AuthStates())
	assert.NotNil(t, r.ClientDone())

	// ClientDone ещё не закрыт.
	select {
	case <-r.ClientDone():
		t.Fatal("clientDone must not be closed before Start")
	default:
	}
}

func TestClose_ResetsClientAdapter(t *testing.T) {
	t.Parallel()

	// Arrange
	m := mocks.NewClientAdapter(t)
	r := New(config.TelegramConfig{})
	r.clientAdapter = m
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)

	// Act
	err := r.Close()

	// Assert
	require.NoError(t, err)
	assert.Nil(t, r.clientAdapter)
}

// --- Submit* ---

func TestSubmitPhone_WritesToChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New(config.TelegramConfig{})
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)

	// Act
	err := r.SubmitPhone(context.Background(), "+79991234567")

	// Assert
	require.NoError(t, err)
	select {
	case got := <-r.phoneCh:
		assert.Equal(t, "+79991234567", got)
	case <-time.After(time.Second):
		t.Fatal("phone was not delivered to channel")
	}
}

func TestSubmitCode_WritesToChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New(config.TelegramConfig{})
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)

	// Act
	err := r.SubmitCode(context.Background(), "12345")

	// Assert
	require.NoError(t, err)
	select {
	case got := <-r.codeCh:
		assert.Equal(t, "12345", got)
	case <-time.After(time.Second):
		t.Fatal("code was not delivered to channel")
	}
}

func TestSubmitPassword_WritesToChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New(config.TelegramConfig{})
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)

	// Act
	err := r.SubmitPassword(context.Background(), "secret")

	// Assert
	require.NoError(t, err)
	select {
	case got := <-r.passwordCh:
		assert.Equal(t, "secret", got)
	case <-time.After(time.Second):
		t.Fatal("password was not delivered to channel")
	}
}

// --- CleanUp ---

func TestCleanUp(t *testing.T) {
	t.Parallel()

	t.Run("removes_existing_directories", func(t *testing.T) {
		t.Parallel()

		// Arrange
		base := t.TempDir()
		dbDir := filepath.Join(base, "db")
		filesDir := filepath.Join(base, "files")
		require.NoError(t, os.MkdirAll(filepath.Join(dbDir, "sub"), 0o755))
		require.NoError(t, os.MkdirAll(filesDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dbDir, "sub", "f.txt"), []byte("x"), 0o600))

		r := New(config.TelegramConfig{
			DatabaseDirectory: dbDir,
			FilesDirectory:    filesDir,
		})

		// Act
		r.CleanUp()

		// Assert
		_, errDB := os.Stat(dbDir)
		_, errFiles := os.Stat(filesDir)
		assert.True(t, os.IsNotExist(errDB), "database dir must be removed")
		assert.True(t, os.IsNotExist(errFiles), "files dir must be removed")
	})

	t.Run("no_op_on_empty_paths", func(t *testing.T) {
		t.Parallel()

		// Arrange: обе директории пустые — не должно быть ни ошибок, ни паники.
		r := New(config.TelegramConfig{})

		// Act + Assert
		assert.NotPanics(t, func() { r.CleanUp() })
	})

	t.Run("missing_directories_logged_as_noop", func(t *testing.T) {
		t.Parallel()

		// Arrange: пути заданы, но физически не существуют — os.RemoveAll вернёт nil,
		// то есть CleanUp просто завершится без фейла.
		base := t.TempDir()
		r := New(config.TelegramConfig{
			DatabaseDirectory: filepath.Join(base, "ghost-db"),
			FilesDirectory:    filepath.Join(base, "ghost-files"),
		})

		// Act + Assert
		assert.NotPanics(t, func() { r.CleanUp() })
	})
}

// --- pendingSends / dispatchSendResult / deliverSendResult ---

func TestDispatchSendResult_SucceededDelivered(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New(config.TelegramConfig{})
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)
	resultCh := make(chan sendResult, 1)
	const tmpID int64 = 100
	r.pendingSends.Store(tmpID, resultCh)

	permanent := &client.Message{Id: 777}

	// Act
	r.dispatchSendResult(&client.UpdateMessageSendSucceeded{
		OldMessageId: tmpID,
		Message:      permanent,
	})

	// Assert
	select {
	case got := <-resultCh:
		require.NoError(t, got.err)
		assert.Equal(t, permanent, got.msg)
	case <-time.After(time.Second):
		t.Fatal("result was not delivered")
	}
	// Entry удалена из pendingSends.
	_, ok := r.pendingSends.Load(tmpID)
	assert.False(t, ok)
}

func TestDispatchSendResult_FailedDelivered(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New(config.TelegramConfig{})
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)
	resultCh := make(chan sendResult, 1)
	const tmpID int64 = 200
	r.pendingSends.Store(tmpID, resultCh)

	// Act
	r.dispatchSendResult(&client.UpdateMessageSendFailed{
		OldMessageId: tmpID,
		Error:        &client.Error{Code: 400, Message: "BAD_REQUEST"},
	})

	// Assert
	select {
	case got := <-resultCh:
		require.Error(t, got.err)
		assert.Nil(t, got.msg)
		assert.Contains(t, got.err.Error(), "code=400")
		assert.Contains(t, got.err.Error(), "BAD_REQUEST")
	case <-time.After(time.Second):
		t.Fatal("result was not delivered")
	}
}

func TestDispatchSendResult_FailedWithNilError(t *testing.T) {
	t.Parallel()

	// Arrange: TDLib может отдать Update без Error — покрываем ветку с "unknown".
	r := New(config.TelegramConfig{})
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)
	resultCh := make(chan sendResult, 1)
	const tmpID int64 = 300
	r.pendingSends.Store(tmpID, resultCh)

	// Act
	r.dispatchSendResult(&client.UpdateMessageSendFailed{
		OldMessageId: tmpID,
		Error:        nil,
	})

	// Assert
	got := <-resultCh
	require.Error(t, got.err)
	assert.Contains(t, got.err.Error(), "unknown")
}

func TestDispatchSendResult_NoSubscriberIsNoOp(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New(config.TelegramConfig{})
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)

	// Act + Assert: dispatch без подписчика не должен паниковать.
	assert.NotPanics(t, func() {
		r.dispatchSendResult(&client.UpdateMessageSendSucceeded{
			OldMessageId: 999,
			Message:      &client.Message{Id: 1},
		})
	})
}

func TestDispatchSendResult_IgnoresUnrelatedUpdates(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New(config.TelegramConfig{})
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)
	resultCh := make(chan sendResult, 1)
	r.pendingSends.Store(int64(1), resultCh)

	// Act: update, не относящийся к send.
	r.dispatchSendResult(&client.UpdateNewMessage{})

	// Assert: подписчик нетронут.
	select {
	case <-resultCh:
		t.Fatal("unrelated update must not deliver result")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestPendingSends_ConcurrentAddRemove(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New(config.TelegramConfig{})
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)
	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)

	// Act
	for i := 0; i < workers; i++ {
		go func(id int64) {
			defer wg.Done()
			ch := make(chan sendResult, 1)
			r.pendingSends.Store(id, ch)
			r.dispatchSendResult(&client.UpdateMessageSendSucceeded{
				OldMessageId: id,
				Message:      &client.Message{Id: id + 1000},
			})
			// Забираем результат, чтобы убедиться, что dispatch доставил ровно одно значение.
			select {
			case got := <-ch:
				require.NoError(t, got.err)
				require.Equal(t, id+1000, got.msg.Id)
			case <-time.After(time.Second):
				t.Errorf("id=%d: result was not delivered", id)
			}
		}(int64(i + 1))
	}
	wg.Wait()

	// Assert: все записи удалены.
	count := 0
	r.pendingSends.Range(func(_, _ any) bool { count++; return true })
	assert.Equal(t, 0, count)
}

// --- SendMessageAndWait ---

func TestSendMessageAndWait_SuccessPath(t *testing.T) {
	t.Parallel()

	// Arrange
	m := mocks.NewClientAdapter(t)
	m.EXPECT().SendMessage(mock.Anything).RunAndReturn(func(_ *client.SendMessageRequest) (*client.Message, error) {
		return &client.Message{Id: 1}, nil
	})
	r := New(config.TelegramConfig{})
	r.clientAdapter = m
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)

	// Фоновая goroutine, эмулирующая UpdateMessageSendSucceeded.
	permanent := &client.Message{Id: 42}
	go func() {
		// Даём немного времени, чтобы SendMessageAndWait добавил запись в pendingSends.
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			_, ok := r.pendingSends.Load(int64(1))
			if ok {
				r.dispatchSendResult(&client.UpdateMessageSendSucceeded{
					OldMessageId: 1,
					Message:      permanent,
				})
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	// Act
	got, err := r.SendMessageAndWait(context.Background(), &client.SendMessageRequest{ChatId: 10})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, permanent, got)
}

func TestSendMessageAndWait_SendMessageError(t *testing.T) {
	t.Parallel()

	// Arrange
	m := mocks.NewClientAdapter(t)
	m.EXPECT().SendMessage(mock.Anything).RunAndReturn(func(_ *client.SendMessageRequest) (*client.Message, error) {
		return nil, errors.New("boom")
	})
	r := New(config.TelegramConfig{})
	r.clientAdapter = m
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)

	// Act
	got, err := r.SendMessageAndWait(context.Background(), &client.SendMessageRequest{ChatId: 10})

	// Assert
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "boom")
}

func TestSendMessageAndWait_ContextCancelled(t *testing.T) {
	t.Parallel()

	// Arrange: SendMessage возвращает tmp_id, но никто не доставит permanent.
	m := mocks.NewClientAdapter(t)
	m.EXPECT().SendMessage(mock.Anything).RunAndReturn(func(_ *client.SendMessageRequest) (*client.Message, error) {
		return &client.Message{Id: 7}, nil
	})
	r := New(config.TelegramConfig{})
	r.clientAdapter = m
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	// Act
	msg, err := r.SendMessageAndWait(ctx, &client.SendMessageRequest{ChatId: 10})

	// Assert
	require.Error(t, err)
	assert.Nil(t, msg)
	assert.ErrorIs(t, err, context.Canceled)

	// pendingSends очищен defer-ом.
	_, ok := r.pendingSends.Load(int64(7))
	assert.False(t, ok)
}

func TestSendMessageAndWait_FloodWaitExhaustsRetries(t *testing.T) {
	t.Parallel()

	// Arrange: каждая попытка возвращает FLOOD_WAIT 1 — после sendFloodWaitRetries+1 попыток
	// ошибка должна просочиться наружу.
	var attempts int32
	mu := sync.Mutex{}
	m := mocks.NewClientAdapter(t)
	m.EXPECT().SendMessage(mock.Anything).RunAndReturn(func(_ *client.SendMessageRequest) (*client.Message, error) {
		mu.Lock()
		attempts++
		mu.Unlock()
		return nil, errors.New("Too Many Requests: retry after 1")
	})
	r := New(config.TelegramConfig{})
	r.clientAdapter = m
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)

	// Ограничиваем общий таймаут теста: wait=1.5s * 2 повторов ~3s, даём запас.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Act
	msg, err := r.SendMessageAndWait(ctx, &client.SendMessageRequest{ChatId: 10})

	// Assert
	require.Error(t, err)
	assert.Nil(t, msg)
	// Ожидаем sendFloodWaitRetries + 1 = 3 попытки.
	mu.Lock()
	assert.EqualValues(t, sendFloodWaitRetries+1, attempts)
	mu.Unlock()
}

func TestSendMessageAndWait_FloodWaitInterruptedByContext(t *testing.T) {
	t.Parallel()

	// Arrange: первый вызов FLOOD_WAIT ≥ 1s, ctx отменяется в середине ожидания.
	m := mocks.NewClientAdapter(t)
	m.EXPECT().SendMessage(mock.Anything).RunAndReturn(func(_ *client.SendMessageRequest) (*client.Message, error) {
		return nil, errors.New("Too Many Requests: retry after 5")
	})
	r := New(config.TelegramConfig{})
	r.clientAdapter = m
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()

	// Act
	msg, err := r.SendMessageAndWait(ctx, &client.SendMessageRequest{ChatId: 10})

	// Assert
	require.Error(t, err)
	assert.Nil(t, msg)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	// Должны выйти задолго до 5-секундного FLOOD_WAIT-ожидания.
	assert.Less(t, time.Since(start), time.Second)
}

func TestSendMessageAndWait_DeliversErrorThroughPendingChannel(t *testing.T) {
	t.Parallel()

	// Arrange: SendMessage успешен, но дальше приходит UpdateMessageSendFailed.
	m := mocks.NewClientAdapter(t)
	m.EXPECT().SendMessage(mock.Anything).RunAndReturn(func(_ *client.SendMessageRequest) (*client.Message, error) {
		return &client.Message{Id: 55}, nil
	})
	r := New(config.TelegramConfig{})
	r.clientAdapter = m
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)

	go func() {
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if _, ok := r.pendingSends.Load(int64(55)); ok {
				r.dispatchSendResult(&client.UpdateMessageSendFailed{
					OldMessageId: 55,
					Error:        &client.Error{Code: 400, Message: "BAD_REQUEST"},
				})
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	// Act
	msg, err := r.SendMessageAndWait(context.Background(), &client.SendMessageRequest{ChatId: 10})

	// Assert
	require.Error(t, err)
	assert.Nil(t, msg)
	assert.Contains(t, err.Error(), "BAD_REQUEST")
}

// --- sendMessageAndWaitOnce непосредственно ---

func TestSendMessageAndWaitOnce_FloodWaitReturnedFromSend(t *testing.T) {
	t.Parallel()

	// Arrange
	m := mocks.NewClientAdapter(t)
	m.EXPECT().SendMessage(mock.Anything).RunAndReturn(func(_ *client.SendMessageRequest) (*client.Message, error) {
		return nil, errors.New("Too Many Requests: retry after 3")
	})
	r := New(config.TelegramConfig{})
	r.clientAdapter = m
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)

	// Act
	msg, wait, err := r.sendMessageAndWaitOnce(context.Background(), &client.SendMessageRequest{ChatId: 1})

	// Assert
	require.Error(t, err)
	assert.Nil(t, msg)
	assert.Equal(t, 3*time.Second+500*time.Millisecond, wait)
}

func TestSendMessageAndWaitOnce_NonFloodErrorReturnsZeroWait(t *testing.T) {
	t.Parallel()

	// Arrange
	m := mocks.NewClientAdapter(t)
	m.EXPECT().SendMessage(mock.Anything).RunAndReturn(func(_ *client.SendMessageRequest) (*client.Message, error) {
		return nil, fmt.Errorf("transport broken")
	})
	r := New(config.TelegramConfig{})
	r.clientAdapter = m
	r.phoneCh = make(chan string, 1)
	r.codeCh = make(chan string, 1)
	r.passwordCh = make(chan string, 1)

	// Act
	msg, wait, err := r.sendMessageAndWaitOnce(context.Background(), &client.SendMessageRequest{ChatId: 1})

	// Assert
	require.Error(t, err)
	assert.Nil(t, msg)
	assert.Equal(t, time.Duration(0), wait)
}
