package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

// Global database connection pool to prevent multiple connections to the same file
var (
	dbConnections = make(map[string]*bbolt.DB)
	dbMutex       sync.RWMutex
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

// getOrCreateDBConnection returns an existing database connection or creates a new one
func getOrCreateDBConnection(path string) (*bbolt.DB, error) {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	// Check if we already have a connection to this database
	if db, exists := dbConnections[path]; exists {
		log.Printf("Reusing existing database connection for %s", path)
		return db, nil
	}

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
		Timeout:      10 * time.Second,
		NoGrowSync:   false,
		FreelistType: bbolt.FreelistArrayType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %s: %w", path, err)
	}

	log.Printf("Successfully opened database at %s", path)

	// Store the connection in our pool
	dbConnections[path] = db

	return db, nil
}

// NewBoltStore creates or opens a BoltDB database at the given path and bucket.
func NewBoltStore(path, bucketName string) (*BoltStore, error) {
	db, err := getOrCreateDBConnection(path)
	if err != nil {
		return nil, err
	}

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
		return nil, fmt.Errorf("failed to initialize bucket %s: %w", bucketName, err)
	}
	return &BoltStore{db: db, path: path, bucketName: bucketName}, nil
}

// Close closes the database connection.
func (s *BoltStore) Close() error {
	// Don't actually close the database here since it might be shared
	// The database will be closed when the application shuts down
	return nil
}

// CloseAllDBConnections closes all database connections (call this on application shutdown)
func CloseAllDBConnections() {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	for path, db := range dbConnections {
		log.Printf("Closing database connection for %s", path)
		if err := db.Close(); err != nil {
			log.Printf("Error closing database %s: %v", path, err)
		}
	}
	dbConnections = make(map[string]*bbolt.DB)
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
