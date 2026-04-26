// Package main is the entry point for the 3x-ui web panel application.
// It initializes the database, web server, and handles command-line operations for managing the panel.
package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	_ "unsafe"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/sub"
	"github.com/mhsanaei/3x-ui/v2/util/crypto"
	"github.com/mhsanaei/3x-ui/v2/util/sys"
	"github.com/mhsanaei/3x-ui/v2/web"
	"github.com/mhsanaei/3x-ui/v2/web/global"
	"github.com/mhsanaei/3x-ui/v2/web/service"

	"github.com/joho/godotenv"
	"github.com/op/go-logging"
)

type settingCommandOptions struct {
	port                    int
	username                string
	password                string
	webBasePath             string
	webDomain               string
	listenIP                string
	reset                   bool
	show                    bool
	getListen               bool
	getCert                 bool
	resetTwoFactor          bool
	tgbotToken              string
	tgbotChatID             string
	tgbotRuntime            string
	enableTgbot             bool
	dbType                  string
	dbHost                  string
	dbPort                  string
	dbUser                  string
	dbPassword              string
	dbName                  string
	nodeRoleSet             bool
	nodeIDSet               bool
	syncIntervalSet         bool
	trafficFlushIntervalSet bool
	settingStatus           bool
}

func (o settingCommandOptions) needsDBInit() bool {
	return o.port > 0 ||
		o.username != "" ||
		o.password != "" ||
		o.webBasePath != "" ||
		o.webDomain != "" ||
		o.listenIP != "" ||
		o.show ||
		o.getListen ||
		o.getCert ||
		o.settingStatus ||
		o.resetTwoFactor ||
		o.tgbotToken != "" ||
		o.tgbotChatID != "" ||
		o.tgbotRuntime != "" ||
		o.enableTgbot
}

// runWebServer initializes and starts the web server for the 3x-ui panel.
func runWebServer() {
	log.Printf("Starting %v %v", config.GetName(), config.GetVersion())

	dbCfg := config.GetDBConfigFromJSON()
	nodeCfg := config.GetNodeConfigFromJSON()
	if err := config.ValidateNodeConfig(nodeCfg, dbCfg); err != nil {
		log.Fatalf("invalid node configuration: %v", err)
	}

	switch config.GetLogLevel() {
	case config.Debug:
		logger.InitLogger(logging.DEBUG)
	case config.Info:
		logger.InitLogger(logging.INFO)
	case config.Notice:
		logger.InitLogger(logging.NOTICE)
	case config.Warning:
		logger.InitLogger(logging.WARNING)
	case config.Error:
		logger.InitLogger(logging.ERROR)
	default:
		log.Fatalf("Unknown log level: %v", config.GetLogLevel())
	}

	godotenv.Load()

	err := database.InitDB()
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}

	var server *web.Server
	server = web.NewServer()
	global.SetWebServer(server)
	err = server.Start()
	if err != nil {
		log.Fatalf("Error starting web server: %v", err)
		return
	}

	var subServer *sub.Server
	subServer = sub.NewServer()
	global.SetSubServer(subServer)
	err = subServer.Start()
	if err != nil {
		log.Fatalf("Error starting sub server: %v", err)
		return
	}

	sigCh := make(chan os.Signal, 1)
	// Trap shutdown signals
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM, sys.SIGUSR1)
	for {
		sig := <-sigCh

		switch sig {
		case syscall.SIGHUP:
			logger.Info("Received SIGHUP signal. Restarting servers...")

			// --- FIX FOR TELEGRAM BOT CONFLICT (409): Stop bot before restart ---
			service.StopBot()
			// --

			err := server.Stop()
			if err != nil {
				logger.Debug("Error stopping web server:", err)
			}
			err = subServer.Stop()
			if err != nil {
				logger.Debug("Error stopping sub server:", err)
			}

			server = web.NewServer()
			global.SetWebServer(server)
			err = server.Start()
			if err != nil {
				log.Fatalf("Error restarting web server: %v", err)
				return
			}
			log.Println("Web server restarted successfully.")

			subServer = sub.NewServer()
			global.SetSubServer(subServer)
			err = subServer.Start()
			if err != nil {
				log.Fatalf("Error restarting sub server: %v", err)
				return
			}
			log.Println("Sub server restarted successfully.")
		case sys.SIGUSR1:
			logger.Info("Received USR1 signal, restarting xray-core...")
			err := server.RestartXray()
			if err != nil {
				logger.Error("Failed to restart xray-core:", err)
			}

		default:
			// --- FIX FOR TELEGRAM BOT CONFLICT (409) on full shutdown ---
			service.StopBot()
			// ------------------------------------------------------------

			server.Stop()
			subServer.Stop()
			log.Println("Shutting down servers.")
			return
		}
	}
}

