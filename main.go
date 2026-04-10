// Package main is the entry point for the 3x-ui web panel application.
// It initializes the database, web server, and handles command-line operations for managing the panel.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
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
	port                 int
	username             string
	password             string
	webBasePath          string
	webDomain            string
	listenIP             string
	reset                bool
	show                 bool
	getListen            bool
	getCert              bool
	resetTwoFactor       bool
	tgbotToken           string
	tgbotChatID          string
	tgbotRuntime         string
	enableTgbot          bool
	dbType               string
	dbHost               string
	dbPort               string
	dbUser               string
	dbPassword           string
	dbName               string
	nodeRoleSet          bool
	nodeIDSet            bool
	syncIntervalSet      bool
	trafficFlushIntervalSet bool
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
	settingCmd.BoolVar(&reset, "reset", false, "Reset all settings")
	settingCmd.BoolVar(&show, "show", false, "Display current settings")
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
		}
		if opts.needsDBInit() {
			if err := database.InitDB(); err != nil {
				fmt.Println("Database initialization failed:", err)
				return
			}
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
	default:
		fmt.Println("Invalid subcommands")
		fmt.Println()
		runCmd.Usage()
		fmt.Println()
		settingCmd.Usage()
	}
}
