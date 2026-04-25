# 2026-04-25 Security: Fix login rate limiting and IP spoofing

## Changes
- Add `RateLimitMiddleware(10, time.Minute)` to `POST /login` endpoint (was unprotected, only register had rate limiting)
- Fix `getRemoteIp()` to use `c.Request.RemoteAddr` instead of trusting `X-Real-IP` / `X-Forwarded-For` headers
- Fix `RateLimitMiddleware` to use `RemoteAddr` directly, preventing IP-based rate limit bypass via header spoofing

## Security Issue
- Login endpoint had zero rate limiting, enabling unlimited brute-force attempts
- Both IP extraction and rate limiter trusted client-supplied headers, allowing attackers to spoof IPs and bypass all rate limiting

## Files Modified
- `web/controller/index.go` — add rate limit middleware to login route
- `web/controller/util.go` — use RemoteAddr in getRemoteIp()
- `web/middleware/ratelimit.go` — use RemoteAddr in rate limiter

## Note
- Trusts Cloudflare's `CF-Connecting-IP` header (CF overwrites it, clients cannot spoof)
- Falls back to `RemoteAddr` for direct connections without CDN
- `X-Real-IP` / `X-Forwarded-For` are NOT trusted (can be spoofed by clients)
