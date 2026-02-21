package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getAssetPath(mediaType string) string {
	buff := make([]byte, 32)
	_, err := rand.Reader.Read(buff)
	if err != nil {
		panic("failed to generate random bytes")
	}
	id := base64.RawURLEncoding.EncodeToString(buff)

	ext := mediaTypeToExt(mediaType)
	return fmt.Sprintf("%s%s", id, ext)
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func (cfg apiConfig) getAssetURL(assetPath string) string {
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, assetPath)
}

func (cfg apiConfig) getS3objectURL(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)
}

func mediaTypeToExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmdName := "ffprobe"
	args := []string{"-v", "error", "-print_format", "json", "-show_streams", filePath}

	cmd := exec.Command(cmdName, args...)

	var buff bytes.Buffer
	cmd.Stdout = &buff

	err := cmd.Run()

	if err != nil {
		return "", err
	}

	var cmdOutput struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}

	err = json.Unmarshal(buff.Bytes(), &cmdOutput)

	if err != nil {
		return "", err
	}

	if len(cmdOutput.Streams) == 0 {
		return "", errors.New("no video streams found")
	}

	width, height := cmdOutput.Streams[0].Width, cmdOutput.Streams[0].Height
	aspect := float64(width) / float64(height)

	switch {
	case aspect > 1.2:
		return "landscape", nil
	case aspect < 0.8:
		return "portrait", nil
	default:
		return "other", nil
	}
}
