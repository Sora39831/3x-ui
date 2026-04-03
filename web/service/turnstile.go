package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
)

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

var turnstileClient = &http.Client{Timeout: 10 * time.Second}

type turnstileResponse struct {
	Success bool `json:"success"`
}

// VerifyTurnstile verifies a Cloudflare Turnstile token with the given secret key.
func VerifyTurnstile(secretKey, token, remoteIP string) bool {
	form := url.Values{
		"secret":   {secretKey},
		"response": {token},
	}
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	resp, err := turnstileClient.PostForm(turnstileVerifyURL, form)
	if err != nil {
		logger.Warning("Turnstile verification request failed (network error):", err)
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		logger.Warning("Turnstile verification failed to read response:", err)
		return false
	}

	var result turnstileResponse
	if err := json.Unmarshal(body, &result); err != nil {
		logger.Warning("Turnstile verification failed to parse response:", err)
		return false
	}
	return result.Success
}
