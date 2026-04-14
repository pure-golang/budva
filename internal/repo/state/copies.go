package state

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pure-golang/budva-claude/internal/domain"
)

const (
	copiedMessageIDsPrefix  = "copiedMsgIds"
	newMessageIDPrefix      = "newMsgId"
	tmpMessageIDPrefix      = "tmpMsgId"
	viewedMessagesPrefix    = "viewedMsgs"
	forwardedMessagesPrefix = "forwardedMsgs"
	answerMessageIDPrefix   = "answerMsgId"
)

// SetCopiedMessageID сохраняет связь между оригинальным и скопированным сообщением.
// toChatMessageID имеет формат "forwardRuleID:dstChatID:tmpMessageID".
func (r *Repo) SetCopiedMessageID(chatID domain.ChatID, messageID domain.MessageID, toChatMessageID string) error {
	lastPos := strings.LastIndex(toChatMessageID, ":")
	prefix := toChatMessageID[:lastPos+1]

	key := fmt.Sprintf("%s:%d:%d", copiedMessageIDsPrefix, chatID, messageID)
	_, err := r.GetSet(key, func(val string) (string, error) {
		var ss []string
		if val != "" {
			ss = strings.Split(val, ",")
		}
		found := false
		for i, s := range ss {
			if strings.HasPrefix(s, prefix) {
				found = true
				ss[i] = toChatMessageID
				break
			}
		}
		if !found {
			ss = append(ss, toChatMessageID)
		}
		return strings.Join(ss, ","), nil
	})
	return err
}

// GetCopiedMessageIDs возвращает идентификаторы скопированных сообщений.
// Каждый элемент имеет формат "forwardRuleID:dstChatID:tmpMessageID".
func (r *Repo) GetCopiedMessageIDs(chatID domain.ChatID, messageID domain.MessageID) []string {
	key := fmt.Sprintf("%s:%d:%d", copiedMessageIDsPrefix, chatID, messageID)
	val, err := r.Get(key)
	if err != nil || val == "" {
		return nil
	}
	return strings.Split(val, ",")
}

// DeleteCopiedMessageIDs удаляет связь между оригинальным и скопированными сообщениями.
func (r *Repo) DeleteCopiedMessageIDs(chatID domain.ChatID, messageID domain.MessageID) error {
	key := fmt.Sprintf("%s:%d:%d", copiedMessageIDsPrefix, chatID, messageID)
	return r.Delete(key)
}

// SetNewMessageID сохраняет маппинг temp→real ID сообщения.
func (r *Repo) SetNewMessageID(chatID domain.ChatID, tmpMessageID, newMessageID domain.MessageID) error {
	key := fmt.Sprintf("%s:%d:%d", newMessageIDPrefix, chatID, tmpMessageID)
	return r.Set(key, fmt.Sprintf("%d", newMessageID))
}

// GetNewMessageID возвращает real ID по temp ID (0 если не найден).
func (r *Repo) GetNewMessageID(chatID domain.ChatID, tmpMessageID domain.MessageID) domain.MessageID {
	key := fmt.Sprintf("%s:%d:%d", newMessageIDPrefix, chatID, tmpMessageID)
	val, err := r.Get(key)
	if err != nil || val == "" {
		return 0
	}
	id, _ := strconv.ParseInt(val, 10, 64)
	return id
}

// DeleteNewMessageID удаляет маппинг temp→real ID.
func (r *Repo) DeleteNewMessageID(chatID domain.ChatID, tmpMessageID domain.MessageID) error {
	key := fmt.Sprintf("%s:%d:%d", newMessageIDPrefix, chatID, tmpMessageID)
	return r.Delete(key)
}

// SetTmpMessageID сохраняет маппинг real→temp ID.
func (r *Repo) SetTmpMessageID(chatID domain.ChatID, newMessageID, tmpMessageID domain.MessageID) error {
	key := fmt.Sprintf("%s:%d:%d", tmpMessageIDPrefix, chatID, newMessageID)
	return r.Set(key, fmt.Sprintf("%d", tmpMessageID))
}

// GetTmpMessageID возвращает temp ID по real ID (0 если не найден).
func (r *Repo) GetTmpMessageID(chatID domain.ChatID, newMessageID domain.MessageID) domain.MessageID {
	key := fmt.Sprintf("%s:%d:%d", tmpMessageIDPrefix, chatID, newMessageID)
	val, err := r.Get(key)
	if err != nil || val == "" {
		return 0
	}
	id, _ := strconv.ParseInt(val, 10, 64)
	return id
}

// DeleteTmpMessageID удаляет маппинг real→temp ID.
func (r *Repo) DeleteTmpMessageID(chatID domain.ChatID, newMessageID domain.MessageID) error {
	key := fmt.Sprintf("%s:%d:%d", tmpMessageIDPrefix, chatID, newMessageID)
	return r.Delete(key)
}

// SetAnswerMessageID сохраняет ID сообщения-ответа.
func (r *Repo) SetAnswerMessageID(dstChatID domain.ChatID, tmpMessageID domain.MessageID, srcChatID domain.ChatID, srcMessageID domain.MessageID) error {
	val := fmt.Sprintf("%d:%d", srcChatID, srcMessageID)
	key := fmt.Sprintf("%s:%d:%d", answerMessageIDPrefix, dstChatID, tmpMessageID)
	return r.Set(key, val)
}

// GetAnswerMessageID возвращает "srcChatID:srcMessageID" по dst-координатам.
func (r *Repo) GetAnswerMessageID(dstChatID domain.ChatID, tmpMessageID domain.MessageID) string {
	key := fmt.Sprintf("%s:%d:%d", answerMessageIDPrefix, dstChatID, tmpMessageID)
	val, _ := r.Get(key)
	return val
}

// DeleteAnswerMessageID удаляет ID сообщения-ответа.
func (r *Repo) DeleteAnswerMessageID(dstChatID domain.ChatID, tmpMessageID domain.MessageID) error {
	key := fmt.Sprintf("%s:%d:%d", answerMessageIDPrefix, dstChatID, tmpMessageID)
	return r.Delete(key)
}

// IncrementViewedMessages увеличивает счётчик просмотренных сообщений.
func (r *Repo) IncrementViewedMessages(toChatID domain.ChatID, date string) (uint64, error) {
	key := fmt.Sprintf("%s:%d:%s", viewedMessagesPrefix, toChatID, date)
	return r.Increment(key)
}

// IncrementForwardedMessages увеличивает счётчик пересланных сообщений.
func (r *Repo) IncrementForwardedMessages(toChatID domain.ChatID, date string) (uint64, error) {
	key := fmt.Sprintf("%s:%d:%s", forwardedMessagesPrefix, toChatID, date)
	return r.Increment(key)
}
