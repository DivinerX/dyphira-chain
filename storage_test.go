package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMemoryStore(t *testing.T) {
	store := NewMemoryStore()

	key := []byte("test_key")
	value := []byte("test_value")

	// Test Put and Has
	err := store.Put(key, value)
	assert.Nil(t, err)

	// Test Get
	retrievedValue, err := store.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value, retrievedValue)

	// Test Get non-existent key
	_, err = store.Get([]byte("non_existent_key"))
	assert.NotNil(t, err)
}

func TestBoltStore(t *testing.T) {
	tmpfile := filepath.Join(os.TempDir(), fmt.Sprintf("boltstore_%d.db", time.Now().UnixNano()))
	defer os.Remove(tmpfile)

	store, err := NewBoltStore(tmpfile, "bolt")
	assert.Nil(t, err)

	key := []byte("bolt_key")
	value := []byte("bolt_value")
	// Use the new API: Put/Get without bucket name
	err = store.Put(key, value)
	assert.Nil(t, err)

	got, err := store.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value, got)

	all, err := store.List()
	assert.Nil(t, err)
	assert.Equal(t, value, all[string(key)])

	assert.Nil(t, store.Close())
}