// resetSetting resets all panel settings to their default values.
func resetSetting() {
	err := database.InitDB()
	if err != nil {
		fmt.Println("Failed to initialize database:", err)
		return
	}

	settingService := service.SettingService{}
	err = settingService.ResetSettings()
	if err != nil {
		fmt.Println("Failed to reset settings:", err)
	} else {
		fmt.Println("Settings successfully reset.")
	}
}

// showSetting displays the current panel settings if show is true.
func showSetting(show bool) {
	if show {
		settingService := service.SettingService{}
		port, err := settingService.GetPort()
		if err != nil {
			fmt.Println("get current port failed, error info:", err)
		}

		webBasePath, err := settingService.GetBasePath()
		if err != nil {
			fmt.Println("get webBasePath failed, error info:", err)
		}

		webDomain, err := settingService.GetWebDomain()
		if err != nil {
			fmt.Println("get webDomain failed, error info:", err)
		}

		certFile, err := settingService.GetCertFile()
		if err != nil {
			fmt.Println("get cert file failed, error info:", err)
		}
		keyFile, err := settingService.GetKeyFile()
		if err != nil {
			fmt.Println("get key file failed, error info:", err)
		}

		userService := service.UserService{}
		userModel, err := userService.GetFirstUser()
		if err != nil {
			fmt.Println("get current user info failed, error info:", err)
		}

		if userModel.Username == "" || userModel.Password == "" {
			fmt.Println("current username or password is empty")
		}

		fmt.Println("current panel settings as follows:")
		if certFile == "" || keyFile == "" {
			fmt.Println("Warning: Panel is not secure with SSL")
		} else {
			fmt.Println("Panel is secure with SSL")
		}

		hasDefaultCredential := func() bool {
			return userModel.Username == "admin" && crypto.CheckPasswordHash(userModel.Password, "admin")
		}()

		fmt.Println("hasDefaultCredential:", hasDefaultCredential)
		fmt.Println("port:", port)
		fmt.Println("webDomain:", webDomain)
		fmt.Println("webBasePath:", webBasePath)
		nodeCfg := config.GetNodeConfigFromJSON()
		fmt.Println("nodeRole:", nodeCfg.Role)
		fmt.Println("nodeId:", nodeCfg.NodeID)
		fmt.Println("syncInterval:", nodeCfg.SyncIntervalSeconds)
		fmt.Println("trafficFlushInterval:", nodeCfg.TrafficFlushSeconds)
	}
}

// showSettingStatus outputs all settings in a single machine-parseable call.
// This avoids multiple CLI invocations that each re-init the database.
func showSettingStatus() {
	settingService := service.SettingService{}

	port, _ := settingService.GetPort()
	webBasePath, _ := settingService.GetBasePath()
	webDomain, _ := settingService.GetWebDomain()
	certFile, _ := settingService.GetCertFile()
	keyFile, _ := settingService.GetKeyFile()

	userService := service.UserService{}
	userModel, _ := userService.GetFirstUser()

	hasDefaultCredential := userModel.Username == "admin" && crypto.CheckPasswordHash(userModel.Password, "admin")

	nodeCfg := config.GetNodeConfigFromJSON()

	fmt.Printf("port:%d\n", port)
	fmt.Printf("webBasePath:%s\n", webBasePath)
	fmt.Printf("webDomain:%s\n", webDomain)
	fmt.Printf("certFile:%s\n", certFile)
	fmt.Printf("keyFile:%s\n", keyFile)
	fmt.Printf("hasDefaultCredential:%v\n", hasDefaultCredential)
	fmt.Printf("username:%s\n", userModel.Username)
	fmt.Printf("nodeRole:%s\n", nodeCfg.Role)
	fmt.Printf("nodeId:%s\n", nodeCfg.NodeID)
	fmt.Printf("syncInterval:%d\n", nodeCfg.SyncIntervalSeconds)
	fmt.Printf("trafficFlushInterval:%d\n", nodeCfg.TrafficFlushSeconds)
}

