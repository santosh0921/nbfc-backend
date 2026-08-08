package surepass

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Post(endpoint string, body interface{}) ([]byte, error) {

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		c.BaseURL+endpoint,
		bytes.NewBuffer(jsonBody),
	)

	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}

	println("Surepass HTTP Status:", resp.Status)

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
        if err != nil {
	        return nil, err
        }

    println("Surepass Response:")
    println(string(bodyBytes))

    return bodyBytes, nil
}