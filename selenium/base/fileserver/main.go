package main

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	dir, err := downloadsDir()
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(http.ListenAndServe(":8080", mux(dir)))
}

func downloadsDir() (string, error) {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/home/selenium"
	}
	dir := filepath.Join(homeDir, "Downloads")
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create downloads dir: %v", err)
	}
	return dir, nil
}

const (
	jsonParam = "json"
	hashSum   = "hash"
)

func mux(dir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteFileIfExists(w, r, dir)
			return
		}
		if _, ok := r.URL.Query()[jsonParam]; ok {
			hashSumQuery, ok := r.URL.Query()[hashSum]
			if ok {
				listFilesAsJson(w, dir, hashSumQuery[0])
				return
			}
			listFilesAsJson(w, dir, "")
			return
		}
		http.FileServer(http.Dir(dir)).ServeHTTP(w, r)
	})
	return mux
}

type FileInfo struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	LastModified int64  `json:"lastModified"`
	HashSum      string `json:"hashSum,omitempty"`
}

func getHash(file string, algo string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()

	h := NewHash(algo)
	if h == nil {
		return "", nil
	}

	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to copy: %v", err)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func NewHash(algo string) hash.Hash {
	switch strings.ToLower(algo) {

	case "md5":
		return md5.New()
	case "sha1":
		return sha1.New()

	case "sha256":
		return sha256.New()

	default:
		return nil
	}
}

func listFilesAsJson(w http.ResponseWriter, dir string, algo string) {

	entries, err := os.ReadDir(dir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		hashFile, err := getHash(dir+"/"+entry.Name(), algo)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		files = append(files, FileInfo{
			Name:         info.Name(),
			Size:         info.Size(),
			LastModified: info.ModTime().Unix(),
			HashSum:      hashFile,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].LastModified > files[j].LastModified
	})

	w.Header().Add("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(files)
}

func deleteFileIfExists(w http.ResponseWriter, r *http.Request, dir string) {
	fileName := strings.TrimPrefix(r.URL.Path, "/")
	filePath := filepath.Join(dir, fileName)
	_, err := os.Stat(filePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unknown file %s", fileName), http.StatusNotFound)
		return
	}
	err = os.Remove(filePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete file %s: %v", fileName, err), http.StatusInternalServerError)
		return
	}
}
