package grpc

import (
	"context"
	"log/slog"

	alogger "github.com/pure-golang/adapters/logger"
	"github.com/zelenin/go-tdlib/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/transport/grpc/pb"
)

// facadeService — частично применяемый интерфейс к service/facade.
// Все методы вернут raw-TDLib типы, которые gRPC-слой сворачивает в свой proto-DTO.
type facadeService interface {
	GetMessage(ctx context.Context, chatID int64, messageID int64) (*client.Message, error)
	GetMessages(ctx context.Context, chatID int64, messageIDs []int64) ([]*client.Message, error)
	GetChatHistory(ctx context.Context, chatID int64, fromMessageID int64, offset, limit int32) ([]*client.Message, error)
	SendMessage(ctx context.Context, chatID int64, text string) error
	SendMessageAlbum(ctx context.Context, chatID int64, items []domain.AlbumItem) error
	ForwardMessage(ctx context.Context, chatID int64, messageID int64) error
	UpdateMessage(ctx context.Context, chatID int64, messageID int64, text string) error
	DeleteMessages(ctx context.Context, chatID int64, messageIDs []int64) error
	GetMessageLink(ctx context.Context, chatID int64, messageID int64) (string, error)
	GetMessageLinkInfo(ctx context.Context, link string) (*client.MessageLinkInfo, error)
}

// Transport реализует gRPC-сервер FacadeGRPC.
type Transport struct {
	pb.UnimplementedFacadeGRPCServer
	facadeService facadeService
}

// New создаёт новый экземпляр gRPC-транспорта.
func New(facadeService facadeService) *Transport {
	return &Transport{facadeService: facadeService}
}

