// Package provides filesystem abstraction layer for both native and web platforms
package main

import (
	"io"
	"os"
)

// FileSystem provides an interface for platform-specific file operations
type FileSystem interface {
	// File operations
	Open(name string) (File, error)
	Stat(name string) (FileInfo, error)
	ReadFile(name string) ([]byte, error)
	Create(name string) (File, error)
	Remove(name string) error

	// Directory operations
	Mkdir(path string, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	ReadDir(path string) ([]FileInfo, error)
	Walk(root string, fn func(path string, info FileInfo, err error) error) error

	// Path operations
	Abs(path string) (string, error)
	Join(elem ...string) string
	Dir(path string) string
	Base(path string) string

	// File search
	SearchFile(filename string, dirs []string) string
	FileExists(filename string) bool
}

// File interface abstracts basic file operations
type File interface {
	io.ReadWriter
	io.Closer
	Name() string
	Seek(offset int64, whence int) (int64, error)
}

type FileInfo interface {
	Name() string
	Size() int64
	IsDir() bool
}
