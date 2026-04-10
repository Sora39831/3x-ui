package service

import (
	"errors"

	"github.com/mhsanaei/3x-ui/v2/config"
)

var ErrSharedWriteRequiresMaster = errors.New("shared-account writes are only allowed on master nodes")

func IsWorker() bool {
	return config.GetNodeConfigFromJSON().Role == config.NodeRoleWorker
}

func IsMaster() bool {
	return !IsWorker()
}

func RequireMaster() error {
	if IsWorker() {
		return ErrSharedWriteRequiresMaster
	}
	return nil
}

func IsSharedModeEnabled() bool {
	return config.GetDBConfigFromJSON().Type == "mariadb"
}
