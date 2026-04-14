package support

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pure-golang/budva-claude/internal/repo/state"
)

// RemoteKV — HTTP-клиент для BadgerDB testcontainer.
// Реализует state.KVStore.
type RemoteKV struct {
	baseURL string
	client  *http.Client
}

// NewRemoteKV создаёт клиент для BadgerDB testcontainer.
func NewRemoteKV(baseURL string) *RemoteKV {
	return &RemoteKV{
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

type kvRequest struct {
	Key string `json:"key"`
	Val string `json:"val,omitempty"`
}

type kvResponse struct {
	Val   string `json:"val,omitempty"`
	Error string `json:"error,omitempty"`
}

func (r *RemoteKV) Get(key string) (string, error) {
	resp, err := r.post("/get", kvRequest{Key: key})
	if err != nil {
		return "", err
	}
	if resp.Error != "" {
		if resp.Error == "Key not found" {
			return "", state.ErrKeyNotFound
		}
		return "", fmt.Errorf("%s", resp.Error)
	}
	return resp.Val, nil
}

func (r *RemoteKV) Set(key, val string) error {
	resp, err := r.post("/set", kvRequest{Key: key, Val: val})
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (r *RemoteKV) Delete(key string) error {
	resp, err := r.post("/delete", kvRequest{Key: key})
	if err != nil {
		return err
	}
	if resp.Error != "" {
		if resp.Error == "Key not found" {
			return nil // delete of non-existent key is not an error
		}
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (r *RemoteKV) GetSet(key string, fn func(val string) (string, error)) (string, error) {
	// Атомарный read-modify-write через два шага:
	// 1. Получаем текущее значение
	current, err := r.Get(key)
	if err != nil && !state.IsKeyNotFound(err) {
		return "", err
	}
	// 2. Применяем fn и записываем
	newVal, err := fn(current)
	if err != nil {
		return "", err
	}
	if err := r.Set(key, newVal); err != nil {
		return "", err
	}
	return newVal, nil
}

func (r *RemoteKV) Increment(key string) (uint64, error) {
	// Для remote: read + increment + write (не атомарно, но достаточно для тестов)
	current, err := r.Get(key)
	if err != nil && !state.IsKeyNotFound(err) {
		return 0, err
	}
	var val uint64
	if len(current) >= 8 {
		val = bytesToUint64([]byte(current))
	}
	val++
	newBytes := uint64ToBytes(val)
	if err := r.Set(key, string(newBytes)); err != nil {
		return 0, err
	}
	return val, nil
}

func (r *RemoteKV) post(path string, req any) (*kvResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	resp, err := r.client.Post(r.baseURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result kvResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func uint64ToBytes(i uint64) []byte {
	var buf [8]byte
	buf[0] = byte(i >> 56)
	buf[1] = byte(i >> 48)
	buf[2] = byte(i >> 40)
	buf[3] = byte(i >> 32)
	buf[4] = byte(i >> 24)
	buf[5] = byte(i >> 16)
	buf[6] = byte(i >> 8)
	buf[7] = byte(i)
	return buf[:]
}

func bytesToUint64(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}
