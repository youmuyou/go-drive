package drive

import (
	"go-drive/common"
	"io"
	"io/ioutil"
	"os"
	fsPath "path"
	"path/filepath"
	"strings"
	"time"
)

type FsDrive struct {
	path string
}

type FsFile struct {
	drive *FsDrive
	path  string

	name  string
	size  int64
	isDir bool

	createdAt int64
	updatedAt int64
}

type fsDriveMeta struct {
}

type fsFileMeta struct {
}

func NewFsDrive(path string) (*FsDrive, error) {
	path, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	return &FsDrive{path}, nil
}

func (f *FsDrive) newFsFile(path string, file os.FileInfo) (common.IEntry, error) {
	path, e := filepath.Abs(path)
	if e != nil {
		return nil, common.NewNotFoundError("invalid path")
	}
	if !strings.HasPrefix(path, f.path) {
		panic("invalid file path")
	}
	path = path[len(f.path):]
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	modTime := file.ModTime().UnixNano() / int64(time.Millisecond)
	return &FsFile{
		drive:     f,
		path:      path,
		name:      file.Name(),
		size:      file.Size(),
		isDir:     file.IsDir(),
		createdAt: modTime,
		updatedAt: modTime,
	}, nil
}

func (f *FsDrive) getPath(path string) string {
	path = fsPath.Clean(path)
	return filepath.Join(f.path, path)
}

func (f *FsDrive) isRootPath(path string) bool {
	return fsPath.Clean(path) == f.path
}

func (f *FsDrive) Get(path string) (common.IEntry, error) {
	path = f.getPath(path)
	if f.isRootPath(path) {
		return nil, common.NewNotFoundError("not found")
	}
	stat, e := os.Stat(path)
	if os.IsNotExist(e) {
		return nil, common.NewNotFoundError("not found")
	}
	if e != nil {
		return nil, e
	}
	return f.newFsFile(path, stat)
}

func (f *FsDrive) Save(path string, reader io.Reader, progress common.OnProgress) (common.IEntry, error) {
	path = f.getPath(path)
	file, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if e != nil {
		return nil, e
	}
	defer func() { _ = file.Close() }()
	_, e = common.CopyWithProgress(file, reader, progress)
	if e != nil {
		return nil, e
	}
	stat, e := file.Stat()
	if e != nil {
		return nil, e
	}
	return f.newFsFile(path, stat)
}

func (f *FsDrive) MakeDir(path string) (common.IEntry, error) {
	path = f.getPath(path)
	if e := requireFile(path, false); e != nil {
		return nil, e
	}
	if e := os.Mkdir(path, 0755); e != nil {
		return nil, e
	}
	stat, e := os.Stat(path)
	if e != nil {
		return nil, e
	}
	return f.newFsFile(path, stat)
}

func (f *FsDrive) Copy(from common.IEntry, to string, progress common.OnProgress) (common.IEntry, error) {
	return nil, common.NewNotSupportedError()
}

func (f *FsDrive) Move(from string, to string) (common.IEntry, error) {
	fromPath := f.getPath(from)
	toPath := f.getPath(to)
	if f.isRootPath(fromPath) || f.isRootPath(toPath) {
		return nil, common.NewNotAllowedError("not allowed")
	}
	if e := requireFile(fromPath, true); e != nil {
		return nil, e
	}
	if e := requireFile(toPath, false); e != nil {
		return nil, e
	}
	if e := os.Rename(fromPath, toPath); e != nil {
		return nil, e
	}
	stat, e := os.Stat(toPath)
	if e != nil {
		return nil, e
	}
	return f.newFsFile(toPath, stat)
}

func (f *FsDrive) List(path string) ([]common.IEntry, error) {
	path = f.getPath(path)
	isDir, e := common.IsDir(path)
	if os.IsNotExist(e) {
		return nil, common.NewNotFoundError("file does not exist")
	}
	if !isDir {
		return nil, common.NewNotAllowedError("cannot list on file")
	}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	entries := make([]common.IEntry, len(files))
	for i, file := range files {
		entry, e := f.newFsFile(fsPath.Join(path, file.Name()), file)
		if e != nil {
			return nil, e
		}
		entries[i] = entry
	}
	return entries, nil
}

func (f *FsDrive) Delete(path string) error {
	path = f.getPath(path)
	if f.isRootPath(path) {
		return common.NewNotAllowedError("root cannot be deleted")
	}
	if e := requireFile(path, true); e != nil {
		return e
	}
	return os.RemoveAll(path)
}

func requireFile(path string, requireExists bool) error {
	exists, e := common.FileExists(path)
	if e != nil {
		return e
	}
	if requireExists && !exists {
		return common.NewNotFoundError("file does not exist")
	}
	if !requireExists && exists {
		return common.NewNotAllowedError("file exists")
	}
	return nil
}

func (f *FsDrive) Meta() common.IDriveMeta {
	return &fsDriveMeta{}
}

func (f *FsFile) Name() string {
	return f.name
}

func (f *FsFile) Type() common.EntryType {
	if f.isDir {
		return common.TypeDir
	}
	return common.TypeFile
}

func (f *FsFile) Size() int64 {
	if f.Type().IsDir() {
		return -1
	}
	return f.size
}

func (f *FsFile) Meta() common.IEntryMeta {
	return &fsFileMeta{}
}

func (f *FsFile) CreatedAt() int64 {
	return f.createdAt
}

func (f *FsFile) UpdatedAt() int64 {
	return f.updatedAt
}

func (f *FsFile) GetReader() (io.ReadCloser, error) {
	if !f.Type().IsFile() {
		return nil, common.NewNotAllowedError("cannot read non-file")
	}
	path := f.drive.getPath(f.path)
	exists, e := common.FileExists(path)
	if e != nil {
		return nil, e
	}
	if !exists {
		return nil, common.NewNotFoundError("file does not exist")
	}
	return os.Open(path)
}

func (f *fsDriveMeta) CanWrite() bool {
	return true
}

func (f *fsDriveMeta) Props() map[string]interface{} {
	return make(map[string]interface{})
}

func (f *fsDriveMeta) DirectlyUpload() bool {
	return true
}

func (f *fsFileMeta) CanRead() bool {
	return true
}

func (f *fsFileMeta) CanWrite() bool {
	return true
}

func (f *fsFileMeta) Props() map[string]interface{} {
	return make(map[string]interface{})
}