package surepass

import (
	"encoding/json"
)

type PANRequest struct {
	IDNumber string `json:"id_number"`
}

func (c *Client) VerifyPAN(pan string) (*PANResponse, error) {

	req := PANRequest{
		IDNumber: pan,
	}

	body, err := c.Post("/api/v1/pan/pan-comprehensive", req)
	if err != nil {
		return nil, err
	}

	var response PANResponse

	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}