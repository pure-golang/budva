package album_test

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/service/album"
)

func TestAddMessage_FirstReturnsTrue(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()

	// Act
	isFirst := svc.AddMessage("album:1", &client.Message{Id: 100})

	// Assert
	assert.True(t, isFirst)
}

func TestAddMessage_SecondReturnsFalse(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()
	svc.AddMessage("album:1", &client.Message{Id: 100})

	// Act
	isFirst := svc.AddMessage("album:1", &client.Message{Id: 101})

	// Assert
	assert.False(t, isFirst)
}

func TestAddMessage_DifferentKeysEachFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()

	// Act
	first1 := svc.AddMessage("album:1", &client.Message{Id: 100})
	first2 := svc.AddMessage("album:2", &client.Message{Id: 200})

	// Assert
	assert.True(t, first1)
	assert.True(t, first2)
}

func TestAddMessage_NilMessageStored(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()

	// Act
	isFirst := svc.AddMessage("album:1", nil)
	msgs := svc.PopMessages("album:1")

	// Assert
	assert.True(t, isFirst)
	require.Len(t, msgs, 1)
	assert.Nil(t, msgs[0])
}

func TestAddMessage_EmptyKey(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()

	// Act
	isFirst1 := svc.AddMessage("", &client.Message{Id: 1})
	isFirst2 := svc.AddMessage("", &client.Message{Id: 2})

	// Assert
	assert.True(t, isFirst1)
	assert.False(t, isFirst2)
}

func TestAddMessage_PreservesOrder(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()
	ids := []int64{10, 20, 30, 40, 50}
	for _, id := range ids {
		svc.AddMessage("album:1", &client.Message{Id: id})
	}

	// Act
	msgs := svc.PopMessages("album:1")

	// Assert
	require.Len(t, msgs, len(ids))
	for i, id := range ids {
		assert.Equal(t, id, msgs[i].Id, "message at index %d", i)
	}
}

func TestPopMessages_ReturnsAllInOrder(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()
	svc.AddMessage("album:1", &client.Message{Id: 100})
	svc.AddMessage("album:1", &client.Message{Id: 101})
	svc.AddMessage("album:1", &client.Message{Id: 102})

	// Act
	msgs := svc.PopMessages("album:1")

	// Assert
	require.Len(t, msgs, 3)
	assert.Equal(t, int64(100), msgs[0].Id)
	assert.Equal(t, int64(101), msgs[1].Id)
	assert.Equal(t, int64(102), msgs[2].Id)
}

func TestPopMessages_RemovesAlbum(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()
	svc.AddMessage("album:1", &client.Message{Id: 100})
	_ = svc.PopMessages("album:1")

	// Act
	msgs := svc.PopMessages("album:1")

	// Assert
	assert.Nil(t, msgs)
}

func TestPopMessages_NonexistentKey(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()

	// Act
	msgs := svc.PopMessages("nonexistent")

	// Assert
	assert.Nil(t, msgs)
}

func TestPopMessages_EmptyStringKey(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()

	// Act
	msgs := svc.PopMessages("")

	// Assert
	assert.Nil(t, msgs)
}

func TestPopMessages_IsolatesKeys(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()
	svc.AddMessage("album:1", &client.Message{Id: 1})
	svc.AddMessage("album:2", &client.Message{Id: 2})

	// Act
	msgs1 := svc.PopMessages("album:1")
	msgs2 := svc.PopMessages("album:2")

	// Assert
	require.Len(t, msgs1, 1)
	require.Len(t, msgs2, 1)
	assert.Equal(t, int64(1), msgs1[0].Id)
	assert.Equal(t, int64(2), msgs2[0].Id)
}

func TestPopMessages_AllowsReuseOfKey(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()
	svc.AddMessage("album:1", &client.Message{Id: 1})
	_ = svc.PopMessages("album:1")

	// Act
	isFirst := svc.AddMessage("album:1", &client.Message{Id: 2})
	msgs := svc.PopMessages("album:1")

	// Assert
	assert.True(t, isFirst, "after PopMessages key must be treated as new")
	require.Len(t, msgs, 1)
	assert.Equal(t, int64(2), msgs[0].Id)
}

func TestLastReceivedAge_AfterAdd(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()
	svc.AddMessage("album:1", &client.Message{Id: 100})
	time.Sleep(10 * time.Millisecond)

	// Act
	age := svc.LastReceivedAge("album:1")

	// Assert
	assert.GreaterOrEqual(t, age, 10*time.Millisecond)
}

