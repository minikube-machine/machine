package assets

import (
	"io"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"k8s.io/klog"
)

// BaseAsset is the base asset class
type BaseAsset struct {
	SourcePath  string
	TargetDir   string
	TargetName  string
	Permissions string
	Source      string
}

// GetSourcePath returns asset name
func (b *BaseAsset) GetSourcePath() string {
	return b.SourcePath
}

// GetTargetPath returns target path
func (b *BaseAsset) GetTargetPath() string {
	return path.Join(b.GetTargetDir(), b.GetTargetName())
}

// GetTargetDir returns target dir
func (b *BaseAsset) GetTargetDir() string {
	return b.TargetDir
}

// GetTargetName returns target name
func (b *BaseAsset) GetTargetName() string {
	return b.TargetName
}

// GetPermissions returns permissions
func (b *BaseAsset) GetPermissions() string {
	return b.Permissions
}

// GetModTime returns mod time
func (b *BaseAsset) GetModTime() (time.Time, error) {
	return time.Time{}, nil
}

// FileAsset is an asset using a file
type FileAsset struct {
	BaseAsset
	reader io.ReadSeeker
	writer io.Writer
	file   *os.File // Optional pointer to close file through FileAsset.Close()
}

// NewFileAsset creates a new FileAsset
func NewFileAsset(src, targetDir, targetName, permissions string) (*FileAsset, error) {
	klog.V(4).Infof("NewFileAsset: %s -> %s", src, path.Join(targetDir, targetName))

	info, err := os.Stat(src)
	if err != nil {
		return nil, errors.Wrapf(err, "stat")
	}

	if info.Size() == 0 {
		klog.Warningf("NewFileAsset: %s is an empty file!", src)
	}

	f, err := os.Open(src)
	if err != nil {
		return nil, errors.Wrap(err, "open")
	}

	return &FileAsset{
		BaseAsset: BaseAsset{
			SourcePath:  src,
			TargetDir:   targetDir,
			TargetName:  targetName,
			Permissions: permissions,
		},
		reader: io.NewSectionReader(f, 0, info.Size()),
		file:   f,
	}, nil
}

// GetLength returns the file length, or 0 (on error)
func (f *FileAsset) GetLength() (flen int) {
	fi, err := os.Stat(f.SourcePath)
	if err != nil {
		klog.Errorf("stat(%q) failed: %v", f.SourcePath, err)
		return 0
	}
	return int(fi.Size())
}

// SetLength sets the file length
func (f *FileAsset) SetLength(flen int) {
	err := os.Truncate(f.SourcePath, int64(flen))
	if err != nil {
		klog.Errorf("truncate(%q) failed: %v", f.SourcePath, err)
	}
}

// GetModTime returns modification time of the file
func (f *FileAsset) GetModTime() (time.Time, error) {
	fi, err := os.Stat(f.SourcePath)
	if err != nil {
		klog.Errorf("stat(%q) failed: %v", f.SourcePath, err)
		return time.Time{}, err
	}
	return fi.ModTime(), nil
}

// Read reads the asset
func (f *FileAsset) Read(p []byte) (int, error) {
	if f.reader == nil {
		return 0, errors.New("Error attempting FileAsset.Read, FileAsset.reader uninitialized")
	}
	return f.reader.Read(p)
}

// Write writes the asset
func (f *FileAsset) Write(p []byte) (int, error) {
	if f.writer == nil {
		f.file.Close()
		perms, err := strconv.ParseUint(f.Permissions, 8, 32)
		if err != nil || perms > 07777 {
			return 0, err
		}
		f.file, err = os.OpenFile(f.SourcePath, os.O_RDWR|os.O_CREATE, os.FileMode(perms))
		if err != nil {
			return 0, err
		}
		f.writer = io.Writer(f.file)
	}
	return f.writer.Write(p)
}

// Seek resets the reader to offset
func (f *FileAsset) Seek(offset int64, whence int) (int64, error) {
	return f.reader.Seek(offset, whence)
}

// Close closes the opend file.
func (f *FileAsset) Close() error {
	if f.file == nil {
		return nil
	}
	return f.file.Close()
}
