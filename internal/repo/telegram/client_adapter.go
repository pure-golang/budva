package telegram

import (
	"context"
	"log/slog"

	"github.com/pure-golang/budva-claude/internal/domain"
)

// clientAdapter — контракт TDLib-клиента.
type clientAdapter interface {
	// Операции с сообщениями.
	SendMessage(ctx context.Context, chatID domain.ChatID, content domain.InputMessageContent) (domain.MessageID, error)
	SendMessageAlbum(ctx context.Context, chatID domain.ChatID, contents []domain.InputMessageContent) ([]domain.MessageID, error)
	ForwardMessages(ctx context.Context, fromChatID, toChatID domain.ChatID, messageIDs []domain.MessageID) ([]domain.MessageID, error)
	GetMessage(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) (*domain.Message, error)
	EditMessageText(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, text *domain.FormattedText) error
	EditMessageCaption(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, text *domain.FormattedText) error
	DeleteMessages(ctx context.Context, chatID domain.ChatID, messageIDs []domain.MessageID, revoke bool) error

	// Операции со ссылками.
	GetMessageLink(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, forAlbum bool) (string, error)
	GetMessageLinkInfo(ctx context.Context, url string) (*domain.MessageLinkInfo, error)

	// Операции с текстом.
	TranslateText(ctx context.Context, text *domain.FormattedText, lang string) (*domain.FormattedText, error)
	GetCallbackQueryAnswer(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, payload []byte) (string, error)
	ParseTextEntities(ctx context.Context, text string) (*domain.FormattedText, error)

	// Операции с чатами.
	LoadChats(ctx context.Context, limit int32) error
	WarmUpChat(ctx context.Context, chatID domain.ChatID, limit int32) error
	GetChatHistory(ctx context.Context, chatID domain.ChatID, fromMessageID domain.MessageID, offset int32, limit int32) ([]*domain.Message, error)
	GetChatType(ctx context.Context, chatID domain.ChatID) (string, error)

	// Операции с текстом (static TDLib methods).
	GetMarkdownText(ctx context.Context, text *domain.FormattedText) (*domain.FormattedText, error)

	// Системные операции.
	GetOption(ctx context.Context, name string) (string, error)
	GetMe(ctx context.Context) (int64, error)

	// Batch-операции.
	GetMessages(ctx context.Context, chatID domain.ChatID, messageIDs []domain.MessageID) ([]*domain.Message, error)

	// Управление чатами (stand).
	CreateNewSupergroupChat(ctx context.Context, title string, isChannel bool, description string) (domain.ChatID, int64, error)
	CreateNewBasicGroupChat(ctx context.Context, title string, userIDs []int64) (domain.ChatID, error)
	SetSupergroupUsername(ctx context.Context, supergroupID int64, username string) error
	DeleteChat(ctx context.Context, chatID domain.ChatID) error

	// Отправка данных авторизации.
	SubmitPhone(ctx context.Context, phone string) error
	SubmitCode(ctx context.Context, code string) error
	SubmitPassword(ctx context.Context, password string) error
}

var _ clientAdapter = (*Repo)(nil)

// SendMessage отправляет сообщение в чат.
func (r *Repo) SendMessage(_ context.Context, _ domain.ChatID, _ domain.InputMessageContent) (domain.MessageID, error) {
	return 0, nil
}

// SendMessageAlbum отправляет медиа-альбом.
func (r *Repo) SendMessageAlbum(_ context.Context, _ domain.ChatID, _ []domain.InputMessageContent) ([]domain.MessageID, error) {
	return nil, nil
}

// ForwardMessages пересылает сообщения из одного чата в другой.
func (r *Repo) ForwardMessages(_ context.Context, _, _ domain.ChatID, _ []domain.MessageID) ([]domain.MessageID, error) {
	return nil, nil
}

// GetMessage возвращает сообщение по ID.
func (r *Repo) GetMessage(_ context.Context, _ domain.ChatID, _ domain.MessageID) (*domain.Message, error) {
	return nil, nil
}

// EditMessageText редактирует текст сообщения.
func (r *Repo) EditMessageText(_ context.Context, _ domain.ChatID, _ domain.MessageID, _ *domain.FormattedText) error {
	return nil
}

// EditMessageCaption редактирует подпись медиа-сообщения.
func (r *Repo) EditMessageCaption(_ context.Context, _ domain.ChatID, _ domain.MessageID, _ *domain.FormattedText) error {
	return nil
}

// DeleteMessages удаляет сообщения из чата.
func (r *Repo) DeleteMessages(_ context.Context, _ domain.ChatID, _ []domain.MessageID, _ bool) error {
	return nil
}

// GetMessageLink возвращает ссылку на сообщение.
func (r *Repo) GetMessageLink(_ context.Context, _ domain.ChatID, _ domain.MessageID, _ bool) (string, error) {
	return "", nil
}