// GetMessages возвращает сообщения по списку ID (batch).
func (t *Transport) GetMessages(ctx context.Context, req *pb.GetMessagesRequest) (*pb.MessagesResponse, error) {
	msgs, err := t.facadeService.GetMessages(ctx, req.GetChatId(), req.GetMessageIds())
	if err != nil {
		alogger.FromContext(ctx).Error("Failed to get messages", slog.Any("err", err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	var messages []*pb.Message
	for _, msg := range msgs {
		if msg != nil {
			messages = append(messages, clientMessageToProto(msg))
		}
	}
	return &pb.MessagesResponse{Messages: messages}, nil
}

// GetChatHistory возвращает историю чата с пагинацией.
func (t *Transport) GetChatHistory(ctx context.Context, req *pb.GetChatHistoryRequest) (*pb.MessagesResponse, error) {
	msgs, err := t.facadeService.GetChatHistory(ctx, req.GetChatId(), req.GetFromMessageId(), req.GetOffset(), req.GetLimit())
	if err != nil {
		alogger.FromContext(ctx).Error("Failed to get chat history", slog.Any("err", err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	var messages []*pb.Message
	for _, msg := range msgs {
		messages = append(messages, clientMessageToProto(msg))
	}
	return &pb.MessagesResponse{Messages: messages}, nil
}

// SendMessage отправляет сообщение.
func (t *Transport) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.EmptyResponse, error) {
	msg := req.GetMessage()
	if msg == nil {
		return nil, status.Error(codes.InvalidArgument, "message is required")
	}
	if err := t.facadeService.SendMessage(ctx, msg.GetChatId(), msg.GetText()); err != nil {
		alogger.FromContext(ctx).Error("Failed to send message", slog.Any("err", err))
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
	items := make([]domain.AlbumItem, 0, len(msgs))
	for _, m := range msgs {
		items = append(items, domain.AlbumItem{
			Text:     m.GetText(),
			FilePath: m.GetFilePath(),
		})
	}
	if err := t.facadeService.SendMessageAlbum(ctx, chatID, items); err != nil {
		alogger.FromContext(ctx).Error("Failed to send message album", slog.Any("err", err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.EmptyResponse{}, nil
}

// ForwardMessage пересылает сообщение.
func (t *Transport) ForwardMessage(ctx context.Context, req *pb.ForwardMessageRequest) (*pb.EmptyResponse, error) {
	if err := t.facadeService.ForwardMessage(ctx, req.GetChatId(), req.GetMessageId()); err != nil {
		alogger.FromContext(ctx).Error("Failed to forward message", slog.Any("err", err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.EmptyResponse{}, nil
}

// GetMessage возвращает одно сообщение.
func (t *Transport) GetMessage(ctx context.Context, req *pb.GetMessageRequest) (*pb.MessageResponse, error) {
	msg, err := t.facadeService.GetMessage(ctx, req.GetChatId(), req.GetMessageId())
	if err != nil {
		alogger.FromContext(ctx).Error("Failed to get message", slog.Any("err", err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.MessageResponse{Message: clientMessageToProto(msg)}, nil
}

// UpdateMessage обновляет текст сообщения.
func (t *Transport) UpdateMessage(ctx context.Context, req *pb.UpdateMessageRequest) (*pb.EmptyResponse, error) {
	msg := req.GetMessage()
	if msg == nil {
		return nil, status.Error(codes.InvalidArgument, "message is required")
	}
	if err := t.facadeService.UpdateMessage(ctx, msg.GetChatId(), msg.GetId(), msg.GetText()); err != nil {
		alogger.FromContext(ctx).Error("Failed to update message", slog.Any("err", err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.EmptyResponse{}, nil
}

// DeleteMessages удаляет сообщения.
func (t *Transport) DeleteMessages(ctx context.Context, req *pb.DeleteMessagesRequest) (*pb.EmptyResponse, error) {
	if err := t.facadeService.DeleteMessages(ctx, req.GetChatId(), req.GetMessageIds()); err != nil {
		alogger.FromContext(ctx).Error("Failed to delete messages", slog.Any("err", err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.EmptyResponse{}, nil
}

// GetMessageLink возвращает публичную ссылку на сообщение.
func (t *Transport) GetMessageLink(ctx context.Context, req *pb.GetMessageLinkRequest) (*pb.MessageLinkResponse, error) {
	link, err := t.facadeService.GetMessageLink(ctx, req.GetChatId(), req.GetMessageId())
	if err != nil {
		alogger.FromContext(ctx).Error("Failed to get message link", slog.Any("err", err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.MessageLinkResponse{Link: link}, nil
}

// GetMessageLinkInfo извлекает информацию о сообщении по ссылке.
func (t *Transport) GetMessageLinkInfo(ctx context.Context, req *pb.GetMessageLinkInfoRequest) (*pb.MessageResponse, error) {
	info, err := t.facadeService.GetMessageLinkInfo(ctx, req.GetLink())
	if err != nil {
		alogger.FromContext(ctx).Error("Failed to get message link info", slog.Any("err", err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	var messageID int64
	if info != nil && info.Message != nil {
		messageID = info.Message.Id
	}
	var chatID int64
	if info != nil {
		chatID = info.ChatId
	}
	return &pb.MessageResponse{
		Message: &pb.Message{
			Id:     messageID,
			ChatId: chatID,
		},
	}, nil
}

// clientMessageToProto маппит *client.Message (raw TDLib) в pb.Message.
func clientMessageToProto(msg *client.Message) *pb.Message {
	if msg == nil {
		return nil
	}
	var text string
	switch c := msg.Content.(type) {
	case *client.MessageText:
		if c.Text != nil {
			text = c.Text.Text
		}
	case *client.MessagePhoto:
		if c.Caption != nil {
			text = c.Caption.Text
		}
	case *client.MessageVideo:
		if c.Caption != nil {
			text = c.Caption.Text
		}
	case *client.MessageDocument:
		if c.Caption != nil {
			text = c.Caption.Text
		}
	}
	return &pb.Message{
		Id:      msg.Id,
		ChatId:  msg.ChatId,
		Text:    text,
		Forward: msg.ForwardInfo != nil,
	}
}
