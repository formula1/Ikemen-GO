//go:build js

package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall/js"
)

var fs = NewFileSystem()

type WebFileSystem struct{}

func NewFileSystem() FileSystem {
	return &WebFileSystem{}
}

// WebFile implements the File interface for browser
type WebFile struct {
	name     string
	data     []byte
	position int64
}

func (f *WebFile) Read(p []byte) (n int, err error) {
	if f.position >= int64(len(f.data)) {
		return 0, io.EOF
	}
	n = copy(p, f.data[f.position:])
	f.position += int64(n)
	return
}

func (f *WebFile) Write(p []byte) (n int, err error) {
	// Write to in-memory buffer
	f.data = append(f.data, p...)
	return len(p), nil
}

func (f *WebFile) Close() error {
	// Sync with JS storage if needed
	if len(f.data) > 0 {
		getFileSystem().Call("writeFile", f.name, f.data)
	}
	return nil
}

func (f *WebFile) Name() string {
	return f.name
}

func (f *WebFile) Size() (int64, error) {
	return int64(len(f.data)), nil
}

func (f *WebFile) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = f.position + offset
	case io.SeekEnd:
		abs = int64(len(f.data)) + offset
	default:
		return 0, errors.New("invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("negative position")
	}
	f.position = abs
	return abs, nil
}

type WebFileInfo struct {
	name string
	size int64
	dir  bool
}

func (fi *WebFileInfo) Name() string {
	return fi.name
}
func (fi *WebFileInfo) Size() int64 {
	return fi.size
}
func (fi *WebFileInfo) IsDir() bool {
	return fi.dir
}

func (fs *WebFileSystem) Open(name string) (File, error) {
	// Call JS to load file data
	dataBytes := getFileSystem().Call("readFile", name)
	if dataBytes.IsNull() {
		return nil, os.ErrNotExist
	}

	// Convert JS data to Go bytes
	data := make([]byte, dataBytes.Length())
	js.CopyBytesToGo(data, dataBytes)

	return &WebFile{
		name: name,
		data: data,
	}, nil
}

func (fs *WebFileSystem) Stat(name string) (FileInfo, error) {
	// Call JS to get file info
	info := getFileSystem().Call("stat", name)
	if info.IsNull() {
		return nil, os.ErrNotExist
	}

	return &WebFileInfo{
		name: info.Get("name").String(),
		size: int64(info.Get("size").Int()),
		dir:  info.Get("dir").Bool(),
	}, nil
}

func (fs *WebFileSystem) ReadFile(name string) ([]byte, error) {
	// Call JS to load file data
	dataBytes := getFileSystem().Call("readFile", name)
	if dataBytes.IsNull() {
		return nil, os.ErrNotExist
	}

	// Convert JS data to Go bytes
	data := make([]byte, dataBytes.Length())
	js.CopyBytesToGo(data, dataBytes)

	return data, nil
}

func (fs *WebFileSystem) Create(name string) (File, error) {
	return &WebFile{
		name: name,
		data: make([]byte, 0),
	}, nil
}

func (fs *WebFileSystem) Remove(name string) error {
	// Call JS to remove file
	getFileSystem().Call("remove", name)
	return nil
}

func (fs *WebFileSystem) Mkdir(path string, perm os.FileMode) error {
	// No-op for web (directories are virtual)
	getFileSystem().Call("mkdir", path)
	return nil
}

func (fs *WebFileSystem) MkdirAll(path string, perm os.FileMode) error {
	// No-op for web (directories are virtual)
	getFileSystem().Call("mkdirAll", path)
	return nil
}

func (fs *WebFileSystem) ReadDir(path string) ([]FileInfo, error) {
	entries := getFileSystem().Call("readDir", path)
	if entries.IsNull() {
		return nil, os.ErrNotExist
	}

	// Convert JS array to Go slice
	length := entries.Length()
	fileInfos := make([]FileInfo, length)
	for i := 0; i < length; i++ {
		name := entries.Index(i).String()
		fullPath := fs.Join(path, name)
		info, err := fs.Stat(fullPath)
		if err != nil {
			return nil, err
		}
		fileInfos[i] = info
	}

	return fileInfos, nil
}

func (fs *WebFileSystem) Walk(root string, fn func(path string, info FileInfo, err error) error) error {
	// First call fn for the root itself
	rootInfo, err := fs.Stat(root)
	if err != nil {
		return fn(root, nil, err)
	}
	if err := fn(root, rootInfo, nil); err != nil {
		return err
	}

	if !rootInfo.IsDir() {
		return nil // If root is a file, we're done
	}

	var walkDir func(path string) error
	walkDir = func(path string) error {
		entries := getFileSystem().Call("readDir", path)
		if entries.IsNull() {
			return nil
		}

		// Convert JS array to Go slice
		length := entries.Length()
		for i := 0; i < length; i++ {
			name := entries.Index(i).String()
			fullPath := fs.Join(path, name)

			info, err := fs.Stat(fullPath)
			if err != nil {
				if err := fn(fullPath, nil, err); err != nil {
					return err
				}
				continue
			}

			if err := fn(fullPath, info, nil); err != nil {
				if err == filepath.SkipDir && info.IsDir() {
					continue
				}
				return err
			}

			if info.IsDir() {
				if err := walkDir(fullPath); err != nil {
					if err == filepath.SkipDir {
						continue
					}
					return err
				}
			}
		}
		return nil
	}

	return walkDir(root)
}

// Path operations remain similar but use forward slashes
func (fs *WebFileSystem) Join(elem ...string) string {
	return filepath.ToSlash(filepath.Join(elem...))
}

func (fs *WebFileSystem) Dir(path string) string {
	return filepath.ToSlash(filepath.Dir(path))
}

func (fs *WebFileSystem) Base(path string) string {
	return filepath.Base(path)
}

func (fs *WebFileSystem) Abs(path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.ToSlash(path), nil
	}
	return "/" + filepath.ToSlash(path), nil
}

func (fs *WebFileSystem) SearchFile(filename string, dirs []string) string {
	// Call JS to search for file
	result := getFileSystem().Call("searchFile", filename, dirs)
	if result.IsNull() {
		return ""
	}
	return result.String()
}

func (fs *WebFileSystem) FileExists(filename string) bool {
	result := getFileSystem().Call("exists", filename)
	return result.Bool()
}
