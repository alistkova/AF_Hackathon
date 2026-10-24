package content

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStorageRoundTrip(t *testing.T) {
	storage := NewLocalStorage(t.TempDir())
	ctx := context.Background()

	if err := storage.Put(ctx, "pdf/ab/hash.pdf", []byte("%PDF-data")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	data, err := storage.Get(ctx, "pdf/ab/hash.pdf")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(data) != "%PDF-data" {
		t.Fatalf("data = %q", data)
	}

	if err := storage.Delete(ctx, "pdf/ab/hash.pdf"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := storage.Get(ctx, "pdf/ab/hash.pdf"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after delete = %v, want ErrNotFound", err)
	}
}

func TestLocalStorageCreatesNestedDirs(t *testing.T) {
	root := t.TempDir()
	storage := NewLocalStorage(root)

	if err := storage.Put(context.Background(), "pdf/aa/bb/file.pdf", []byte("x")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "pdf", "aa", "bb", "file.pdf")); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}

func TestLocalStorageRejectsTraversal(t *testing.T) {
	storage := NewLocalStorage(t.TempDir())

	cases := []string{"../escape.pdf", "pdf/../../escape.pdf", "/absolute.pdf", "", "."}
	for _, key := range cases {
		t.Run(key, func(t *testing.T) {
			if err := storage.Put(context.Background(), key, []byte("x")); err == nil {
				t.Fatalf("Put(%q) succeeded, want error", key)
			}
		})
	}
}

func TestLocalStorageWithoutRoot(t *testing.T) {
	storage := NewLocalStorage("")
	if err := storage.Put(context.Background(), "a.pdf", []byte("x")); err == nil {
		t.Fatal("Put without root succeeded")
	}
}