func TestLastReceivedAge_NonexistentKey(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()

	// Act
	age := svc.LastReceivedAge("nonexistent")

	// Assert
	assert.Equal(t, time.Duration(0), age)
}

func TestLastReceivedAge_AfterPopReturnsZero(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()
	svc.AddMessage("album:1", &client.Message{Id: 100})
	_ = svc.PopMessages("album:1")

	// Act
	age := svc.LastReceivedAge("album:1")

	// Assert
	assert.Equal(t, time.Duration(0), age)
}

func TestLastReceivedAge_UpdatedOnNewMessage(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()
	svc.AddMessage("album:1", &client.Message{Id: 100})
	time.Sleep(30 * time.Millisecond)
	svc.AddMessage("album:1", &client.Message{Id: 101})

	// Act
	age := svc.LastReceivedAge("album:1")

	// Assert
	assert.Less(t, age, 30*time.Millisecond)
}

func TestLastReceivedAge_MonotonicGrowthBetweenAdds(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()
	svc.AddMessage("album:1", &client.Message{Id: 100})
	time.Sleep(5 * time.Millisecond)
	age1 := svc.LastReceivedAge("album:1")
	time.Sleep(10 * time.Millisecond)

	// Act
	age2 := svc.LastReceivedAge("album:1")

	// Assert
	assert.Greater(t, age2, age1)
}

func TestMakeKey(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		ruleID       domain.ForwardRuleID
		mediaAlbumID int64
		expected     domain.MediaAlbumKey
	}{
		{
			name:         "simple_rule_positive_id",
			ruleID:       "rule_1",
			mediaAlbumID: 12345,
			expected:     "rule_1:12345",
		},
		{
			name:         "empty_rule",
			ruleID:       "",
			mediaAlbumID: 42,
			expected:     ":42",
		},
		{
			name:         "zero_album_id",
			ruleID:       "rule_1",
			mediaAlbumID: 0,
			expected:     "rule_1:0",
		},
		{
			name:         "negative_album_id",
			ruleID:       "rule_1",
			mediaAlbumID: -99,
			expected:     "rule_1:-99",
		},
		{
			name:         "rule_with_colon",
			ruleID:       "rule:with:colons",
			mediaAlbumID: 1,
			expected:     "rule:with:colons:1",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			key := album.MakeKey(tt.ruleID, tt.mediaAlbumID)

			// Assert
			assert.Equal(t, tt.expected, key)
		})
	}
}

func TestMakeKey_UsedByService(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()
	key := album.MakeKey("rule_1", 42)
	svc.AddMessage(key, &client.Message{Id: 7})

	// Act
	msgs := svc.PopMessages(key)

	// Assert
	require.Len(t, msgs, 1)
	assert.Equal(t, int64(7), msgs[0].Id)
}

func TestService_ConcurrentAddMessage_DifferentKeys(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()
	const goroutines = 16
	const perGoroutine = 32

	// Act
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			key := "album:" + strconv.Itoa(g)
			for i := 0; i < perGoroutine; i++ {
				svc.AddMessage(key, &client.Message{Id: int64(g*1000 + i)})
			}
		}()
	}
	wg.Wait()

	// Assert
	for g := 0; g < goroutines; g++ {
		key := "album:" + strconv.Itoa(g)
		msgs := svc.PopMessages(key)
		assert.Len(t, msgs, perGoroutine, "key=%s", key)
	}
}

func TestService_ConcurrentAddMessage_SameKey(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()
	const goroutines = 32
	const perGoroutine = 16

	// Act
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				svc.AddMessage("album:shared", &client.Message{Id: int64(g*1000 + i)})
			}
		}()
	}
	wg.Wait()

	// Assert
	msgs := svc.PopMessages("album:shared")
	assert.Len(t, msgs, goroutines*perGoroutine)
}

func TestService_ConcurrentMixedOperations(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := album.New()
	const workers = 8
	const iterations = 50

	// Act
	var wg sync.WaitGroup
	wg.Add(workers * 3)
	for w := 0; w < workers; w++ {
		key := "album:" + strconv.Itoa(w)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				svc.AddMessage(key, &client.Message{Id: int64(i)})
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = svc.LastReceivedAge(key)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = svc.PopMessages(key)
			}
		}()
	}
	wg.Wait()

	// Assert
	// Race detector must not report data races; final state is non-deterministic.
	_ = svc.PopMessages("album:any")
}
