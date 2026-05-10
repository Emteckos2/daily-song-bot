package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// struct for de/encoding json settings
type UserConfigDataStruct struct {
	PostHours        int
	PostMinutes      int
	PlaylistNextSong int64
	PlaylistID       string
	ChannelID        string
	NextRunTime      time.Time
}

// struct for data from system environment values
type envConfigStruct struct {
	DiscordBotToken string
	YoutubeApiKey   string
	GuildID         string
}

var (
	userConfig atomic.Value
	EnvConfig  envConfigStruct
)

func getPath() (string, error) {
	// read current path of program in config folder
	fp := filepath.Dir(os.Args[0])
	fp += "/config.json"
	dir, err := filepath.Abs(fp)
	if err != nil {
		return "", fmt.Errorf("filepath: %w", err)
	}
	return dir, nil
}

func ReadConfig() error {
	var (
		err        error
		configPath string
	)

	// load os env
	EnvConfig = envConfigStruct{
		DiscordBotToken: os.Getenv("DiscordBotToken"),
		YoutubeApiKey:   os.Getenv("YoutubeApiKey"),
		GuildID:         os.Getenv("GuildID"),
	}

	// if there is not present value, drop error
	if EnvConfig.DiscordBotToken == "" {
		return fmt.Errorf("environment variable: DiscordBotToken is not set")
	}
	if EnvConfig.YoutubeApiKey == "" {
		return fmt.Errorf("environment variable: YoutubeApiKey is not set")
	}
	if EnvConfig.GuildID == "" {
		return fmt.Errorf("environment variable: GuildID is not set")
	}

	// config.json is intended next program binary
	configPath, err = getPath()
	if err != nil {
		return fmt.Errorf("get path: %w", err)
	}
	// reading config.json file
	c, err := os.ReadFile(configPath)
	if err != nil {
		// config will be created if not existing
		if errors.Is(err, os.ErrNotExist) {
			e := createConfig(configPath)
			if e != nil {
				return fmt.Errorf("create config: %w", e)
			}
		} else {
			return fmt.Errorf("read config file: %w", err)
		}
	}
	if !json.Valid(c) {
		return fmt.Errorf("invalid config json")
	}
	// parse data from config.json to user config struck
	var data UserConfigDataStruct
	err = json.Unmarshal(c, &data)
	if err != nil {
		return fmt.Errorf("decode config: %w", err)
	}

	err = testConfig(&data)
	if err != nil {
		return fmt.Errorf("check config settings: %w", err)
	}

	userConfig.Store(data)

	return nil
}

func createConfig(configPath string) error {
	var input string
	for range 3 {
		fmt.Print("Wanna create config file? (Y/n) ")
		_, err := fmt.Scanln(&input)
		if err != nil {
			return fmt.Errorf("input scanln: %w", err)
		}

		switch strings.ToLower(input) {
		case "y", "":
			fmt.Println("Creating config")

			data := UserConfigDataStruct{
				PostHours:        12,
				PostMinutes:      00,
				PlaylistNextSong: 1,
				PlaylistID:       "",
				ChannelID:        "",
				NextRunTime:      time.Date(0, time.January, 1, 0, 0, 0, 0, time.UTC),
			}

			err := Save(&data)
			if err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			fmt.Println("Please edit:", configPath)
			fmt.Println("Closing program")
			os.Exit(0)
		case "n":
			fmt.Println("Cancelled by user")
			os.Exit(0)
		default:
		}
	}
	os.Exit(0)
	return nil
}

func Get() (UserConfigDataStruct, error) {
	var (
		data UserConfigDataStruct
		ok   bool
	)
	if data, ok = userConfig.Load().(UserConfigDataStruct); !ok {
		return data, fmt.Errorf("user config load: nil data")
	}
	return data, nil
}

// write data, doesnt check if are correct or not
func Save(data *UserConfigDataStruct) error {
	var err error

	err = testConfig(data)
	if err != nil {
		return fmt.Errorf("test config: %w", err)
	}

	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	configPath, err := getPath()
	if err != nil {
		return fmt.Errorf("get path: %w", err)
	}

	err = os.WriteFile(configPath, b, 0644)
	if err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	// save also safe data to variable to keep data sync
	userConfig.Store(*data)

	return nil
}

// tests data if are seems to be ok
func testConfig(data *UserConfigDataStruct) error {
	switch {
	case data.ChannelID == "":
		return fmt.Errorf("channel id: not set")
	case data.PlaylistID == "":
		return fmt.Errorf("playlist id: not set")
	case data.NextRunTime.IsZero():
		return fmt.Errorf("last run time: not set")
	case data.PlaylistNextSong < 1:
		return fmt.Errorf("playlist next song: negative position: %d", data.PlaylistNextSong)
	case data.PostHours < 0 || data.PostHours > 24:
		return fmt.Errorf("post hours: out of range: %d h", data.PostHours)
	case data.PostMinutes < 0 || data.PostMinutes > 59:
		return fmt.Errorf("post hours: out of range: %d m", data.PostMinutes)
	default:
		return nil
	}
}
