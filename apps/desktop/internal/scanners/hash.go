package scanners

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
)

// fileMD5 returns the hex MD5 digest of a file's content, streamed (never
// loads the file into memory). limit > 0 hashes only the first limit bytes;
// files shorter than limit simply hash their full content.
func fileMD5(path string, limit int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	var r io.Reader = f
	if limit > 0 {
		r = io.LimitReader(f, limit)
	}
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
