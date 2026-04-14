package album

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAddMessage_FirstReturnsTrue(t *testing.T) {
	t.Parallel()
	svc := New()

	isFirst := svc.AddMessage("album:1", 100)
	assert.True(t, isFirst)
}

func TestAddMessage_SecondReturnsFalse(t *testing.T) {
	t.Parallel()
	svc := New()

	svc.AddMessage("album:1", 100)
	isFirst := svc.AddMessage("album:1", 101)
	assert.False(t, isFirst)
}

func TestAddMessage_DifferentKeys(t *testing.T) {
	t.Parallel()
	svc := New()

	assert.True(t, svc.AddMessage("album:1", 100))
	assert.True(t, svc.AddMessage("album:2", 200))
}

func TestPopMessages(t *testing.T) {
	t.Parallel()
	svc := New()

	svc.AddMessage("album:1", 100)
	svc.AddMessage("album:1", 101)
	svc.AddMessage("album:1", 102)

	ids := svc.PopMessages("album:1")
	assert.Equal(t, []int64{100, 101, 102}, ids)
}

func TestPopMessages_RemovesAlbum(t *testing.T) {
	t.Parallel()
	svc := New()

	svc.AddMessage("album:1", 100)
	svc.PopMessages("album:1")

	ids := svc.PopMessages("album:1")
	assert.Nil(t, ids)
}

func TestPopMessages_EmptyKey(t *testing.T) {
	t.Parallel()
	svc := New()

	ids := svc.PopMessages("nonexistent")
	assert.Nil(t, ids)
}

func TestLastReceivedAge(t *testing.T) {
	t.Parallel()
	svc := New()

	svc.AddMessage("album:1", 100)
	time.Sleep(10 * time.Millisecond)

	age := svc.LastReceivedAge("album:1")
	assert.GreaterOrEqual(t, age, 10*time.Millisecond)
}

func TestLastReceivedAge_NonexistentKey(t *testing.T) {
	t.Parallel()
	svc := New()

	age := svc.LastReceivedAge("nonexistent")
	assert.Equal(t, time.Duration(0), age)
}

func TestLastReceivedAge_UpdatedOnNewMessage(t *testing.T) {
	t.Parallel()
	svc := New()

	svc.AddMessage("album:1", 100)
	time.Sleep(20 * time.Millisecond)
	svc.AddMessage("album:1", 101)

	age := svc.LastReceivedAge("album:1")
	assert.Less(t, age, 20*time.Millisecond)
}

func TestMakeKey(t *testing.T) {
	t.Parallel()

	key := MakeKey("rule_1", 12345)
	assert.Equal(t, "rule_1:12345", key)
}
