package state

import (
	"context"
	"encoding/binary"
	"errors"
	"log/slog"
	"time"

	"github.com/dgraph-io/badger/v4"

	"github.com/pure-golang/budva-claude/internal/config"
)

// ErrKeyNotFound означает, что ключ не найден.
var ErrKeyNotFound = errors.New("key not found")

// Repo реализует KV-хранилище на основе BadgerDB.
type Repo struct {
	logger *slog.Logger
	cfg    config.StorageConfig
	db     *badger.DB
	stop   chan struct{}
}

// New создаёт новый экземпляр хранилища.
func New(cfg config.StorageConfig) *Repo {
	return &Repo{
		logger: slog.Default().With("module", "infra.state"),
		cfg:    cfg,
	}
}

// Start открывает базу данных и запускает фоновую сборку мусора.
func (r *Repo) Start(_ context.Context) error {
	opts := badger.DefaultOptions(r.cfg.DatabaseDirectory)
	opts.Logger = newBadgerLogger(r.logger)
	db, err := badger.Open(opts)
	if err != nil {
		return err
	}
	r.db = db
	r.stop = make(chan struct{})

	go r.runGC()
	return nil
}

// Close останавливает GC и закрывает базу данных.
func (r *Repo) Close() error {
	if r.stop != nil {
		close(r.stop)
	}
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// Get возвращает значение по ключу.
func (r *Repo) Get(key string) (string, error) {
	var val string
	err := r.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		valBytes, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		val = string(valBytes)
		return nil
	})
	return val, err
}

// Set устанавливает значение по ключу.
func (r *Repo) Set(key, val string) error {
	return r.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), []byte(val))
	})
}

// Delete удаляет значение по ключу.
func (r *Repo) Delete(key string) error {
	return r.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// GetSet выполняет атомарное чтение-изменение-запись в одной транзакции.
func (r *Repo) GetSet(key string, fn func(val string) (string, error)) (string, error) {
	var val string
	err := r.db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		var current string
		if err != nil && err != badger.ErrKeyNotFound {
			return err
		}
		if err != badger.ErrKeyNotFound {
			valBytes, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			current = string(valBytes)
		}
		val, err = fn(current)
		if err != nil {
			return err
		}
		return txn.Set([]byte(key), []byte(val))
	})
	return val, err
}

// Increment атомарно увеличивает uint64-значение по ключу на 1.
func (r *Repo) Increment(key string) (uint64, error) {
	add := func(existing, newVal []byte) []byte {
		return uint64ToBytes(bytesToUint64(existing) + bytesToUint64(newVal))
	}
	m := r.db.GetMergeOperator([]byte(key), add, 200*time.Millisecond)
	defer m.Stop()

	if err := m.Add(uint64ToBytes(1)); err != nil {
		return 0, err
	}
	val, err := m.Get()
	if err != nil {
		return 0, err
	}
	return bytesToUint64(val), nil
}

// IsKeyNotFound проверяет, является ли ошибка отсутствием ключа.
func IsKeyNotFound(err error) bool {
	return errors.Is(err, ErrKeyNotFound) || err == badger.ErrKeyNotFound
}

// Ping проверяет доступность хранилища.
func (r *Repo) Ping(_ context.Context) error {
	_, err := r.Get("__ping__")
	if IsKeyNotFound(err) {
		return nil
	}
	return err
}

func (r *Repo) runGC() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			for {
				err := r.db.RunValueLogGC(0.7)
				if err == nil {
					continue
				}
				if err != badger.ErrNoRewrite {
					r.logger.Error("BadgerDB GC error", slog.Any("err", err))
				}
				break
			}
		}
	}
}

func uint64ToBytes(i uint64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], i)
	return buf[:]
}

func bytesToUint64(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return binary.BigEndian.Uint64(b)
}
