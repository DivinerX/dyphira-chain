package main

import (
	"fmt"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

// MemoryStore is an in-memory key-value store for blocks and other data.
type MemoryStore struct {
	lock sync.RWMutex
	data map[string][]byte
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data: make(map[string][]byte),
	}
}

// Put adds a key-value pair to the store.
func (s *MemoryStore) Put(key, value []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.data[string(key)] = value
	return nil
}

// Get retrieves a value by its key.
func (s *MemoryStore) Get(key []byte) ([]byte, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	value, ok := s.data[string(key)]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return value, nil
}

// Add Close method to MemoryStore
func (s *MemoryStore) Close() error {
	return nil
}

// BoltStore provides a simple key-value store using BoltDB.
type BoltStore struct {
	db         *bbolt.DB
	path       string
	bucketName string
}

// NewBoltStore creates or opens a BoltDB database at the given path and bucket.
func NewBoltStore(path, bucketName string) (*BoltStore, error) {
	db, err := bbolt.Open(path, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}
	// Ensure the requested bucket exists
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})
	if err != nil {
		return nil, err
	}
	return &BoltStore{db: db, path: path, bucketName: bucketName}, nil
}

// Close closes the database connection.
func (s *BoltStore) Close() error {
	return s.db.Close()
}

// Put saves a key-value pair in the store's bucket.
func (s *BoltStore) Put(key, value []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(s.bucketName))
		if b == nil {
			return fmt.Errorf("bucket %s not found", s.bucketName)
		}
		return b.Put(key, value)
	})
}

// Get retrieves a value from the store's bucket by its key.
func (s *BoltStore) Get(key []byte) ([]byte, error) {
	var value []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(s.bucketName))
		if b == nil {
			return fmt.Errorf("bucket %s not found", s.bucketName)
		}
		value = b.Get(key)
		return nil
	})
	return value, err
}

// List retrieves all key-value pairs from the store's bucket.
func (s *BoltStore) List() (map[string][]byte, error) {
	values := make(map[string][]byte)
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(s.bucketName))
		if b == nil {
			return fmt.Errorf("bucket %s not found", s.bucketName)
		}
		return b.ForEach(func(k, v []byte) error {
			values[string(k)] = v
			return nil
		})
	})
	return values, err
}
