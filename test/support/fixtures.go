package support

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pure-golang/budva-claude/internal/domain"
)

// ChatFixture описывает один тестовый чат из stand.json.
// ChatType — TDLib-совместимый: "supergroup" или "basicGroup".
// Каналы — это supergroup с IsChannel=true (как ChatTypeSupergroup в TDLib).
type ChatFixture struct {
	Name         string        `json:"name"`
	ChatID       domain.ChatID `json:"chat_id"`
	SupergroupID int64         `json:"supergroup_id,omitempty"`
	ChatType     string        `json:"chat_type"`
	IsChannel    bool          `json:"is_channel,omitempty"`
	Username     string        `json:"username,omitempty"`
}

// Fixtures содержит набор тестовых чатов, загруженных из stand.json.
type Fixtures struct {
	Chats  []ChatFixture `json:"chats"`
	byName map[string]*ChatFixture
}

// LoadFixtures загружает фикстуры из JSON-файла.
func LoadFixtures(path string) (*Fixtures, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixtures: %w", err)
	}
	var f Fixtures
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse fixtures: %w", err)
	}
	f.buildIndex()
	return &f, nil
}

// ChatByName возвращает фикстуру по абстрактному имени из BDD Examples.
func (f *Fixtures) ChatByName(name string) (ChatFixture, error) {
	if fix, ok := f.byName[name]; ok {
		return *fix, nil
	}
	return ChatFixture{}, fmt.Errorf("fixture %q not found", name)
}

func (f *Fixtures) buildIndex() {
	f.byName = make(map[string]*ChatFixture, len(f.Chats))
	for i := range f.Chats {
		f.byName[f.Chats[i].Name] = &f.Chats[i]
	}
}

// SaveFixtures записывает фикстуры в JSON-файл и строит индекс для ChatByName.
func SaveFixtures(path string, f *Fixtures) error {
	f.buildIndex()
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal fixtures: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// ApplyToFakeTelegram регистрирует типы чатов и supergroupID из фикстур в FakeTelegram.
func (f *Fixtures) ApplyToFakeTelegram(ft *FakeTelegram) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	for _, chat := range f.Chats {
		ft.chatTypes[chat.ChatID] = chat.ChatType
		if chat.SupergroupID != 0 {
			ft.supergroupIDs[chat.ChatID] = chat.SupergroupID
		}
	}
}