// updateTgbotEnableSts enables or disables the Telegram bot notifications based on the status parameter.
func updateTgbotEnableSts(status bool) {
	settingService := service.SettingService{}
	currentTgSts, err := settingService.GetTgbotEnabled()
	if err != nil {
		fmt.Println(err)
		return
	}
	logger.Infof("current enabletgbot status[%v],need update to status[%v]", currentTgSts, status)
	if currentTgSts != status {
		err := settingService.SetTgbotEnabled(status)
		if err != nil {
			fmt.Println(err)
			return
		} else {
			logger.Infof("SetTgbotEnabled[%v] success", status)
		}
	}
}

// updateTgbotSetting updates Telegram bot settings including token, chat ID, and runtime schedule.
func updateTgbotSetting(tgBotToken string, tgBotChatid string, tgBotRuntime string) {
	settingService := service.SettingService{}

	if tgBotToken != "" {
		err := settingService.SetTgBotToken(tgBotToken)
		if err != nil {
			fmt.Printf("Error setting Telegram bot token: %v\n", err)
			return
		}
		logger.Info("Successfully updated Telegram bot token.")
	}

	if tgBotRuntime != "" {
		err := settingService.SetTgbotRuntime(tgBotRuntime)
		if err != nil {
			fmt.Printf("Error setting Telegram bot runtime: %v\n", err)
			return
		}
		logger.Infof("Successfully updated Telegram bot runtime to [%s].", tgBotRuntime)
	}

	if tgBotChatid != "" {
		err := settingService.SetTgBotChatId(tgBotChatid)
		if err != nil {
			fmt.Printf("Error setting Telegram bot chat ID: %v\n", err)
			return
		}
		logger.Info("Successfully updated Telegram bot chat ID.")
	}
}

// updateSetting updates various panel settings including port, domain, credentials, base path, listen IP, and two-factor authentication.
func updateSetting(port int, username string, password string, webBasePath string, webDomain string, listenIP string, resetTwoFactor bool) {
	settingService := service.SettingService{}
	userService := service.UserService{}

	if port > 0 {
		err := settingService.SetPort(port)
		if err != nil {
			fmt.Println("Failed to set port:", err)
		} else {
			fmt.Printf("Port set successfully: %v\n", port)
		}
	}

	if username != "" || password != "" {
		err := userService.UpdateFirstUser(username, password)
		if err != nil {
			fmt.Println("Failed to update username and password:", err)
		} else {
			fmt.Println("Username and password updated successfully")
		}
	}

	if webBasePath != "" {
		err := settingService.SetBasePath(webBasePath)
		if err != nil {
			fmt.Println("Failed to set base URI path:", err)
		} else {
			fmt.Println("Base URI path set successfully")
		}
	}

	if webDomain != "" {
		err := settingService.SetWebDomain(webDomain)
		if err != nil {
			fmt.Println("Failed to set web domain:", err)
		} else {
			fmt.Printf("Web domain set successfully: %v\n", webDomain)
		}
	}

	if resetTwoFactor {
		err := settingService.SetTwoFactorEnable(false)

		if err != nil {
			fmt.Println("Failed to reset two-factor authentication:", err)
		} else {
			settingService.SetTwoFactorToken("")
			fmt.Println("Two-factor authentication reset successfully")
		}
	}

	if listenIP != "" {
		err := settingService.SetListen(listenIP)
		if err != nil {
			fmt.Println("Failed to set listen IP:", err)
		} else {
			fmt.Printf("listen %v set successfully", listenIP)
		}
	}
}

// updateCert updates the SSL certificate files for the panel.
func updateCert(publicKey string, privateKey string) {
	err := database.InitDB()
	if err != nil {
		fmt.Println(err)
		return
	}

	if (privateKey != "" && publicKey != "") || (privateKey == "" && publicKey == "") {
		settingService := service.SettingService{}
		err = settingService.SetCertFile(publicKey)
		if err != nil {
			fmt.Println("set certificate public key failed:", err)
		} else {
			fmt.Println("set certificate public key success")
		}

		err = settingService.SetKeyFile(privateKey)
		if err != nil {
			fmt.Println("set certificate private key failed:", err)
		} else {
			fmt.Println("set certificate private key success")
		}

		err = settingService.SetSubCertFile(publicKey)
		if err != nil {
			fmt.Println("set certificate for subscription public key failed:", err)
		} else {
			fmt.Println("set certificate for subscription public key success")
		}

		err = settingService.SetSubKeyFile(privateKey)
		if err != nil {
			fmt.Println("set certificate for subscription private key failed:", err)
		} else {
			fmt.Println("set certificate for subscription private key success")
		}
	} else {
		fmt.Println("both public and private key should be entered.")
	}
}

