package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

// MemoryStore is an in-memory key-value store for blocks and other data.
type MemoryStore struct {
	lock sync.RWMutex
	data map[string][]byte
}

// Storage is a generic interface for a key-value store.
type Storage interface {
	Put(key, value []byte) error
	Get(key []byte) ([]byte, error)
	Delete(key []byte) error
	Close() error
	List() (map[string][]byte, error)
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

// Delete removes a key-value pair from the store.
func (s *MemoryStore) Delete(key []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.data, string(key))
	return nil
}

// Close is a no-op for the in-memory store.
func (s *MemoryStore) Close() error {
	return nil
}

// List returns all key-value pairs from the memory store.
func (s *MemoryStore) List() (map[string][]byte, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	// Return a copy to prevent external modifications
	dataCopy := make(map[string][]byte)
	for k, v := range s.data {
		dataCopy[k] = v
	}
	return dataCopy, nil
}

// BoltStore provides a simple key-value store using BoltDB.
type BoltStore struct {
	db         *bbolt.DB
	path       string
	bucketName string
}

// NewBoltStore creates or opens a BoltDB database at the given path and bucket.
func NewBoltStore(path, bucketName string) (*BoltStore, error) {
	// Check if file exists and log for debugging
	if _, err := os.Stat(path); err == nil {
		log.Printf("Database file %s already exists", path)
	} else if os.IsNotExist(err) {
		log.Printf("Creating new database file at %s", path)
	} else {
		log.Printf("Warning: Could not stat database file %s: %v", path, err)
	}

	// Create new database connection
	db, err := bbolt.Open(path, 0600, &bbolt.Options{
		Timeout:      1 * time.Second, // Reduced timeout
		NoGrowSync:   false,
		FreelistType: bbolt.FreelistArrayType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %s: %w", path, err)
	}

	log.Printf("Successfully opened database at %s", path)

	// Ensure the requested bucket exists
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
		}
		log.Printf("Successfully created/verified bucket %s", bucketName)
		return nil
	})
	if err != nil {
		// If bucket creation fails, close the database before returning the error
		db.Close()
		return nil, fmt.Errorf("failed to initialize bucket %s: %w", bucketName, err)
	}
	return &BoltStore{db: db, path: path, bucketName: bucketName}, nil
}

// Close closes the database connection.
func (s *BoltStore) Close() error {
	log.Printf("Closing database connection for %s", s.path)
	if s.db != nil {
		return s.db.Close()
	}
	return nil
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
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return value, nil
}

// Delete removes a key-value pair from the store's bucket.
func (s *BoltStore) Delete(key []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(s.bucketName))
		if b == nil {
			return fmt.Errorf("bucket %s not found", s.bucketName)
		}
		return b.Delete(key)
	})
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
			// Create copies of the key and value to ensure they are safe to use after the transaction.
			keyCopy := make([]byte, len(k))
			copy(keyCopy, k)
			valueCopy := make([]byte, len(v))
			copy(valueCopy, v)
			values[string(keyCopy)] = valueCopy
			return nil
		})
	})
	return values, err
}
