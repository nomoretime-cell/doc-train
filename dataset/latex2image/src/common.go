package src

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func GetArXivYearIndex(filename string) string {
	// in: arXiv_src_2003_026.tar
	// out: 2003_026
	parts := strings.Split(filename, "_")
	if len(parts) >= 3 {
		result := parts[len(parts)-2] + "_" + strings.TrimSuffix(parts[len(parts)-1], ".tar")
		return result
	} else {
		return ""
	}
}

func FolderExists(name string) bool {
	_, err := os.Stat(name)
	return !os.IsNotExist(err)
}

func ExtractTar(tarFile, destDir string) error {
	file, err := os.Open(tarFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var tr *tar.Reader
	var gzr *gzip.Reader

	if strings.HasSuffix(tarFile, ".gz") || strings.HasSuffix(tarFile, ".tgz") {
		gzr, err = gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer gzr.Close()

		tr = tar.NewReader(gzr)
	} else {
		tr = tar.NewReader(file)
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		parts := strings.Split(header.Name, "/")
		var target string
		if len(parts) == 2 {
			if parts[1] == "" && strings.Contains(destDir, parts[0]) {
				continue
			}
			target = parts[1]
		} else {
			target = header.Name
		}

		if !strings.HasPrefix(target, destDir) {
			target = filepath.Join(destDir, target)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	return nil
}

func loadFile(filename string) (string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