// GetCertificate displays the current SSL certificate settings if getCert is true.
func GetCertificate(getCert bool) {
	if getCert {
		settingService := service.SettingService{}
		certFile, err := settingService.GetCertFile()
		if err != nil {
			fmt.Println("get cert file failed, error info:", err)
		}
		keyFile, err := settingService.GetKeyFile()
		if err != nil {
			fmt.Println("get key file failed, error info:", err)
		}

		fmt.Println("cert:", certFile)
		fmt.Println("key:", keyFile)
	}
}

// GetListenIP displays the current panel listen IP address if getListen is true.
func GetListenIP(getListen bool) {
	if getListen {

		settingService := service.SettingService{}
		ListenIP, err := settingService.GetListen()
		if err != nil {
			log.Printf("Failed to retrieve listen IP: %v", err)
			return
		}

		fmt.Println("listenIP:", ListenIP)
	}
}

// migrateDb performs database migration operations for the 3x-ui panel.
func migrateDb() {
	inboundService := service.InboundService{}
	switch config.GetLogLevel() {
	case config.Debug:
		logger.InitLogger(logging.DEBUG)
	case config.Info:
		logger.InitLogger(logging.INFO)
	case config.Notice:
		logger.InitLogger(logging.NOTICE)
	case config.Warning:
		logger.InitLogger(logging.WARNING)
	case config.Error:
		logger.InitLogger(logging.ERROR)
	default:
		logger.InitLogger(logging.INFO)
	}

	err := database.InitDB()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Start migrating database...")
	if err := inboundService.MigrateDB(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Migration done!")
}

// migrateDbBetweenDrivers migrates data between SQLite and MariaDB.
// The direction can be specified via --direction flag, otherwise it falls back to dbType from config.
func migrateDbBetweenDrivers(direction string) {
	switch direction {
	case "sqlite-to-mariadb":
		fmt.Println("Migrating data from SQLite to MariaDB...")
		if err := database.MigrateSQLiteToMariaDB(); err != nil {
			log.Fatal("Migration failed: ", err)
		}
		fmt.Println("Migration to MariaDB completed successfully.")
	case "mariadb-to-sqlite":
		fmt.Println("Migrating data from MariaDB to SQLite...")
		if err := database.MigrateMariaDBToSQLite(); err != nil {
			log.Fatal("Migration failed: ", err)
		}
		fmt.Println("Migration to SQLite completed successfully.")
	default:
		// Fall back to inferring from dbType config
		dbType := config.GetDBTypeFromJSON()
		switch dbType {
		case "mariadb":
			fmt.Println("Migrating data from SQLite to MariaDB...")
			if err := database.MigrateSQLiteToMariaDB(); err != nil {
				log.Fatal("Migration failed: ", err)
			}
			fmt.Println("Migration to MariaDB completed successfully.")
		case "sqlite":
			fmt.Println("Migrating data from MariaDB to SQLite...")
			if err := database.MigrateMariaDBToSQLite(); err != nil {
				log.Fatal("Migration failed: ", err)
			}
			fmt.Println("Migration to SQLite completed successfully.")
		default:
			log.Fatalf("Unknown dbType: %s", dbType)
		}
	}
}

// main is the entry point of the 3x-ui application.
// It parses command-line arguments to run the web server, migrate database, or update settings.
func main() {
	if len(os.Args) < 2 {
		runWebServer()
		return
	}

	var showVersion bool
	flag.BoolVar(&showVersion, "v", false, "show version")

	runCmd := flag.NewFlagSet("run", flag.ExitOnError)

	settingCmd := flag.NewFlagSet("setting", flag.ExitOnError)
	var port int
	var username string
	var password string
	var webBasePath string
	var webDomain string
	var listenIP string
	var getListen bool
	var webCertFile string
	var webKeyFile string
	var tgbottoken string
	var tgbotchatid string
	var enabletgbot bool
	var tgbotRuntime string
	var reset bool
	var show bool
	var getCert bool
	var resetTwoFactor bool
	var settingStatus bool
	settingCmd.BoolVar(&reset, "reset", false, "Reset all settings")
	settingCmd.BoolVar(&show, "show", false, "Display current settings")
	settingCmd.BoolVar(&settingStatus, "settingStatus", false, "Display all settings and cert info in one call")
	settingCmd.IntVar(&port, "port", 0, "Set panel port number")
	settingCmd.StringVar(&username, "username", "", "Set login username")
	settingCmd.StringVar(&password, "password", "", "Set login password")
	settingCmd.StringVar(&webBasePath, "webBasePath", "", "Set base path for Panel")
	settingCmd.StringVar(&webDomain, "webDomain", "", "Set panel domain")
	settingCmd.StringVar(&listenIP, "listenIP", "", "set panel listenIP IP")
	settingCmd.BoolVar(&resetTwoFactor, "resetTwoFactor", false, "Reset two-factor authentication settings")
	settingCmd.BoolVar(&getListen, "getListen", false, "Display current panel listenIP IP")
	settingCmd.BoolVar(&getCert, "getCert", false, "Display current certificate settings")
	settingCmd.StringVar(&webCertFile, "webCert", "", "Set path to public key file for panel")
	settingCmd.StringVar(&webKeyFile, "webCertKey", "", "Set path to private key file for panel")
	settingCmd.StringVar(&tgbottoken, "tgbottoken", "", "Set token for Telegram bot")
	settingCmd.StringVar(&tgbotRuntime, "tgbotRuntime", "", "Set cron time for Telegram bot notifications")
	settingCmd.StringVar(&tgbotchatid, "tgbotchatid", "", "Set chat ID for Telegram bot notifications")
	settingCmd.BoolVar(&enabletgbot, "enabletgbot", false, "Enable notifications via Telegram bot")
	var dbTypeFlag string
	var dbHost string
	var dbPort string
	var dbUser string
	var dbPassword string
	var dbName string
	var showDbType bool
	var nodeRoleFlag string
	var nodeIDFlag string
	var syncIntervalFlag int
	var trafficFlushIntervalFlag int
	settingCmd.StringVar(&dbTypeFlag, "dbType", "", "Set database type (sqlite or mariadb)")
	settingCmd.StringVar(&dbHost, "dbHost", "", "Set MariaDB host")
	settingCmd.StringVar(&dbPort, "dbPort", "", "Set MariaDB port")
	settingCmd.StringVar(&dbUser, "dbUser", "", "Set MariaDB username")
	settingCmd.StringVar(&dbPassword, "dbPassword", "", "Set MariaDB password")
	settingCmd.StringVar(&dbName, "dbName", "", "Set MariaDB database name")
	settingCmd.BoolVar(&showDbType, "showDbType", false, "Print current database type and exit")
	settingCmd.StringVar(&nodeRoleFlag, "nodeRole", "", "Set node role (master or worker)")
	settingCmd.StringVar(&nodeIDFlag, "nodeId", "", "Set node identifier")
	settingCmd.IntVar(&syncIntervalFlag, "syncInterval", 0, "Set shared sync interval in seconds")
	settingCmd.IntVar(&trafficFlushIntervalFlag, "trafficFlushInterval", 0, "Set traffic flush interval in seconds")

	migrateDbCmd := flag.NewFlagSet("migrate-db", flag.ExitOnError)
	var migrateDirection string
	migrateDbCmd.StringVar(&migrateDirection, "direction", "", "Migration direction: sqlite-to-mariadb or mariadb-to-sqlite")

	backupCmd := flag.NewFlagSet("backup", flag.ExitOnError)

	restoreCmd := flag.NewFlagSet("restore", flag.ExitOnError)
	var restoreFile string
	restoreCmd.StringVar(&restoreFile, "file", "", "Backup file name to restore from")

	// Allow dbPassword to be passed via env var to avoid leaking it in process args
	if p := os.Getenv("XUI_DB_PASSWORD"); p != "" {
		dbPassword = p
	}

	oldUsage := flag.Usage
	flag.Usage = func() {
		oldUsage()
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("    run            run web panel")
		fmt.Println("    migrate        migrate form other/old x-ui")
		fmt.Println("    migrate-db     migrate data between SQLite and MariaDB")
		fmt.Println("    backup         create a database backup")
		fmt.Println("    restore        restore database from backup")
		fmt.Println("    setting        set settings")
	}

	flag.Parse()
	if showVersion {
		fmt.Println(config.GetVersion())
		return
	}

	switch os.Args[1] {
	case "run":
		err := runCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		runWebServer()
	case "migrate":
		migrateDb()
	case "migrate-db":
		err := migrateDbCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		migrateDbBetweenDrivers(migrateDirection)
	case "setting":
		err := settingCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		nodeRoleSet := false
		nodeIDSet := false
		syncIntervalSet := false
		trafficFlushIntervalSet := false
		settingCmd.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "nodeRole":
				nodeRoleSet = true
			case "nodeId":
				nodeIDSet = true
			case "syncInterval":
				syncIntervalSet = true
			case "trafficFlushInterval":
				trafficFlushIntervalSet = true
			}
		})
		if showDbType {
			fmt.Println(config.GetDBTypeFromJSON())
			return
		}
		if dbTypeFlag != "" {
			if err := config.WriteSettingToJSON("dbType", dbTypeFlag); err != nil {
				fmt.Println("Failed to set dbType:", err)
			} else {
				fmt.Println("dbType set to:", dbTypeFlag)
			}
		}
		if dbHost != "" {
			if err := config.WriteSettingToJSON("dbHost", dbHost); err != nil {
				fmt.Println("Failed to set dbHost:", err)
			} else {
				fmt.Println("dbHost set to:", dbHost)
			}
		}
		if dbPort != "" {
			if err := config.WriteSettingToJSON("dbPort", dbPort); err != nil {
				fmt.Println("Failed to set dbPort:", err)
			} else {
				fmt.Println("dbPort set to:", dbPort)
			}
		}
		if dbUser != "" {
			if err := config.WriteSettingToJSON("dbUser", dbUser); err != nil {
				fmt.Println("Failed to set dbUser:", err)
			} else {
				fmt.Println("dbUser set to:", dbUser)
			}
		}
		if dbPassword != "" {
			if err := config.WriteSettingToJSON("dbPassword", dbPassword); err != nil {
				fmt.Println("Failed to set dbPassword:", err)
			} else {
				fmt.Println("dbPassword set")
			}
		}
		if dbName != "" {
			if err := config.WriteSettingToJSON("dbName", dbName); err != nil {
				fmt.Println("Failed to set dbName:", err)
			} else {
				fmt.Println("dbName set to:", dbName)
			}
		}
		if nodeRoleSet || nodeIDSet || syncIntervalSet || trafficFlushIntervalSet {
			candidate := config.GetNodeConfigFromJSON()
			if nodeRoleSet {
				candidate.Role = config.NodeRole(nodeRoleFlag)
			}
			if nodeIDSet {
				candidate.NodeID = nodeIDFlag
			}
			if syncIntervalSet {
				candidate.SyncIntervalSeconds = syncIntervalFlag
			}
			if trafficFlushIntervalSet {
				candidate.TrafficFlushSeconds = trafficFlushIntervalFlag
			}
			if err := config.ValidateNodeConfig(candidate, config.GetDBConfigFromJSON()); err != nil {
				fmt.Println("Invalid node settings:", err)
				return
			}
			if nodeRoleSet {
				if err := config.WriteSettingToJSON("nodeRole", nodeRoleFlag); err != nil {
					fmt.Println("Failed to set nodeRole:", err)
				} else {
					fmt.Println("nodeRole set to:", nodeRoleFlag)
				}
			}
			if nodeIDSet {
				if err := config.WriteSettingToJSON("nodeId", nodeIDFlag); err != nil {
					fmt.Println("Failed to set nodeId:", err)
				} else {
					fmt.Println("nodeId set to:", nodeIDFlag)
				}
			}
			if syncIntervalSet {
				if err := config.WriteSettingToJSON("syncInterval", fmt.Sprintf("%d", syncIntervalFlag)); err != nil {
					fmt.Println("Failed to set syncInterval:", err)
				} else {
					fmt.Println("syncInterval set to:", syncIntervalFlag)
				}
			}
			if trafficFlushIntervalSet {
				if err := config.WriteSettingToJSON("trafficFlushInterval", fmt.Sprintf("%d", trafficFlushIntervalFlag)); err != nil {
					fmt.Println("Failed to set trafficFlushInterval:", err)
				} else {
					fmt.Println("trafficFlushInterval set to:", trafficFlushIntervalFlag)
				}
			}
		}
		opts := settingCommandOptions{
			port:                    port,
			username:                username,
			password:                password,
			webBasePath:             webBasePath,
			webDomain:               webDomain,
			listenIP:                listenIP,
			reset:                   reset,
			show:                    show,
			getListen:               getListen,
			getCert:                 getCert,
			resetTwoFactor:          resetTwoFactor,
			tgbotToken:              tgbottoken,
			tgbotChatID:             tgbotchatid,
			tgbotRuntime:            tgbotRuntime,
			enableTgbot:             enabletgbot,
			dbType:                  dbTypeFlag,
			dbHost:                  dbHost,
			dbPort:                  dbPort,
			dbUser:                  dbUser,
			dbPassword:              dbPassword,
			dbName:                  dbName,
			nodeRoleSet:             nodeRoleSet,
			nodeIDSet:               nodeIDSet,
			syncIntervalSet:         syncIntervalSet,
			trafficFlushIntervalSet: trafficFlushIntervalSet,
			settingStatus:           settingStatus,
		}
		if opts.needsDBInit() {
			if err := database.InitDB(); err != nil {
				fmt.Println("Database initialization failed:", err)
				return
			}
		}
		if settingStatus {
			showSettingStatus()
			return
		}
		if reset {
			resetSetting()
		} else {
			updateSetting(port, username, password, webBasePath, webDomain, listenIP, resetTwoFactor)
		}
		if show {
			showSetting(show)
		}
		if getListen {
			GetListenIP(getListen)
		}
		if getCert {
			GetCertificate(getCert)
		}
		if (tgbottoken != "") || (tgbotchatid != "") || (tgbotRuntime != "") {
			updateTgbotSetting(tgbottoken, tgbotchatid, tgbotRuntime)
		}
		if enabletgbot {
			updateTgbotEnableSts(enabletgbot)
		}
	case "cert":
		err := settingCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		if reset {
			updateCert("", "")
		} else {
			updateCert(webCertFile, webKeyFile)
		}
	case "backup":
		err := backupCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		runBackup()
	case "restore":
		err := restoreCmd.Parse(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			return
		}
		if restoreFile == "" {
			fmt.Println("--file flag is required")
			return
		}
		runRestore(restoreFile)
	default:
		fmt.Println("Invalid subcommands")
		fmt.Println()
		runCmd.Usage()
		fmt.Println()
		settingCmd.Usage()
	}
}

