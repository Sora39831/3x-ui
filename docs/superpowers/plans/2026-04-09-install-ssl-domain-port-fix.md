# Install SSL Domain and Port Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the install flow so a user-chosen port is persisted, Cloudflare SSL writes certificate paths into panel settings, and the final access URL uses the configured domain for domain certificates while only falling back to IP for IP certificates.

**Architecture:** Keep the fix narrow and local to the install and settings paths. The shell installer should collect the domain once, persist it through the existing `x-ui setting` command, and decide the final URL host from the SSL mode instead of from a generic fallback. The Go settings layer should expose the domain field in the command-line settings updater so the installer can write the configured hostname into the same JSON-backed settings file as the port and certificate paths.

**Tech Stack:** Bash installer script, Go CLI entrypoint, JSON-backed settings service, Go unit tests.

---

### Task 1: Persist panel domain and verify SSL paths in the Go CLI

**Files:**
- Modify: `main.go`
- Modify: `web/service/setting.go`
- Modify: `web/service/setting_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestUpdateAllSettingCanPersistWebDomain(t *testing.T) {
	setupTestSettings(t)

	svc := &SettingService{}
	if err := svc.setString("webDomain", "panel.example.com"); err != nil {
		t.Fatalf("setString webDomain error: %v", err)
	}

	got, err := svc.GetWebDomain()
	if err != nil {
		t.Fatalf("GetWebDomain error: %v", err)
	}
	if got != "panel.example.com" {
		t.Fatalf("expected webDomain to be persisted, got %q", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails or is missing the CLI path**

Run: `go test ./web/service -run TestUpdateAllSettingCanPersistWebDomain -v`
Expected: pass only after the CLI writes `webDomain`; before the change, there is no installer path using it.

- [ ] **Step 3: Write minimal implementation**

```go
// in main.go setting command handling
var webDomain string
settingCmd.StringVar(&webDomain, "webDomain", "", "Set panel domain")

// in updateSetting
if webDomain != "" {
	if err := settingService.SetWebDomain(webDomain); err != nil {
		fmt.Println("Failed to set web domain:", err)
	} else {
		fmt.Printf("Web domain set successfully: %v\n", webDomain)
	}
}
```

```go
// in web/service/setting.go
func (s *SettingService) GetWebDomain() (string, error) {
	return s.getString("webDomain")
}

func (s *SettingService) SetWebDomain(domain string) error {
	return s.setString("webDomain", domain)
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./web/service -run TestUpdateAllSettingCanPersistWebDomain -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add main.go web/service/setting.go web/service/setting_test.go
git commit -m "fix: persist panel domain in settings"
```

### Task 2: Make install.sh write the configured domain and certificate paths, then derive the final URL host correctly

**Files:**
- Modify: `install.sh`

- [ ] **Step 1: Write the failing test**

```bash
#!/bin/bash
# Add a shell-level regression check by running the installer in a controlled
# environment and confirming it emits the configured domain in the final URL
# when Cloudflare SSL is selected, and the configured port in the saved settings.
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash install.sh` in a sandboxed test environment with mocked inputs for:
`port`, `webBasePath`, `SSL choice = Cloudflare`, and `cf_domain`.
Expected: before the fix, the final URL can still print an IP host or omit the domain from settings.

- [ ] **Step 3: Write minimal implementation**

```bash
# After collecting `cf_domain`, persist it alongside port and path.
${xui_folder}/x-ui setting -username "${config_username}" -password "${config_password}" -port "${config_port}" -webBasePath "${config_webBasePath}" -webDomain "${cf_domain}"

# After writing cert paths, confirm the settings file actually contains them.
${xui_folder}/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile" >/dev/null 2>&1
current_cert=$(${xui_folder}/x-ui setting -getCert true | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
current_key=$(${xui_folder}/x-ui setting -getCert true | grep 'key:' | awk -F': ' '{print $2}' | tr -d '[:space:]')

if [[ "$current_cert" != "$webCertFile" || "$current_key" != "$webKeyFile" ]]; then
	echo -e "${red}证书路径写入失败，已终止。${plain}"
	return 1
fi

# Track the host used for final output separately from the current server IP.
case "$ssl_choice" in
	1|4)
		SSL_HOST="${domain_or_cf_domain}"
		;;
	2)
		SSL_HOST="${server_ip}"
		;;
	*)
		SSL_HOST="${server_ip}"
		;;
esac
```

```bash
# For Cloudflare and domain SSL modes, print the domain-based access URL.
echo -e "${green}访问地址：  https://${SSL_HOST}:${config_port}/${config_webBasePath}${plain}"
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash install.sh` with mocked answers for the four SSL branches.
Expected: Cloudflare and domain SSL branches print the entered domain; IP SSL prints the IP.

- [ ] **Step 5: Commit**

```bash
git add install.sh
git commit -m "fix: use configured domain and verify ssl settings in installer"
```

### Task 3: Add regression coverage for settings round-trip and URL selection behavior

**Files:**
- Modify: `web/service/setting_test.go`
- Modify: `web/html/settings.html` only if the post-restart URL selection needs a matching frontend update

- [ ] **Step 1: Write the failing test**

```go
func TestSettingServiceStoresWebDomainAlongsidePortAndCert(t *testing.T) {
	setupTestSettings(t)

	svc := &SettingService{}
	if err := svc.SetPort(8443); err != nil {
		t.Fatalf("SetPort error: %v", err)
	}
	if err := svc.SetWebDomain("panel.example.com"); err != nil {
		t.Fatalf("SetWebDomain error: %v", err)
	}
	if err := svc.SetCertFile("/root/cert/panel.example.com/fullchain.pem"); err != nil {
		t.Fatalf("SetCertFile error: %v", err)
	}
	if err := svc.SetKeyFile("/root/cert/panel.example.com/privkey.pem"); err != nil {
		t.Fatalf("SetKeyFile error: %v", err)
	}

	allSetting, err := svc.GetAllSetting()
	if err != nil {
		t.Fatalf("GetAllSetting error: %v", err)
	}
	if allSetting.WebPort != 8443 || allSetting.WebDomain != "panel.example.com" {
		t.Fatalf("unexpected stored values: %+v", allSetting)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./web/service -run TestSettingServiceStoresWebDomainAlongsidePortAndCert -v`
Expected: fails until `SetWebDomain` exists and the settings round-trip is covered.

- [ ] **Step 3: Write minimal implementation**

Use the new `SetWebDomain` and `GetWebDomain` methods, plus any tiny frontend adjustment only if the current post-restart URL logic still prefers IP when a domain cert is configured.

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./web/service -run TestSettingServiceStoresWebDomainAlongsidePortAndCert -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/service/setting_test.go web/html/settings.html
git commit -m "test: cover web domain settings round-trip"
```

---

### Self-Review

- [ ] No placeholders remain.
- [ ] Each requirement maps to a task.
- [ ] Domain persistence is written through the same JSON-backed settings path as port and cert files.
- [ ] Final installer output distinguishes domain SSL from IP SSL.
- [ ] Tests cover the new settings round-trip.
