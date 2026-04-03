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

// UserService provides business logic for user management and authentication.
// It handles user creation, login, password management, and 2FA operations.
type UserService struct {
	settingService SettingService
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

func (s *UserService) RegisterUser(username string, password string, inboundService *InboundService) error {
	if username == "" {
		return errors.New("username can not be empty")
	}
	if password == "" {
		return errors.New("password can not be empty")
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
			errMsg := err.Error()
			if strings.Contains(errMsg, "UNIQUE constraint failed") || strings.Contains(errMsg, "Duplicate") {
				return ErrUsernameAlreadyExists
			}
			return err
		}

		// Add the new user as a disabled client to all existing inbounds
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

			// Build the client JSON entry based on protocol
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

			// Parse inbound settings and append the new client
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

			// Save the updated inbound settings
			if err := tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).Update("settings", inbound.Settings).Error; err != nil {
				return err
			}

			// Create ClientTraffic record for this inbound
			if err := inboundService.AddClientStat(tx, inbound.Id, &client); err != nil {
				return err
			}
		}

		return nil
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
