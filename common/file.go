package common

import (
	"io"
	"os"
	"path/filepath"
)

// GetFileSize get the length in bytes of file of the specified path.
func (g *GuFile) GetFileSize(path string) int64 {
	fi, err := os.Stat(path)
	if nil != err {
		return -1
	}
	return fi.Size()
}

// IsExist determines whether the file specified by the given path is exists.
func (g *GuFile) IsExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

// IsBinary determines whether the specified content is a binary file content.
func (g *GuFile) IsBinary(content string) bool {
	for _, b := range content {
		if 0 == b {
			return true
		}
	}
	return false
}

// IsDir determines whether the specified path is a directory.
func (g *GuFile) IsDir(path string) bool {
	fio, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false
	}

	if nil != err {
		Log.Warn("determines whether [%s] is a directory failed: [%v]", path, err)
		return false
	}
	return fio.IsDir()
}

func (g *GuFile) ReadFile(filePath string) (data []byte, err error) {

	data, err = os.ReadFile(filePath)
	if err != nil {
		Log.Error("failed to read file: [%v]", err)
	}
	return
}

func (g *GuFile) WriteFileSafer(writePath string, data []byte, perm os.FileMode) error {
	return g.writeFileSafer(writePath, func(f *os.File) error {
		_, err := f.Write(data)
		return err
	}, perm)
}

func (g *GuFile) WriteFileSaferByReader(writePath string, reader io.Reader, perm os.FileMode) error {
	return g.writeFileSafer(writePath, func(f *os.File) error {
		_, err := io.Copy(f, reader)
		return err
	}, perm)
}

func (g *GuFile) writeFileSafer(writePath string, writeFunc func(*os.File) error, perm os.FileMode) error {
	dir, name := filepath.Split(writePath)
	tmp, err := os.CreateTemp(dir, name+"-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	err = writeFunc(tmp)
	if err != nil {
		return err
	}

	err = tmp.Sync()
	if err != nil {
		return err
	}

	err = tmp.Close()
	if err != nil {
		return err
	}

	err = os.Chmod(tmp.Name(), perm)
	if err != nil {
		return err
	}

	err = os.Rename(tmp.Name(), writePath)
	if err == nil {
		return nil
	}
	return err
}