func runBackup() {
	backupDir := "/etc/x-ui/backups"
	os.MkdirAll(backupDir, 0755)

	dbCfg := config.GetDBConfigFromJSON()
	if dbCfg.Type == "" {
		dbCfg.Type = "sqlite"
	}

	timestamp := time.Now().Format("2006-01-02-150405")
	filename := fmt.Sprintf("backup-%s.tar.gz", timestamp)
	filePath := filepath.Join(backupDir, filename)

	var dumpSQL string
	var err error

	switch dbCfg.Type {
	case "mariadb":
		dumpSQL, err = dumpMariaDBCLI(dbCfg)
	case "sqlite":
		dumpSQL, err = dumpSQLiteCLI(config.GetDBPath())
	default:
		fmt.Println("unsupported database type:", dbCfg.Type)
		os.Exit(1)
	}
	if err != nil {
		fmt.Println("dump failed:", err)
		os.Exit(1)
	}

	meta := map[string]string{
		"dbType":    dbCfg.Type,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   config.GetVersion(),
	}

	if err := createTarGzCLI(filePath, meta, dumpSQL); err != nil {
		fmt.Println("archive creation failed:", err)
		os.Exit(1)
	}

	fmt.Println("backup created:", filePath)
}

func runRestore(filename string) {
	nodeCfg := config.GetNodeConfigFromJSON()
	if nodeCfg.Role == config.NodeRoleWorker {
		fmt.Println("backup and restore can only be performed on the master node")
		os.Exit(1)
	}

	backupDir := "/etc/x-ui/backups"
	filePath := filepath.Join(backupDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Println("backup file not found:", filePath)
		os.Exit(1)
	}

	f, err := os.Open(filePath)
	if err != nil {
		fmt.Println("cannot open backup:", err)
		os.Exit(1)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		fmt.Println("invalid backup file:", err)
		os.Exit(1)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	meta := make(map[string]string)
	var dumpSQL strings.Builder

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("invalid backup:", err)
			os.Exit(1)
		}
		var itemBuf strings.Builder
		if _, err := io.Copy(&itemBuf, tr); err != nil {
			fmt.Println("read error:", err)
			os.Exit(1)
		}
		switch hdr.Name {
		case "metadata.json":
			json.Unmarshal([]byte(itemBuf.String()), &meta)
		case "dump.sql":
			dumpSQL.WriteString(itemBuf.String())
		}
	}

	currentDBType := config.GetDBConfigFromJSON().Type
	if currentDBType == "" {
		currentDBType = "sqlite"
	}
	if meta["dbType"] != currentDBType {
		fmt.Printf("backup type (%s) does not match current database (%s)\n", meta["dbType"], currentDBType)
		os.Exit(1)
	}

	if dumpSQL.Len() == 0 {
		fmt.Println("dump.sql not found in backup")
		os.Exit(1)
	}

	// Create safety backup
	safetyTimestamp := time.Now().Format("2006-01-02-150405")
	safetyFile := filepath.Join(backupDir, "pre-restore-"+safetyTimestamp+".tar.gz")
	var safetySQL string
	var safetyErr error
	switch currentDBType {
	case "mariadb":
		safetySQL, safetyErr = dumpMariaDBCLI(config.GetDBConfigFromJSON())
	default:
		safetySQL, safetyErr = dumpSQLiteCLI(config.GetDBPath())
	}
	if safetyErr == nil {
		safetyMeta := map[string]string{
			"dbType":    currentDBType,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   config.GetVersion(),
		}
		if err := createTarGzCLI(safetyFile, safetyMeta, safetySQL); err == nil {
			fmt.Println("safety backup created:", safetyFile)
		}
	}

	// Restore
	switch currentDBType {
	case "mariadb":
		dbCfg := config.GetDBConfigFromJSON()
		args := []string{
			fmt.Sprintf("-h%s", dbCfg.Host), fmt.Sprintf("-P%s", dbCfg.Port),
		}
		if dbCfg.User != "" {
			args = append(args, fmt.Sprintf("-u%s", dbCfg.User))
		}
		if dbCfg.Password != "" {
			args = append(args, fmt.Sprintf("-p%s", dbCfg.Password))
		}
		args = append(args, dbCfg.Name)
		cmd := exec.Command("mysql", args...)
		cmd.Stdin = strings.NewReader(dumpSQL.String())
		var stderr strings.Builder
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			fmt.Println("restore failed:", err, stderr.String())
			os.Exit(1)
		}
	default:
		cmd := exec.Command("sqlite3", config.GetDBPath())
		cmd.Stdin = strings.NewReader(dumpSQL.String())
		var stderr strings.Builder
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			fmt.Println("restore failed:", err, stderr.String())
			os.Exit(1)
		}
	}

	fmt.Println("restore completed successfully")
}

