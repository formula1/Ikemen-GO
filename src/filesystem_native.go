//go:build !js

package main

import (
	"os"
	"path/filepath"
)

var fs = NewFileSystem()

type NativeFileSystem struct{}

func NewFileSystem() FileSystem {
	return &NativeFileSystem{}
}

func (fs *NativeFileSystem) Open(name string) (File, error) {
	return os.Open(name)
}

func (fs *NativeFileSystem) Stat(name string) (FileInfo, error) {
	return os.Stat(name)
}

func (fs *NativeFileSystem) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (fs *NativeFileSystem) Create(name string) (File, error) {
	return os.Create(name)
}

func (fs *NativeFileSystem) Remove(name string) error {
	return os.Remove(name)
}

func (fs *NativeFileSystem) Mkdir(path string, perm os.FileMode) error {
	return os.Mkdir(path, perm)
}

func (fs *NativeFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (fs *NativeFileSystem) ReadDir(path string) ([]FileInfo, error) {
	dirEntrys, err := os.ReadDir(path)

	if err != nil {
		return nil, err
	}

	fileInfos := make([]FileInfo, len(dirEntrys))
	for i, entry := range dirEntrys {
		filepath := filepath.Join(path, entry.Name())
		info, err := fs.Stat(filepath)
		if err != nil {
			return nil, err
		}
		fileInfos[i] = info
	}

	return fileInfos, nil
}

func (fs *NativeFileSystem) Walk(root string, fn func(path string, info FileInfo, err error) error) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		return fn(path, info, err)
	})
}

func (fs *NativeFileSystem) Abs(path string) (string, error) {
	return filepath.Abs(path)
}

func (fs *NativeFileSystem) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (fs *NativeFileSystem) Dir(path string) string {
	return filepath.Dir(path)
}

func (fs *NativeFileSystem) Base(path string) string {
	return filepath.Base(path)
}

func (fs *NativeFileSystem) SearchFile(filename string, dirs []string) string {
	return SearchFile(filename, dirs)
}

func (fs *NativeFileSystem) FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}
