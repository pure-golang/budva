package album

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAddMessage_FirstReturnsTrue(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New()

	// Act
	isFirst := svc.AddMessage("album:1", 100)

	// Assert
	assert.True(t, isFirst)
}

func TestAddMessage_SecondReturnsFalse(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New()
	svc.AddMessage("album:1", 100)

	// Act
	isFirst := svc.AddMessage("album:1", 101)

	// Assert
	assert.False(t, isFirst)
}

func TestAddMessage_DifferentKeys(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New()

	// Act + Assert
	assert.True(t, svc.AddMessage("album:1", 100))
	assert.True(t, svc.AddMessage("album:2", 200))
}

func TestPopMessages(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New()
	svc.AddMessage("album:1", 100)
	svc.AddMessage("album:1", 101)
	svc.AddMessage("album:1", 102)

	// Act
	ids := svc.PopMessages("album:1")

	// Assert
	assert.Equal(t, []int64{100, 101, 102}, ids)
}

func TestPopMessages_RemovesAlbum(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New()
	svc.AddMessage("album:1", 100)
	svc.PopMessages("album:1")

	// Act
	ids := svc.PopMessages("album:1")

	// Assert
	assert.Nil(t, ids)
}

func TestPopMessages_EmptyKey(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New()

	// Act
	ids := svc.PopMessages("nonexistent")

	// Assert
	assert.Nil(t, ids)
}

func TestLastReceivedAge(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New()
	svc.AddMessage("album:1", 100)
	time.Sleep(10 * time.Millisecond)

	// Act
	age := svc.LastReceivedAge("album:1")

	// Assert
	assert.GreaterOrEqual(t, age, 10*time.Millisecond)
}

func TestLastReceivedAge_NonexistentKey(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New()

	// Act
	age := svc.LastReceivedAge("nonexistent")

	// Assert
	assert.Equal(t, time.Duration(0), age)
}

func TestLastReceivedAge_UpdatedOnNewMessage(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New()
	svc.AddMessage("album:1", 100)
	time.Sleep(20 * time.Millisecond)
	svc.AddMessage("album:1", 101)

	// Act
	age := svc.LastReceivedAge("album:1")

	// Assert
	assert.Less(t, age, 20*time.Millisecond)
}

func TestMakeKey(t *testing.T) {
	t.Parallel()

	// Act
	key := MakeKey("rule_1", 12345)

	// Assert
	assert.Equal(t, "rule_1:12345", key)
}
