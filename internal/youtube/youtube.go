package youtube

import (
	"dailysongbot/internal/config"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

var ErrEndPlaylist = errors.New("end of playlist")

func TestPlaylist(playlistID string) (bool, error) {
	// make main part of url
	u := url.URL{
		Scheme: "https",
		Host:   "www.googleapis.com",
		Path:   "/youtube/v3/playlists",
	}

	// make ?part=id&id=XXXXXXXXXXXXXXXXX&key=XXXXXXXXXXXXXXXXXXXXX
	q := u.Query()
	q.Set("part", "id")
	q.Set("id", playlistID)
	q.Set("key", config.EnvConfig.YoutubeApiKey)
	u.RawQuery = q.Encode()

	// cook request
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return false, fmt.Errorf("new request: %w", err)
	}

	// send it
	resp, err := http.DefaultClient.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return false, fmt.Errorf("do http request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("http status: %s", resp.Status)
	}

	// respons structures
	type pageInfo = struct {
		TotalResults int `json:"totalResults"`
	}
	playlistListResponse := struct {
		PageInfo pageInfo `json:"pageInfo"`
	}{}

	// read response
	err = json.NewDecoder(resp.Body).Decode(&playlistListResponse)
	if err != nil {
		return false, fmt.Errorf("json decode: %w", err)
	}

	if playlistListResponse.PageInfo.TotalResults != 1 {
		return false, errors.New("playlist not found")
	} else {
		return true, nil
	}
}

func GetSong(playlistId string, playlistItemNum int64) (string, error) {
	if playlistItemNum <= 0 {
		return "", fmt.Errorf("playlist item number: position cannot be negative: %d", playlistItemNum)
	}

	// respons structures
	type songInfo struct {
		Snippet struct {
			ResourceId struct {
				VideoId string `json:"videoId"`
			} `json:"resourceId"`
		} `json:"snippet"`
	}

	const maxResults int64 = 50
	pageToken := ""
	// calculate page number from playlistItemNum
	targetPage := (playlistItemNum + maxResults - 1) / maxResults
	position := (playlistItemNum - 1) % maxResults

	// make main part of url
	u := url.URL{
		Scheme: "https",
		Host:   "www.googleapis.com",
		Path:   "/youtube/v3/playlistItems",
	}

	q := u.Query()
	q.Set("part", "snippet")
	q.Set("playlistId", playlistId)
	q.Set("maxResults", fmt.Sprint(maxResults))
	q.Set("key", config.EnvConfig.YoutubeApiKey)

	var playlistListsResponse = struct {
		NextPageToken string `json:"nextPageToken"`
		PageInfo      struct {
			TotalResults int `json:"totalResults"`
		} `json:"pageInfo"`
		Items []songInfo `json:"items"`
	}{}
	var songInfoBuff songInfo

	for i := range targetPage {
		q.Set("pageToken", pageToken)
		u.RawQuery = q.Encode()
		// cook request
		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			return "", fmt.Errorf("http new request: %w", err)
		}

		err = func() error {
			// send it, in anonymous func just for defer body close
			resp, err := http.DefaultClient.Do(req)
			if resp != nil {
				defer resp.Body.Close()
			}
			if err != nil {
				return fmt.Errorf("http do request: %w", err)
			}

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("http status: %s", resp.Status)
			}

			err = json.NewDecoder(resp.Body).Decode(&playlistListsResponse)
			if err != nil {
				return fmt.Errorf("json new decoder: %w", err)
			}
			return nil
		}()
		if err != nil {
			// no need for fmt errorf here
			return "", err
		}

		if playlistListsResponse.PageInfo.TotalResults == 0 {
			return "", fmt.Errorf("api total results: zero results in api playlist list response")
		}

		if i < targetPage-1 && playlistListsResponse.NextPageToken == "" {
			return "", fmt.Errorf("next page token: %w", ErrEndPlaylist)
		}
	}

	if int(position) >= len(playlistListsResponse.Items) {
		return "", fmt.Errorf("playlist api response: %w", ErrEndPlaylist)
	}
	songInfoBuff = playlistListsResponse.Items[position]

	return songInfoBuff.Snippet.ResourceId.VideoId, nil
}
