package mcnutils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

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
	fmt.Printf("\r        > %s...: %d / %d bytes complete \t", defaultServerImageFilename, pw.Downloaded, pw.Total)
}

func DownloadPart(url string, start, end int64, partFileName string, pw *ProgressWriter, wg *sync.WaitGroup, retryLimit int) error {
	defer wg.Done()

	var resp *http.Response

	// Retry loop for downloading a part
	for retries := 0; retries <= retryLimit; retries++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Errorf("Error creating request: %v", err)
			return err
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			log.Errorf("Error downloading part: %v", err)
			// Retry after waiting a bit
			time.Sleep(time.Duration(2<<retries) * time.Second) // Exponential backoff
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusPartialContent {
			partFile, err := os.Create(partFileName)
			if err != nil {
				log.Errorf("Error creating part file: %v", err)
				return err
			}
			defer partFile.Close()

			buf := make([]byte, 32*1024) // 32 KB buffer
			_, err = io.CopyBuffer(io.MultiWriter(partFile, pw), resp.Body, buf)
			if err != nil {
				log.Errorf("Error saving part: %v", err)
				return err
			}
			return nil // Successful download, exit retry loop
		}

		// If server does not return expected status, retry
		log.Errorf("Error: expected status 206 Partial Content, got %d", resp.StatusCode)
		// Retry after waiting
		time.Sleep(time.Duration(2<<retries) * time.Second) // Exponential backoff
	}

	// If all retries fail, return an error
	log.Errorf("Failed to download part after %d retries", retryLimit)
	return fmt.Errorf("failed to download part after %d retries", retryLimit)
}

func DownloadVHDX(url string, filePath string, numParts int, retryLimit int) error {
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
	var downloadErrors []error

	for i := 0; i < numParts; i++ {
		start := int64(i) * partSize
		end := start + partSize - 1
		if i == numParts-1 {
			end = totalSize - 1
		}

		partFileName := fmt.Sprintf("part-%d.tmp", i)
		partFiles[i] = partFileName

		wg.Add(1)
		go func(i int) {
			err := DownloadPart(url, start, end, partFileName, pw, &wg, retryLimit)
			if err != nil {
				downloadErrors = append(downloadErrors, fmt.Errorf("failed to download part %d: %w", i, err))
			}
		}(i)
	}

	wg.Wait()

	// If there are any errors during download, return them
	if len(downloadErrors) > 0 {
		return fmt.Errorf("download failed for the following parts: %v", downloadErrors)
	}

	// Proceed with merging the parts as before
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

	log.Infof("\n\r\t> Download complete")
	return nil
}
