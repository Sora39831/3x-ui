# Geofiles Scheduled Update Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add configurable scheduled geofile updates, settable from both the web panel and x-ui.sh.

**Architecture:** Follow the existing BackupJob pattern — a cron job registered as `@every 1m` that checks a `shouldRun()` guard based on settings stored in `x-ui.json`. Only the master node executes actual updates; workers sync via shared DB version bump.

**Tech Stack:** Go (robfig/cron/v3), Vue.js (Ant Design Vue), Bash (x-ui.sh), JSON config

---

### Task 1: Add entity fields and settings infrastructure

**Files:**
- Modify: `web/entity/entity.go` (add fields after BackupMaxCount)
- Modify: `web/service/setting.go` (add defaults, group mapping, getters, GetDefaultSettings)

- [ ] **Step 1: Add fields to AllSetting struct**

In `web/entity/entity.go`, after `BackupMaxCount` (line 129), add:

```go
// Geofile update schedule settings
GeofileUpdateEnabled   bool   `json:"geofileUpdateEnabled" form:"geofileUpdateEnabled"`
GeofileUpdateFrequency string `json:"geofileUpdateFrequency" form:"geofileUpdateFrequency"`
GeofileUpdateHour      int    `json:"geofileUpdateHour" form:"geofileUpdateHour"`
```

- [ ] **Step 2: Add defaultValueMap entries**

In `web/service/setting.go`, after line 135 (`"backupMaxCount": "10",`), add:

```go
// Geofile update schedule settings
"geofileUpdateEnabled":   "false",
"geofileUpdateFrequency": "daily",
"geofileUpdateHour":      "4",
```

- [ ] **Step 3: Add settingGroups mapping**

In `web/service/setting.go`, after the `"backup"` group (lines 256-261), add:

```go
"geofileUpdate": {
    "enabled":   "geofileUpdateEnabled",
    "frequency": "geofileUpdateFrequency",
    "hour":      "geofileUpdateHour",
},
```

- [ ] **Step 4: Add getter methods**

In `web/service/setting.go`, after `GetBackupMaxCount()` (line 1167), add:

```go
func (s *SettingService) GetGeofileUpdateEnabled() (bool, error) {
	return s.getBool("geofileUpdateEnabled")
}

func (s *SettingService) GetGeofileUpdateFrequency() (string, error) {
	return s.getString("geofileUpdateFrequency")
}

func (s *SettingService) GetGeofileUpdateHour() (int, error) {
	return s.getInt("geofileUpdateHour")
}
```

- [ ] **Step 5: Add to GetDefaultSettings**

In `web/service/setting.go`, inside the `settings` map in `GetDefaultSettings()` (after line 1251), add:

```go
"geofileUpdateEnabled":   func() (any, error) { return s.GetGeofileUpdateEnabled() },
"geofileUpdateFrequency": func() (any, error) { return s.GetGeofileUpdateFrequency() },
"geofileUpdateHour":      func() (any, error) { return s.GetGeofileUpdateHour() },
```

- [ ] **Step 6: Verify compilation**

```bash
cd /usr/x-ui/3x-ui && go build ./...
```

Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add web/entity/entity.go web/service/setting.go
git commit -m "feat: add geofile update schedule settings infrastructure"
```

---

### Task 2: Create GeofileUpdateJob

**Files:**
- Create: `web/job/geofile_update_job.go`
- Modify: `web/web.go` (register job)

- [ ] **Step 1: Create the job file**

Create `web/job/geofile_update_job.go`:

```go
package job

