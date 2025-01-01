package mcnutils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/docker/machine/libmachine/log"
)

type ProgressWriter struct {
	Total      int64
	Downloaded int64
	mu         sync.Mutex
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.mu.Lock()
	pw.Downloaded += int64(n)
	pw.PrintProgress()
	pw.mu.Unlock()
	return n, nil
}

func (pw *ProgressWriter) PrintProgress() {
	fmt.Printf("\rDownloading... %d/%d bytes complete", pw.Downloaded, pw.Total)
}

func DownloadPart(url string, start, end int64, partFileName string, pw *ProgressWriter, wg *sync.WaitGroup) {
	defer wg.Done()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Infof("Error creating request: %v", err)
		return
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Infof("Error downloading part: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		log.Infof("Error: expected status 206 Partial Content, got %d", resp.StatusCode)
		return
	}

	partFile, err := os.Create(partFileName)
	if err != nil {
		log.Infof("Error creating part file: %v", err)
		return
	}
	defer partFile.Close()

	buf := make([]byte, 32*1024) // 32 KB buffer
	_, err = io.CopyBuffer(io.MultiWriter(partFile, pw), resp.Body, buf)
	if err != nil {
		log.Infof("Error saving part: %v", err)
		return
	}
}

func DownloadVHDX(url string, filePath string, numParts int) error {
	resp, err := http.Head(url)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	totalSize := resp.ContentLength
	partSize := totalSize / int64(numParts)

	pw := &ProgressWriter{Total: totalSize}
	var wg sync.WaitGroup

	partFiles := make([]string, numParts)
	for i := 0; i < numParts; i++ {
		start := int64(i) * partSize
		end := start + partSize - 1
		if i == numParts-1 {
			end = totalSize - 1
		}

		partFileName := fmt.Sprintf("part-%d.tmp", i)
		partFiles[i] = partFileName

		wg.Add(1)
		go DownloadPart(url, start, end, partFileName, pw, &wg)
	}

	wg.Wait()

	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	for _, partFileName := range partFiles {
		partFile, err := os.Open(partFileName)
		if err != nil {
			return fmt.Errorf("failed to open part file: %w", err)
		}

		_, err = io.Copy(out, partFile)
		partFile.Close()
		if err != nil {
			return fmt.Errorf("failed to merge part file: %w", err)
		}

		os.Remove(partFileName)
	}

	log.Infof("\nDownload complete")
	return nil
}
