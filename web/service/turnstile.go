package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"
)

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

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

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(turnstileVerifyURL, form)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	var result turnstileResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return false
	}
	return result.Success
}