// GetMessageLinkInfo возвращает информацию о ссылке на сообщение.
func (r *Repo) GetMessageLinkInfo(_ context.Context, _ string) (*domain.MessageLinkInfo, error) {
	return nil, nil
}

// TranslateText переводит текст на указанный язык.
func (r *Repo) TranslateText(_ context.Context, _ *domain.FormattedText, _ string) (*domain.FormattedText, error) {
	return nil, nil
}

// GetCallbackQueryAnswer получает ответ на callback-запрос.
func (r *Repo) GetCallbackQueryAnswer(_ context.Context, _ domain.ChatID, _ domain.MessageID, _ []byte) (string, error) {
	return "", nil
}

// ParseTextEntities парсит текст с разметкой Markdown v2.
func (r *Repo) ParseTextEntities(_ context.Context, _ string) (*domain.FormattedText, error) {
	return &domain.FormattedText{Text: ""}, nil
}

// LoadChats загружает список чатов.
func (r *Repo) LoadChats(_ context.Context, _ int32) error {
	return nil
}

// WarmUpChat загружает историю чата для прогрева кеша.
func (r *Repo) WarmUpChat(_ context.Context, _ domain.ChatID, _ int32) error {
	return nil
}

// GetChatHistory возвращает сообщения чата с пагинацией.
func (r *Repo) GetChatHistory(_ context.Context, _ domain.ChatID, _ domain.MessageID, _ int32, _ int32) ([]*domain.Message, error) {
	return nil, nil
}

// GetChatType возвращает тип чата.
func (r *Repo) GetChatType(_ context.Context, _ domain.ChatID) (string, error) {
	return "supergroup", nil
}

// GetMarkdownText конвертирует FormattedText в Markdown-представление.
func (r *Repo) GetMarkdownText(_ context.Context, text *domain.FormattedText) (*domain.FormattedText, error) {
	return text, nil
}

// GetMessages возвращает сообщения по списку ID (batch).
func (r *Repo) GetMessages(_ context.Context, _ domain.ChatID, _ []domain.MessageID) ([]*domain.Message, error) {
	return nil, nil
}

// GetOption возвращает значение опции TDLib.
func (r *Repo) GetOption(_ context.Context, name string) (string, error) {
	if name == "version" {
		return "stub", nil
	}
	return "", nil
}

// GetMe возвращает информацию о текущем пользователе.
func (r *Repo) GetMe(_ context.Context) (int64, error) {
	return 0, nil
}

// CreateNewSupergroupChat создаёт новый канал или супергруппу.
// Возвращает chatID и supergroupID (из Chat.Type.(*ChatTypeSupergroup).SupergroupId).
func (r *Repo) CreateNewSupergroupChat(_ context.Context, _ string, _ bool, _ string) (domain.ChatID, int64, error) {
	return 0, 0, nil
}

// CreateNewBasicGroupChat создаёт новую базовую группу.
func (r *Repo) CreateNewBasicGroupChat(_ context.Context, _ string, _ []int64) (domain.ChatID, error) {
	return 0, nil
}

// SetSupergroupUsername устанавливает username для супергруппы или канала.
func (r *Repo) SetSupergroupUsername(_ context.Context, _ int64, _ string) error {
	return nil
}

// DeleteChat удаляет чат.
func (r *Repo) DeleteChat(_ context.Context, _ domain.ChatID) error {
	return nil
}

// SubmitPhone отправляет номер телефона для авторизации.
func (r *Repo) SubmitPhone(_ context.Context, phone string) error {
	r.logger.Info("Phone submitted", slog.String("phone", domain.MaskPhoneNumber(phone)))
	r.authStates <- domain.AuthStateEvent{State: domain.AuthStateWaitCode}
	return nil
}

// SubmitCode отправляет код подтверждения.
func (r *Repo) SubmitCode(_ context.Context, _ string) error {
	r.logger.Info("Code submitted")
	if r.has2FA {
		r.authStates <- domain.AuthStateEvent{
			State: domain.AuthStateWaitPassword,
			Extra: &domain.WaitPasswordState{PasswordHint: "2FA password"},
		}
	} else {
		r.authStates <- domain.AuthStateEvent{State: domain.AuthStateReady}
		close(r.clientDone)
		r.logger.Info("Authorization complete")
	}
	return nil
}

// SubmitPassword отправляет пароль двухфакторной аутентификации.
func (r *Repo) SubmitPassword(_ context.Context, _ string) error {
	r.logger.Info("Password submitted")
	r.authStates <- domain.AuthStateEvent{State: domain.AuthStateReady}
	close(r.clientDone)
	r.logger.Info("Authorization complete")
	return nil
}