func dumpMariaDBCLI(dbCfg config.DBConfig) (string, error) {
	args := []string{
		"--single-transaction", "--routines", "--triggers", "--no-tablespaces",
		fmt.Sprintf("-h%s", dbCfg.Host), fmt.Sprintf("-P%s", dbCfg.Port),
	}
	if dbCfg.User != "" {
		args = append(args, fmt.Sprintf("-u%s", dbCfg.User))
	}
	if dbCfg.Password != "" {
		args = append(args, fmt.Sprintf("-p%s", dbCfg.Password))
	}
	args = append(args, dbCfg.Name)
	cmd := exec.Command("mysqldump", args...)
	var out, stderr strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}
	return out.String(), nil
}

func dumpSQLiteCLI(dbPath string) (string, error) {
	cmd := exec.Command("sqlite3", dbPath, ".dump")
	var out, stderr strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}
	return out.String(), nil
}

func createTarGzCLI(filePath string, meta map[string]string, dumpSQL string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{Name: "metadata.json", Size: int64(len(metaBytes)), Mode: 0644, Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	if _, err := tw.Write(metaBytes); err != nil {
		return err
	}

	dumpBytes := []byte(dumpSQL)
	if err := tw.WriteHeader(&tar.Header{Name: "dump.sql", Size: int64(len(dumpBytes)), Mode: 0644, Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	if _, err := tw.Write(dumpBytes); err != nil {
		return err
	}
	return nil
}