import (
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// GeofileUpdateJob handles scheduled geofile updates.
type GeofileUpdateJob struct {
	settingService service.SettingService
	serverService  service.ServerService
}

// NewGeofileUpdateJob creates a new GeofileUpdateJob instance.
func NewGeofileUpdateJob() *GeofileUpdateJob {
	return &GeofileUpdateJob{}
}

// Run executes the scheduled geofile update if enabled and time matches the configured frequency.
func (j *GeofileUpdateJob) Run() {
	enabled, err := j.settingService.GetGeofileUpdateEnabled()
	if err != nil || !enabled {
		return
	}

	frequency, err := j.settingService.GetGeofileUpdateFrequency()
	if err != nil {
		return
	}

	if !j.shouldRun(frequency) {
		return
	}

	if err := j.serverService.UpdateGeofile(""); err != nil {
		logger.Warning("scheduled geofile update failed:", err)
		return
	}

	if err := database.BumpSharedGeoVersion(database.GetDB()); err != nil {
		logger.Warning("bump shared geo version failed:", err)
	}
}

// shouldRun checks if the geofile update should run based on the configured frequency.
func (j *GeofileUpdateJob) shouldRun(frequency string) bool {
	now := time.Now()

	switch frequency {
	case "hourly":
		return now.Minute() == 0
	case "every12h":
		return (now.Hour() == 0 || now.Hour() == 12) && now.Minute() == 0
	case "daily":
		hour, err := j.settingService.GetGeofileUpdateHour()
		if err != nil {
			hour = 4
		}
		return now.Hour() == hour && now.Minute() == 0
	case "weekly":
		if now.Weekday() != time.Sunday {
			return false
		}
		hour, err := j.settingService.GetGeofileUpdateHour()
		if err != nil {
			hour = 4
		}
		return now.Hour() == hour && now.Minute() == 0
	default:
		return false
	}
}
```

- [ ] **Step 2: Register job in web.go**

In `web/web.go`, after line 415 (`s.cron.AddJob("@every 1m", job.NewBackupJob())`), add:

```go
s.cron.AddJob("@every 1m", job.NewGeofileUpdateJob())
```

- [ ] **Step 3: Verify compilation**

```bash
cd /usr/x-ui/3x-ui && go build ./...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add web/job/geofile_update_job.go web/web.go
git commit -m "feat: add GeofileUpdateJob for scheduled geofile updates"
```

---

### Task 3: Add UI controls to the panel

**Files:**
- Modify: `web/html/index.html`
- Modify: `web/translation/translate.en_US.toml`
- Modify: `web/translation/translate.zh_CN.toml`

- [ ] **Step 1: Add i18n keys — English**

In `web/translation/translate.en_US.toml`, after line 168 (`"geofileUpdatePopover"`), add:

```toml
"geofileScheduleTitle" = "Scheduled Update"
"geofileScheduleEnable" = "Enable"
"geofileScheduleFrequency" = "Frequency"
"geofileScheduleHour" = "Hour"
"geofileScheduleHourly" = "Hourly"
"geofileScheduleEvery12h" = "Every 12 Hours"
"geofileScheduleDaily" = "Daily"
"geofileScheduleWeekly" = "Weekly"
```

- [ ] **Step 2: Add i18n keys — Chinese**

In `web/translation/translate.zh_CN.toml`, after line 168, add:

```toml
"geofileScheduleTitle" = "定时更新"
"geofileScheduleEnable" = "启用"
"geofileScheduleFrequency" = "更新频率"
"geofileScheduleHour" = "更新小时"
"geofileScheduleHourly" = "每小时"
"geofileScheduleEvery12h" = "每12小时"
"geofileScheduleDaily" = "每天"
"geofileScheduleWeekly" = "每周"
```

- [ ] **Step 3: Add Vue data fields**

In `web/html/index.html`, in the `data:` section, after line 922 (`ipLimitEnable: false,`), add:

```javascript
geofileUpdateEnabled: false,
geofileUpdateFrequency: 'daily',
geofileUpdateHour: 4,
```

- [ ] **Step 4: Load geofile update settings on mount**

In `web/html/index.html`, in the `mounted()` function, after the line that reads `this.ipLimitEnable = msg.obj.ipLimitEnable;` (line 1184), add:

```javascript
this.geofileUpdateEnabled = msg.obj.geofileUpdateEnabled;
this.geofileUpdateFrequency = msg.obj.geofileUpdateFrequency;
this.geofileUpdateHour = msg.obj.geofileUpdateHour;
```

- [ ] **Step 5: Add save method**

In `web/html/index.html`, in the `methods:` section, add after the `syncUpdateGeofile` method (after line 1042):

```javascript
async saveGeofileSchedule() {
  try {
    await HttpUtil.post('/panel/setting/update', {
      geofileUpdateEnabled: this.geofileUpdateEnabled,
      geofileUpdateFrequency: this.geofileUpdateFrequency,
      geofileUpdateHour: this.geofileUpdateHour,
    });
  } catch (e) {
    console.error("Failed to save geofile schedule:", e);
  }
},
```

- [ ] **Step 6: Add UI controls in the Geofiles panel**

In `web/html/index.html`, after the existing `syncUpdateGeofile` button row (after line 344, the closing `</div>` of the buttons), add:

```html
<div class="mt-10">
  <h4 class="mb-5">{{ i18n "pages.index.geofileScheduleTitle" }}</h4>
  <a-form layout="inline">
    <a-form-item :label='{{ i18n "pages.index.geofileScheduleEnable" }}'>
      <a-switch v-model="geofileUpdateEnabled" @change="saveGeofileSchedule" />
    </a-form-item>
    <a-form-item :label='{{ i18n "pages.index.geofileScheduleFrequency" }}'>
      <a-select v-model="geofileUpdateFrequency" style="width: 160px" @change="saveGeofileSchedule"
        :dropdown-class-name="themeSwitcher.currentTheme">
        <a-select-option value="hourly">{{ i18n "pages.index.geofileScheduleHourly" }}</a-select-option>
        <a-select-option value="every12h">{{ i18n "pages.index.geofileScheduleEvery12h" }}</a-select-option>
        <a-select-option value="daily">{{ i18n "pages.index.geofileScheduleDaily" }}</a-select-option>
        <a-select-option value="weekly">{{ i18n "pages.index.geofileScheduleWeekly" }}</a-select-option>
      </a-select>
    </a-form-item>
    <a-form-item v-if="geofileUpdateFrequency === 'daily' || geofileUpdateFrequency === 'weekly'"
      :label='{{ i18n "pages.index.geofileScheduleHour" }}'>
      <a-select v-model="geofileUpdateHour" style="width: 80px" @change="saveGeofileSchedule"
        :dropdown-class-name="themeSwitcher.currentTheme">
        <a-select-option v-for="h in 24" :key="h-1" :value="h-1">[[ h-1 ]]</a-select-option>
      </a-select>
    </a-form-item>
  </a-form>
</div>
```

- [ ] **Step 7: Verify compilation**

```bash
cd /usr/x-ui/3x-ui && go build ./...
```

Expected: no errors

- [ ] **Step 8: Commit**

```bash
git add web/html/index.html web/translation/translate.en_US.toml web/translation/translate.zh_CN.toml
git commit -m "feat: add geofile scheduled update UI controls to panel"
```

---

### Task 4: Add geofile-cron command to x-ui.sh

**Files:**
- Modify: `x-ui.sh`

- [ ] **Step 1: Add geofile-cron functions**

In `x-ui.sh`, after the `update_all_geofiles` function (after line 1081), add:

```bash
# Config file path (must match config.GetSettingPath())
SETTING_FILE="/etc/x-ui/x-ui.json"

# Helper: read a value from x-ui.json using python3 or jq
read_geofile_setting() {
    local key="$1"
    if command -v python3 &>/dev/null; then
        python3 -c "
import json, sys
try:
    with open('$SETTING_FILE') as f:
        data = json.load(f)
    print(data.get('geofileUpdate', {}).get('$key', ''))
except: pass
" 2>/dev/null
    elif command -v jq &>/dev/null; then
        jq -r ".geofileUpdate.$key // empty" "$SETTING_FILE" 2>/dev/null
    fi
}

# Helper: write geofileUpdate section to x-ui.json
write_geofile_setting() {
    local enabled="$1"
    local frequency="$2"
    local hour="$3"
    if command -v python3 &>/dev/null; then
        python3 -c "
import json
try:
    with open('$SETTING_FILE') as f:
        data = json.load(f)
except:
    data = {}
data['geofileUpdate'] = {'enabled': $enabled, 'frequency': '$frequency', 'hour': $hour}
with open('$SETTING_FILE', 'w') as f:
    json.dump(data, f, indent=2)
print('ok')
" 2>/dev/null
    else
        tmp=$(mktemp)
        jq ".geofileUpdate = {\"enabled\": $enabled, \"frequency\": \"$frequency\", \"hour\": $hour}" "$SETTING_FILE" > "$tmp" && mv "$tmp" "$SETTING_FILE"
    fi
}

geofile_cron_status() {
    if [ ! -f "$SETTING_FILE" ]; then
        echo -e "${red}x-ui.json not found at $SETTING_FILE${plain}"
        return 1
    fi
    local enabled=$(read_geofile_setting "enabled")
    local frequency=$(read_geofile_setting "frequency")
    local hour=$(read_geofile_setting "hour")
    echo -e "${green}Geofile Scheduled Update:${plain}"
    echo -e "  Enabled:   ${green}${enabled:-false}${plain}"
    echo -e "  Frequency: ${green}${frequency:-daily}${plain}"
    echo -e "  Hour:      ${green}${hour:-4}${plain}"
}

geofile_cron_enable() {
    local frequency="daily"
    local hour="4"
    while [ $# -gt 0 ]; do
        case "$1" in
            --frequency) frequency="$2"; shift 2;;
            --hour) hour="$2"; shift 2;;
            *) shift;;
        esac
    done
    case "$frequency" in
        hourly|every12h|daily|weekly) ;;
        *) echo -e "${red}Invalid frequency: $frequency (must be hourly, every12h, daily, or weekly)${plain}"; return 1;;
    esac
    if ! [ "$hour" -ge 0 ] 2>/dev/null || ! [ "$hour" -le 23 ] 2>/dev/null; then
        echo -e "${red}Invalid hour: $hour (must be 0-23)${plain}"
        return 1
    fi
    write_geofile_setting "true" "$frequency" "$hour"
    echo -e "${green}Geofile scheduled update enabled (frequency=$frequency, hour=$hour)${plain}"
    echo -e "${yellow}Restarting x-ui to apply changes...${plain}"
    systemctl restart x-ui
}

