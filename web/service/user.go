package service

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/crypto"
	ldaputil "github.com/mhsanaei/3x-ui/v2/util/ldap"
	"github.com/xlzd/gotp"
	"gorm.io/gorm"
)

// ErrUsernameAlreadyExists is returned when a user tries to register with a taken username.
var ErrUsernameAlreadyExists = errors.New("username already exists")
var ErrInvalidUserRole = errors.New("role must be admin or user")
var ErrUserNotFound = errors.New("user not found")
var ErrCannotDeleteSelf = errors.New("cannot delete current user")
var ErrLastAdminRequired = errors.New("at least one admin user must remain")
var ErrCannotDemoteSelf = errors.New("cannot change your own role to non-admin")

// UserInfo is the sanitized user payload returned to the frontend.
type UserInfo struct {
	Id       int    `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// UserService provides business logic for user management and authentication.
// It handles user creation, login, password management, and 2FA operations.
type UserService struct {
	settingService SettingService
}

func normalizeManagedUserInput(username string, password string, role string, passwordRequired bool) (string, string, string, error) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		role = "user"
	}

	if username == "" {
		return "", "", "", errors.New("username can not be empty")
	}
	if len(username) < 3 || len(username) > 64 {
		return "", "", "", errors.New("username must be 3-64 characters")
	}
	if role != "admin" && role != "user" {
		return "", "", "", ErrInvalidUserRole
	}
	if passwordRequired && password == "" {
		return "", "", "", errors.New("password can not be empty")
	}
	if password != "" && (len(password) < 8 || len(password) > 128) {
		return "", "", "", errors.New("password must be 8-128 characters")
	}
	return username, password, role, nil
}

func sanitizeUser(user *model.User) *UserInfo {
	if user == nil {
		return nil
	}
	return &UserInfo{
		Id:       user.Id,
		Username: user.Username,
		Role:     user.Role,
	}
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "UNIQUE constraint failed") || strings.Contains(errMsg, "Duplicate")
}

func (s *UserService) countAdmins(tx *gorm.DB) (int64, error) {
	var count int64
	err := tx.Model(&model.User{}).Where("role = ?", "admin").Count(&count).Error
	return count, err
}

func (s *UserService) addUserClientsToAllInbounds(tx *gorm.DB, username string, inboundService *InboundService) error {
	inbounds, err := inboundService.GetAllInbounds()
	if err != nil {
		return err
	}

	for _, inbound := range inbounds {
		clientID := uuid.New().String()
		client := model.Client{
			ID:      clientID,
			Email:   username,
			Enable:  false,
			SubID:   uuid.New().String()[:8],
			Comment: "auto-added on registration",
		}
		if shouldAutoFillVisionFlow(inbound.Protocol, inbound.StreamSettings) {
			client.Flow = "xtls-rprx-vision"
		}

		clientEntry := map[string]any{
			"email":      client.Email,
			"enable":     client.Enable,
			"totalGB":    0,
			"expiryTime": 0,
			"limitIp":    0,
			"subId":      client.SubID,
			"comment":    client.Comment,
			"created_at": 0,
			"updated_at": 0,
		}
		switch inbound.Protocol {
		case "trojan":
			clientEntry["password"] = clientID
		case "shadowsocks":
			clientEntry["password"] = clientID
		default:
			clientEntry["id"] = clientID
		}
		if client.Flow != "" {
			clientEntry["flow"] = client.Flow
		}

		var settings map[string]any
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			return err
		}
		clientsRaw, ok := settings["clients"].([]any)
		if !ok {
			clientsRaw = []any{}
		}
		clientsRaw = append(clientsRaw, clientEntry)
		settings["clients"] = clientsRaw

		newSettings, err := json.Marshal(settings)
		if err != nil {
			return err
		}
		inbound.Settings = string(newSettings)

		if err := tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).Update("settings", inbound.Settings).Error; err != nil {
			return err
		}
		if err := inboundService.AddClientStat(tx, inbound.Id, &client); err != nil {
			return err
		}
	}

	return nil
}

func (s *UserService) removeUserClientsFromAllInbounds(tx *gorm.DB, username string, inboundService *InboundService) error {
	inbounds, err := inboundService.GetAllInbounds()
	if err != nil {
		return err
	}

	for _, inbound := range inbounds {
		var settings map[string]any
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			return err
		}

		clientsRaw, ok := settings["clients"].([]any)
		if !ok {
			continue
		}

		newClients := make([]any, 0, len(clientsRaw))
		removedEmails := make(map[string]struct{})
		for _, clientRaw := range clientsRaw {
			clientMap, ok := clientRaw.(map[string]any)
			if !ok {
				newClients = append(newClients, clientRaw)
				continue
			}

			email, _ := clientMap["email"].(string)
			if strings.EqualFold(email, username) {
				if email != "" {
					removedEmails[email] = struct{}{}
				}
				continue
			}
			newClients = append(newClients, clientRaw)
		}

		if len(removedEmails) == 0 {
			continue
		}

		settings["clients"] = newClients
		newSettings, err := json.Marshal(settings)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).Update("settings", string(newSettings)).Error; err != nil {
			return err
		}

		for email := range removedEmails {
			if err := inboundService.DelClientStat(tx, inbound.Id, email); err != nil {
				return err
			}
			if err := inboundService.DelClientIPs(tx, email); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetFirstUser retrieves the first user from the database.
// This is typically used for initial setup or when there's only one admin user.
func (s *UserService) GetFirstUser() (*model.User, error) {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		First(user).
		Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) CheckUser(username string, password string, twoFactorCode string) (*model.User, error) {
	db := database.GetDB()

	user := &model.User{}

	err := db.Model(model.User{}).
		Where("username = ?", username).
		First(user).
		Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.New("invalid credentials")
	} else if err != nil {
		logger.Warning("check user err:", err)
		return nil, err
	}

	if !crypto.CheckPasswordHash(user.Password, password) {
		ldapEnabled, _ := s.settingService.GetLdapEnable()
		if !ldapEnabled {
			return nil, errors.New("invalid credentials")
		}

		host, _ := s.settingService.GetLdapHost()
		port, _ := s.settingService.GetLdapPort()
		useTLS, _ := s.settingService.GetLdapUseTLS()
		bindDN, _ := s.settingService.GetLdapBindDN()
		ldapPass, _ := s.settingService.GetLdapPassword()
		baseDN, _ := s.settingService.GetLdapBaseDN()
		userFilter, _ := s.settingService.GetLdapUserFilter()
		userAttr, _ := s.settingService.GetLdapUserAttr()

		cfg := ldaputil.Config{
			Host:       host,
			Port:       port,
			UseTLS:     useTLS,
			BindDN:     bindDN,
			Password:   ldapPass,
			BaseDN:     baseDN,
			UserFilter: userFilter,
			UserAttr:   userAttr,
		}
		ok, err := ldaputil.AuthenticateUser(cfg, username, password)
		if err != nil || !ok {
			return nil, errors.New("invalid credentials")
		}
	}

	twoFactorEnable, err := s.settingService.GetTwoFactorEnable()
	if err != nil {
		logger.Warning("check two factor err:", err)
		return nil, err
	}

	if twoFactorEnable {
		twoFactorToken, err := s.settingService.GetTwoFactorToken()

		if err != nil {
			logger.Warning("check two factor token err:", err)
			return nil, err
		}

		if gotp.NewDefaultTOTP(twoFactorToken).Now() != twoFactorCode {
			return nil, errors.New("invalid 2fa code")
		}
	}

	return user, nil
}

func (s *UserService) UpdateUser(id int, username string, password string) error {
	db := database.GetDB()
	hashedPassword, err := crypto.HashPasswordAsBcrypt(password)

	if err != nil {
		return err
	}

	twoFactorEnable, err := s.settingService.GetTwoFactorEnable()
	if err != nil {
		return err
	}

	if twoFactorEnable {
		s.settingService.SetTwoFactorEnable(false)
		s.settingService.SetTwoFactorToken("")
	}

	return db.Model(model.User{}).
		Where("id = ?", id).
		Updates(map[string]any{"username": username, "password": hashedPassword}).
		Error
}

// GetUsers returns all panel users without sensitive fields.
func (s *UserService) GetUsers() ([]UserInfo, error) {
	db := database.GetDB()
	users := make([]UserInfo, 0)
	err := db.Model(&model.User{}).
		Select("id", "username", "role").
		Order("id asc").
		Find(&users).
		Error
	return users, err
}

// CreateUser creates a new managed user.
func (s *UserService) CreateUser(username string, password string, role string, inboundService *InboundService) (*UserInfo, error) {
	username, password, role, err := normalizeManagedUserInput(username, password, role, true)
	if err != nil {
		return nil, err
	}

	hashedPassword, err := crypto.HashPasswordAsBcrypt(password)
	if err != nil {
		return nil, err
	}

	db := database.GetDB()
	user := &model.User{
		Username: username,
		Password: hashedPassword,
		Role:     role,
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			if isUniqueConstraintError(err) {
				return ErrUsernameAlreadyExists
			}
			return err
		}
		if role == "user" {
			if err := s.addUserClientsToAllInbounds(tx, username, inboundService); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return sanitizeUser(user), nil
}

// UpdateManagedUser updates username, password, and role for a managed user.
func (s *UserService) UpdateManagedUser(id int, username string, password string, role string, currentUserId int) (*UserInfo, error) {
	username, password, role, err := normalizeManagedUserInput(username, password, role, false)
	if err != nil {
		return nil, err
	}

	db := database.GetDB()
	user := &model.User{}
	if err := db.Model(&model.User{}).Where("id = ?", id).First(user).Error; err != nil {
		if database.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if currentUserId == id && role != "admin" {
		return nil, ErrCannotDemoteSelf
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if user.Role == "admin" && role != "admin" {
			adminCount, err := s.countAdmins(tx)
			if err != nil {
				return err
			}
			if adminCount <= 1 {
				return ErrLastAdminRequired
			}
		}

		updates := map[string]any{
			"username": username,
			"role":     role,
		}
		if password != "" {
			hashedPassword, err := crypto.HashPasswordAsBcrypt(password)
			if err != nil {
				return err
			}
			updates["password"] = hashedPassword
		}

		if err := tx.Model(&model.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			if isUniqueConstraintError(err) {
				return ErrUsernameAlreadyExists
			}
			return err
		}
		return tx.Model(&model.User{}).Where("id = ?", id).First(user).Error
	})
	if err != nil {
		return nil, err
	}
	return sanitizeUser(user), nil
}

// DeleteUser deletes a managed user.
func (s *UserService) DeleteUser(id int, currentUserId int, inboundService *InboundService) error {
	if id == currentUserId {
		return ErrCannotDeleteSelf
	}

	db := database.GetDB()
	user := &model.User{}
	if err := db.Model(&model.User{}).Where("id = ?", id).First(user).Error; err != nil {
		if database.IsNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}

	if user.Role == "admin" {
		adminCount, err := s.countAdmins(db)
		if err != nil {
			return err
		}
		if adminCount <= 1 {
			return ErrLastAdminRequired
		}
	}

	if inboundService == nil {
		inboundService = &InboundService{}
	}
	inbounds, err := inboundService.GetInbounds(id)
	if err != nil {
		return err
	}
	for _, inbound := range inbounds {
		if _, err := inboundService.DelInbound(inbound.Id); err != nil {
			return err
		}
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := s.removeUserClientsFromAllInbounds(tx, user.Username, inboundService); err != nil {
			return err
		}
		return tx.Delete(&model.User{}, id).Error
	})
}

func (s *UserService) RegisterUser(username string, password string, inboundService *InboundService) error {
	username, password, _, err := normalizeManagedUserInput(username, password, "user", true)
	if err != nil {
		return err
	}

	hashedPassword, err := crypto.HashPasswordAsBcrypt(password)
	if err != nil {
		return err
	}

	db := database.GetDB()

	// Create user and add as client to all inbounds in a single transaction
	return db.Transaction(func(tx *gorm.DB) error {
		user := &model.User{
			Username: username,
			Password: hashedPassword,
			Role:     "user",
		}
		if err := tx.Create(user).Error; err != nil {
			if isUniqueConstraintError(err) {
				return ErrUsernameAlreadyExists
			}
			return err
		}
		return s.addUserClientsToAllInbounds(tx, username, inboundService)
	})
}

func (s *UserService) UpdateFirstUser(username string, password string) error {
	if username == "" {
		return errors.New("username can not be empty")
	} else if password == "" {
		return errors.New("password can not be empty")
	}
	hashedPassword, er := crypto.HashPasswordAsBcrypt(password)

	if er != nil {
		return er
	}

	db := database.GetDB()
	user := &model.User{}
	err := db.Model(model.User{}).First(user).Error
	if database.IsNotFound(err) {
		user.Username = username
		user.Password = hashedPassword
		user.Role = "admin"
		return db.Model(model.User{}).Create(user).Error
	} else if err != nil {
		return err
	}
	user.Username = username
	user.Password = hashedPassword
	user.Role = "admin"
	return db.Save(user).Error
}
