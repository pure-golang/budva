package grpc

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	alogger "github.com/pure-golang/adapters/logger"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/transport/grpc/pb"
)

type facadeService interface {
	GetMessage(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) (*domain.Message, error)
	GetChatHistory(ctx context.Context, chatID domain.ChatID, fromMessageID domain.MessageID, offset, limit int32) ([]*domain.Message, error)
	SendMessage(ctx context.Context, chatID domain.ChatID, text string) error
	SendMessageAlbum(ctx context.Context, chatID domain.ChatID, texts []string) error
	ForwardMessage(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) error
	UpdateMessage(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, text string) error
	DeleteMessages(ctx context.Context, chatID domain.ChatID, messageIDs []domain.MessageID) error
	GetMessageLink(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) (string, error)
	GetMessageLinkInfo(ctx context.Context, link string) (*domain.MessageLinkInfo, error)
}

// Transport реализует gRPC-сервер FacadeGRPC.
type Transport struct {
	pb.UnimplementedFacadeGRPCServer
	facade facadeService
}

// New создаёт новый экземпляр gRPC-транспорта.
func New(facade facadeService) *Transport {
	return &Transport{
		facade: facade,
	}
}

// GetMessages возвращает сообщения по списку ID.
func (t *Transport) GetMessages(ctx context.Context, req *pb.GetMessagesRequest) (*pb.MessagesResponse, error) {
	var messages []*pb.Message
	for _, id := range req.GetMessageIds() {
		msg, err := t.facade.GetMessage(ctx, req.GetChatId(), id)
		if err != nil {
			alogger.FromContext(ctx).Error("Failed to get message", slog.Any("err", err), slog.Int64("message_id", id))
			continue
		}
		messages = append(messages, domainToProto(msg))
	}
	return &pb.MessagesResponse{Messages: messages}, nil
}

// GetChatHistory возвращает историю чата с пагинацией.
func (t *Transport) GetChatHistory(ctx context.Context, req *pb.GetChatHistoryRequest) (*pb.MessagesResponse, error) {
	msgs, err := t.facade.GetChatHistory(ctx, req.GetChatId(), req.GetFromMessageId(), req.GetOffset(), req.GetLimit())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	var messages []*pb.Message
	for _, msg := range msgs {
		messages = append(messages, domainToProto(msg))
	}
	return &pb.MessagesResponse{Messages: messages}, nil
}

// SendMessage отправляет сообщение.
func (t *Transport) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.EmptyResponse, error) {
	msg := req.GetMessage()
	if msg == nil {
		return nil, status.Error(codes.InvalidArgument, "message is required")
	}
	if err := t.facade.SendMessage(ctx, msg.GetChatId(), msg.GetText()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.EmptyResponse{}, nil
}

// SendMessageAlbum отправляет несколько сообщений как альбом.
func (t *Transport) SendMessageAlbum(ctx context.Context, req *pb.SendMessageAlbumRequest) (*pb.EmptyResponse, error) {
	msgs := req.GetMessages()
	if len(msgs) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one message is required")
	}
	chatID := msgs[0].GetChatId()
	var texts []string
	for _, m := range msgs {
		texts = append(texts, m.GetText())
	}
	if err := t.facade.SendMessageAlbum(ctx, chatID, texts); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.EmptyResponse{}, nil
}

// ForwardMessage пересылает сообщение.
func (t *Transport) ForwardMessage(ctx context.Context, req *pb.ForwardMessageRequest) (*pb.EmptyResponse, error) {
	if err := t.facade.ForwardMessage(ctx, req.GetChatId(), req.GetMessageId()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.EmptyResponse{}, nil
}

// GetMessage возвращает одно сообщение.
func (t *Transport) GetMessage(ctx context.Context, req *pb.GetMessageRequest) (*pb.MessageResponse, error) {
	msg, err := t.facade.GetMessage(ctx, req.GetChatId(), req.GetMessageId())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.MessageResponse{Message: domainToProto(msg)}, nil
}

// UpdateMessage обновляет текст сообщения.
func (t *Transport) UpdateMessage(ctx context.Context, req *pb.UpdateMessageRequest) (*pb.EmptyResponse, error) {
	msg := req.GetMessage()
	if msg == nil {
		return nil, status.Error(codes.InvalidArgument, "message is required")
	}
	if err := t.facade.UpdateMessage(ctx, msg.GetChatId(), msg.GetId(), msg.GetText()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.EmptyResponse{}, nil
}

// DeleteMessages удаляет сообщения.
func (t *Transport) DeleteMessages(ctx context.Context, req *pb.DeleteMessagesRequest) (*pb.EmptyResponse, error) {
	if err := t.facade.DeleteMessages(ctx, req.GetChatId(), req.GetMessageIds()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.EmptyResponse{}, nil
}

// GetMessageLink возвращает публичную ссылку на сообщение.
func (t *Transport) GetMessageLink(ctx context.Context, req *pb.GetMessageLinkRequest) (*pb.MessageLinkResponse, error) {
	link, err := t.facade.GetMessageLink(ctx, req.GetChatId(), req.GetMessageId())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.MessageLinkResponse{Link: link}, nil
}

// GetMessageLinkInfo извлекает информацию о сообщении по ссылке.
func (t *Transport) GetMessageLinkInfo(ctx context.Context, req *pb.GetMessageLinkInfoRequest) (*pb.MessageResponse, error) {
	info, err := t.facade.GetMessageLinkInfo(ctx, req.GetLink())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.MessageResponse{
		Message: &pb.Message{
			Id:     info.MessageID,
			ChatId: info.ChatID,
		},
	}, nil
}

func domainToProto(msg *domain.Message) *pb.Message {
	if msg == nil {
		return nil
	}
	text := ""
	if msg.Content.Text != nil {
		text = msg.Content.Text.Text
	}
	forward := msg.ForwardInfo != nil
	return &pb.Message{
		Id:      msg.ID,
		ChatId:  msg.ChatID,
		Text:    text,
		Forward: forward,
	}
}