geofile_cron_disable() {
    write_geofile_setting "false" "daily" "4"
    echo -e "${green}Geofile scheduled update disabled${plain}"
    echo -e "${yellow}Restarting x-ui to apply changes...${plain}"
    systemctl restart x-ui
}
```

- [ ] **Step 2: Add CLI entry point**

In `x-ui.sh`, find the `"update-all-geofiles")` case in the main argument parser (line 3921) and add after it:

```bash
"geofile-cron")
    shift
    case "${1}" in
        "--enable") geofile_cron_enable "${@}";;
        "--disable") geofile_cron_disable;;
        "--status") geofile_cron_status;;
        *) echo -e "${red}Usage: x-ui geofile-cron [--enable --frequency daily --hour 4 | --disable | --status]${plain}";;
    esac
    ;;
```

- [ ] **Step 3: Add help text**

Find the help display containing `x-ui update-all-geofiles` (line 2309) and add below it:

```bash
│  ${blue}x-ui geofile-cron --enable${plain}          - 启用 Geofile 定时更新 (通过更新 x-ui.json 配置) │
│  ${blue}x-ui geofile-cron --disable${plain}         - 禁用 Geofile 定时更新                         │
│  ${blue}x-ui geofile-cron --status${plain}          - 查看当前 Geofile 定时更新状态                    │
```

- [ ] **Step 4: Verify syntax**

```bash
bash -n /usr/x-ui/3x-ui/x-ui.sh
```

Expected: no output (no syntax errors)

- [ ] **Step 5: Commit**

```bash
git add x-ui.sh
git commit -m "feat: add geofile-cron command to x-ui.sh"
```

---

### Task 5: Go fmt and final verification

- [ ] **Step 1: Run gofmt on all Go files**

```bash
cd /usr/x-ui/3x-ui && gofmt -w web/entity/entity.go web/service/setting.go web/job/geofile_update_job.go web/web.go
```

- [ ] **Step 2: Verify build compiles**

```bash
cd /usr/x-ui/3x-ui && go build ./...
```

Expected: no errors

- [ ] **Step 3: Run existing tests**

```bash
cd /usr/x-ui/3x-ui && go test ./web/... -count=1 -timeout 60s 2>&1 | tail -20
```

Expected: all pass

- [ ] **Step 4: Commit**

```bash
git add -u
git commit -m "chore: gofmt and final polish for geofile scheduled update"
```

---

### Post-Implementation Checklist

- [ ] Panel UI: Geofiles modal shows schedule controls with toggle, frequency dropdown, and hour selector
- [ ] Panel UI: Changing dropdown values triggers save via `/panel/setting/update`
- [ ] Panel UI: Hour selector only visible for `daily` and `weekly` frequencies
- [ ] Job: Only executes when enabled and at the correct time
- [ ] Job: Calls `UpdateGeofile("")` then `BumpSharedGeoVersion` on master
- [ ] x-ui.sh: `geofile-cron --status` reads and displays config
- [ ] x-ui.sh: `geofile-cron --enable --frequency daily --hour 4` writes config and restarts
- [ ] x-ui.sh: `geofile-cron --disable` disables and restarts
- [ ] Settings persist across panel restarts via `x-ui.json`
