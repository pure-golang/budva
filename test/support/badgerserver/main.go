// Минимальный HTTP-сервер для BadgerDB, запускаемый в testcontainer.
package main

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dgraph-io/badger/v4"
)

var db *badger.DB

func main() {
	dir := os.Getenv("BADGER_DIR")
	if dir == "" {
		dir = "/data/badger"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	opts := badger.DefaultOptions(dir).WithLogger(nil)
	var err error
	db, err = badger.Open(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /get", handleGet)
	mux.HandleFunc("POST /set", handleSet)
	mux.HandleFunc("POST /delete", handleDelete)
	mux.HandleFunc("POST /getset", handleGetSet)
	mux.HandleFunc("POST /increment", handleIncrement)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("BadgerDB server starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

type kvRequest struct {
	Key string `json:"key"`
	Val string `json:"val,omitempty"`
}

type kvResponse struct {
	Val   string `json:"val,omitempty"`
	Error string `json:"error,omitempty"`
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	var req kvRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, err)
		return
	}

	var val string
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(req.Key))
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

	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, kvResponse{Val: val})
}

func handleSet(w http.ResponseWriter, r *http.Request) {
	var req kvRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, err)
		return
	}

	err := db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(req.Key), []byte(req.Val))
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, kvResponse{})
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	var req kvRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, err)
		return
	}

	err := db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(req.Key))
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, kvResponse{})
}

func handleGetSet(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, err)
		return
	}

	var req struct {
		Key string `json:"key"`
		Fn  string `json:"fn"` // "identity" — просто возвращает текущее значение
		Val string `json:"val,omitempty"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, err)
		return
	}

	var result string
	err = db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(req.Key))
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
		// Клиент отправляет val — результат fn(current) вычисляется на стороне клиента
		result = req.Val
		if result == "" {
			result = current
		}
		return txn.Set([]byte(req.Key), []byte(result))
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, kvResponse{Val: result})
}

func handleIncrement(w http.ResponseWriter, r *http.Request) {
	var req kvRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, err)
		return
	}

	add := func(existing, newVal []byte) []byte {
		return uint64ToBytes(bytesToUint64(existing) + bytesToUint64(newVal))
	}
	m := db.GetMergeOperator([]byte(req.Key), add, 200*time.Millisecond)
	defer m.Stop()

	if err := m.Add(uint64ToBytes(1)); err != nil {
		writeErr(w, err)
		return
	}
	val, err := m.Get()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, kvResponse{Val: string(uint64ToBytes(bytesToUint64(val)))})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(kvResponse{Error: err.Error()})
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
