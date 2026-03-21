package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Handle     string `yaml:"handle"`
	AccessJWT  string `yaml:"access_jwt"`
	RefreshJWT string `yaml:"refresh_jwt"`
	DID        string `yaml:"did"`
	Theme      string `yaml:"theme"`
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "bsky", "config.yaml"), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

type BookmarkedPost struct {
	URI         string `json:"uri"`
	CID         string `json:"cid"`
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
	Text        string `json:"text"`
	LikeCount   int    `json:"like_count"`
	RepostCount int    `json:"repost_count"`
	ReplyCount  int    `json:"reply_count"`
}

func bookmarksPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "bsky", "bookmarks.json"), nil
}

func LoadBookmarks() ([]BookmarkedPost, error) {
	path, err := bookmarksPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []BookmarkedPost{}, nil
		}
		return nil, err
	}
	var posts []BookmarkedPost
	if err := json.Unmarshal(data, &posts); err != nil {
		return nil, err
	}
	return posts, nil
}

func SaveBookmarks(posts []BookmarkedPost) error {
	path, err := bookmarksPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(posts)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
