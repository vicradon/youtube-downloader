package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"github.com/vicradon/yt-downloader/models"
)

type YouTubeService struct {
	APIKey  string
	APIHost string
}

func NewYouTubeService(apiKey, apiHost string) *YouTubeService {
	return &YouTubeService{
		APIKey:  apiKey,
		APIHost: apiHost,
	}
}

func (s *YouTubeService) ExtractVideoID(url string) (string, error) {
	if strings.Contains(url, "youtu.be/") {
		parts := strings.Split(url, "youtu.be/")
		if len(parts) > 1 {
			videoID := strings.Split(parts[1], "?")[0]
			if videoID != "" {
				return videoID, nil
			}
		}
	}

	if strings.Contains(url, "youtube.com/watch") {
		parsedURL := strings.Split(url, "?")
		if len(parsedURL) > 1 {
			params := strings.Split(parsedURL[1], "&")
			for _, param := range params {
				if strings.HasPrefix(param, "v=") {
					return strings.TrimPrefix(param, "v="), nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not extract video ID from URL")
}

func (s *YouTubeService) GetDownloadURL(videoID string) (*models.RapidAPIResponse, error) {
	rapidAPIURL := fmt.Sprintf("https://%s/download_video/%s?quality=247", s.APIHost, videoID)

	req, err := http.NewRequest("GET", rapidAPIURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("x-rapidapi-key", s.APIKey)
	req.Header.Add("x-rapidapi-host", s.APIHost)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rapidResp models.RapidAPIResponse
	if err := json.Unmarshal(body, &rapidResp); err != nil {
		return nil, err
	}

	if rapidResp.File == "" {
		return nil, fmt.Errorf("no download URL returned from API")
	}

	return &rapidResp, nil
}

func (s *YouTubeService) WaitForFileReady() {
	time.Sleep(20 * time.Second)
}

func (s *YouTubeService) GetVideoTitle(videoID string) (string, error) {
	oEmbedURL := fmt.Sprintf("https://www.youtube.com/oembed?url=https://www.youtube.com/watch?v=%s&format=json", videoID)

	req, err := http.NewRequest("GET", oEmbedURL, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oembed API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var oembedResp struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(body, &oembedResp); err != nil {
		return "", err
	}

	if oembedResp.Title == "" {
		return "", fmt.Errorf("no title found in oembed response")
	}

	return oembedResp.Title, nil
}
