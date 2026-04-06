package job

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

var (
	activeClientIPs   = make(map[string]map[string]time.Time)
	activeClientsLock sync.RWMutex

	clientStatus   = make(map[string]bool)
	clientStatusMu sync.RWMutex
)

type CheckDeviceLimitJob struct {
	inboundService service.InboundService
	xrayService    *service.XrayService
	xrayAPI        xray.XrayAPI
	lastPosition   int64

	violationStartTime map[string]time.Time
	violationMu        sync.Mutex
	running            atomic.Bool

	isXrayRunning    func() bool
	getAPIPort       func() int
	loadAllInbounds  func() ([]*model.Inbound, error)
	getClientTraffic func(email string) (*xray.ClientTraffic, error)
	getClientByEmail func(email string) (*xray.ClientTraffic, *model.Client, error)
	apiInit          func(apiPort int) error
	apiClose         func()
	removeUser       func(inboundTag, email string) error
	addUser          func(protocol, inboundTag string, user map[string]any) error
	sleep            func(time.Duration)
}

type deviceInboundInfo struct {
	Limit    int
	Tag      string
	Protocol model.Protocol
	Enable   bool
}

func NewCheckDeviceLimitJob(xrayService *service.XrayService) *CheckDeviceLimitJob {
	j := &CheckDeviceLimitJob{
		xrayService:        xrayService,
		violationStartTime: make(map[string]time.Time),
	}
	j.isXrayRunning = func() bool {
		return j.xrayService != nil && j.xrayService.IsXrayRunning()
	}
	j.getAPIPort = func() int {
		if j.xrayService == nil {
			return 0
		}
		return j.xrayService.GetAPIPort()
	}
	j.loadAllInbounds = func() ([]*model.Inbound, error) {
		db := database.GetDB()
		var inbounds []*model.Inbound
		err := db.Find(&inbounds).Error
		return inbounds, err
	}
	j.getClientTraffic = func(email string) (*xray.ClientTraffic, error) {
		return j.inboundService.GetClientTrafficByEmail(email)
	}
	j.getClientByEmail = func(email string) (*xray.ClientTraffic, *model.Client, error) {
		return j.inboundService.GetClientByEmail(email)
	}
	j.apiInit = j.xrayAPI.Init
	j.apiClose = j.xrayAPI.Close
	j.removeUser = j.xrayAPI.RemoveUser
	j.addUser = j.xrayAPI.AddUser
	j.sleep = time.Sleep
	return j
}

func (j *CheckDeviceLimitJob) Run() {
	// Avoid concurrent re-entrancy when previous run is still processing.
	if !j.running.CompareAndSwap(false, true) {
		return
	}
	defer j.running.Store(false)

	if j.isXrayRunning == nil || !j.isXrayRunning() {
		return
	}
	j.cleanupExpiredIPs()
	j.parseAccessLog()
	j.checkAllClientsLimit()
}

func (j *CheckDeviceLimitJob) cleanupExpiredIPs() {
	activeClientsLock.Lock()
	defer activeClientsLock.Unlock()

	now := time.Now()
	const activeTTL = 3 * time.Minute
	for email, ips := range activeClientIPs {
		for ip, lastSeen := range ips {
			if now.Sub(lastSeen) > activeTTL {
				delete(activeClientIPs[email], ip)
			}
		}
		if len(activeClientIPs[email]) == 0 {
			delete(activeClientIPs, email)
		}
	}
}

func (j *CheckDeviceLimitJob) parseAccessLog() {
	logPath, err := xray.GetAccessLogPath()
	if err != nil || logPath == "none" || logPath == "" {
		return
	}

	file, err := os.Open(logPath)
	if err != nil {
		return
	}
	defer file.Close()

	if _, err = file.Seek(j.lastPosition, 0); err != nil {
		return
	}

	scanner := bufio.NewScanner(file)
	emailRegex := regexp.MustCompile(`email: ([^ ]+)`)
	ipRegex := regexp.MustCompile(`from (?:tcp:|udp:)?\[?([0-9a-fA-F\.:]+)\]?:\d+ accepted`)

	activeClientsLock.Lock()
	defer activeClientsLock.Unlock()

	now := time.Now()
	for scanner.Scan() {
		line := scanner.Text()
		emailMatch := emailRegex.FindStringSubmatch(line)
		ipMatch := ipRegex.FindStringSubmatch(line)
		if len(emailMatch) <= 1 || len(ipMatch) <= 1 {
			continue
		}

		email := emailMatch[1]
		ip := ipMatch[1]
		if ip == "127.0.0.1" || ip == "::1" {
			continue
		}

		if _, ok := activeClientIPs[email]; !ok {
			activeClientIPs[email] = make(map[string]time.Time)
		}
		activeClientIPs[email][ip] = now
	}

	currentPosition, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return
	}
	if currentPosition < j.lastPosition {
		j.lastPosition = 0
		return
	}
	j.lastPosition = currentPosition
}

