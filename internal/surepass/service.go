package surepass

import "github.com/santosh0921/nbfc-backend/internal/config"

var API *Client

func Initialize(cfg *config.Config) {
	API = NewClient(
		cfg.SurepassBaseURL,
		cfg.SurepassBearerToken,
	)
}