func (j *CheckDeviceLimitJob) checkAllClientsLimit() {
	if j.loadAllInbounds == nil {
		return
	}
	allInbounds, err := j.loadAllInbounds()
	if err != nil || len(allInbounds) == 0 {
		return
	}

	apiPort := j.getAPIPort()
	if apiPort == 0 {
		return
	}
	if err := j.apiInit(apiPort); err != nil {
		return
	}
	defer j.apiClose()

	inboundInfoMap := make(map[int]deviceInboundInfo, len(allInbounds))
	for _, inbound := range allInbounds {
		inboundInfoMap[inbound.Id] = deviceInboundInfo{
			Limit:    inbound.DeviceLimit,
			Tag:      inbound.Tag,
			Protocol: inbound.Protocol,
			Enable:   inbound.Enable,
		}
	}

	activeCounts := make(map[string]int)
	activeClientsLock.RLock()
	for email, ips := range activeClientIPs {
		activeCounts[email] = len(ips)
	}
	activeClientsLock.RUnlock()

	bannedSnapshot := make(map[string]bool)
	clientStatusMu.RLock()
	for email, banned := range clientStatus {
		if banned {
			bannedSnapshot[email] = true
		}
	}
	clientStatusMu.RUnlock()

	for email, activeIPCount := range activeCounts {
		traffic, err := j.getClientTraffic(email)
		if err != nil || traffic == nil {
			continue
		}

		info, ok := inboundInfoMap[traffic.InboundId]
		if !ok {
			continue
		}

		isBanned := j.isClientBanned(email)
		enforcementEnabled := info.Enable && info.Limit > 0

		// If this client was previously banned but device-limit is now disabled
		// (or inbound is disabled), immediately restore access.
		if isBanned && !enforcementEnabled {
			j.violationMu.Lock()
			delete(j.violationStartTime, email)
			j.violationMu.Unlock()
			j.unbanUser(email, activeIPCount, info)
			continue
		}

		if !enforcementEnabled {
			continue
		}

		if activeIPCount > info.Limit && !isBanned {
			j.violationMu.Lock()
			startTime, exists := j.violationStartTime[email]
			if !exists {
				j.violationStartTime[email] = time.Now()
				j.violationMu.Unlock()
				continue
			}
			if time.Since(startTime) < 3*time.Minute {
				j.violationMu.Unlock()
				continue
			}
			delete(j.violationStartTime, email)
			j.violationMu.Unlock()

			j.banUser(email, activeIPCount, info)
		}

		if activeIPCount <= info.Limit {
			j.violationMu.Lock()
			delete(j.violationStartTime, email)
			j.violationMu.Unlock()

			if isBanned {
				j.unbanUser(email, activeIPCount, info)
			}
		}
	}

	for email := range bannedSnapshot {
		if _, online := activeCounts[email]; online {
			continue
		}
		traffic, err := j.getClientTraffic(email)
		if err != nil || traffic == nil {
			continue
		}
		info, ok := inboundInfoMap[traffic.InboundId]
		if !ok {
			// Inbound no longer exists; clear stale ban marker.
			j.clearClientBanned(email)
			j.violationMu.Lock()
			delete(j.violationStartTime, email)
			j.violationMu.Unlock()
			continue
		}
		// Offline users should be restored when enforcement is disabled too.
		if !info.Enable || info.Limit <= 0 {
			j.unbanUser(email, 0, info)
			continue
		}
		j.unbanUser(email, 0, info)
	}
}

func (j *CheckDeviceLimitJob) banUser(email string, activeIPCount int, info deviceInboundInfo) {
	_, client, err := j.getClientByEmail(email)
	if err != nil || client == nil {
		return
	}

	logger.Infof("[DeviceLimit] banning email=%s limit=%d current=%d", email, info.Limit, activeIPCount)
	_ = j.removeUser(info.Tag, email)
	j.sleep(5 * time.Second)

	tempClient := *client
	if tempClient.ID != "" {
		tempClient.ID = randomUUID()
	}
	if tempClient.Password != "" {
		tempClient.Password = randomUUID()
	}

	clientMap := map[string]any{}
	clientJSON, _ := json.Marshal(tempClient)
	_ = json.Unmarshal(clientJSON, &clientMap)

	if err = j.addUser(string(info.Protocol), info.Tag, clientMap); err != nil {
		logger.Warningf("[DeviceLimit] failed to ban user %s: %v", email, err)
		return
	}
	j.setClientBanned(email, true)
}

func (j *CheckDeviceLimitJob) unbanUser(email string, activeIPCount int, info deviceInboundInfo) {
	_, client, err := j.getClientByEmail(email)
	if err != nil || client == nil {
		return
	}

	logger.Infof("[DeviceLimit] unbanning email=%s limit=%d current=%d", email, info.Limit, activeIPCount)
	_ = j.removeUser(info.Tag, email)
	j.sleep(5 * time.Second)

	clientMap := map[string]any{}
	clientJSON, _ := json.Marshal(client)
	_ = json.Unmarshal(clientJSON, &clientMap)

	if err = j.addUser(string(info.Protocol), info.Tag, clientMap); err != nil {
		logger.Warningf("[DeviceLimit] failed to restore user %s: %v", email, err)
		return
	}
	j.clearClientBanned(email)
}

func randomUUID() string {
	uuid := make([]byte, 16)
	if _, err := rand.Read(uuid); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return hex.EncodeToString(uuid[0:4]) + "-" + hex.EncodeToString(uuid[4:6]) + "-" + hex.EncodeToString(uuid[6:8]) + "-" + hex.EncodeToString(uuid[8:10]) + "-" + hex.EncodeToString(uuid[10:16])
}

func (j *CheckDeviceLimitJob) isClientBanned(email string) bool {
	clientStatusMu.RLock()
	defer clientStatusMu.RUnlock()
	return clientStatus[email]
}

func (j *CheckDeviceLimitJob) setClientBanned(email string, banned bool) {
	clientStatusMu.Lock()
	clientStatus[email] = banned
	clientStatusMu.Unlock()
}

func (j *CheckDeviceLimitJob) clearClientBanned(email string) {
	clientStatusMu.Lock()
	delete(clientStatus, email)
	clientStatusMu.Unlock()
}
