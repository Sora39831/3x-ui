package sub

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v2/config"

	"github.com/gin-gonic/gin"
)

// SUBController handles HTTP requests for subscription links and JSON configurations.
type SUBController struct {
	subTitle         string
	subSupportUrl    string
	subProfileUrl    string
	subAnnounce      string
	subEnableRouting bool
	subRoutingRules  string
	subPath          string
	subJsonPath      string
	jsonEnabled      bool
	subEncrypt       bool
	updateInterval   string

	clashEnabled bool
	subClashPath string

	subService      *SubService
	subJsonService  *SubJsonService
	subClashService *SubClashService
}

// NewSUBController creates a new subscription controller with the given configuration.
func NewSUBController(
	g *gin.RouterGroup,
	subPath string,
	jsonPath string,
	jsonEnabled bool,
	encrypt bool,
	showInfo bool,
	rModel string,
	update string,
	jsonFragment string,
	jsonNoise string,
	jsonMux string,
	jsonRules string,
	subTitle string,
	subSupportUrl string,
	subProfileUrl string,
	subAnnounce string,
	subEnableRouting bool,
	subRoutingRules string,
	clashEnabled bool,
	subClashPath string,
	subClashTemplate string,
	clashServers []ClashServer,
) *SUBController {
	sub := NewSubService(showInfo, rModel)
	a := &SUBController{
		subTitle:         subTitle,
		subSupportUrl:    subSupportUrl,
		subProfileUrl:    subProfileUrl,
		subAnnounce:      subAnnounce,
		subEnableRouting: subEnableRouting,
		subRoutingRules:  subRoutingRules,
		subPath:          subPath,
		subJsonPath:      jsonPath,
		jsonEnabled:      jsonEnabled,
		subEncrypt:       encrypt,
		updateInterval:   update,

		clashEnabled: clashEnabled,
		subClashPath: subClashPath,

		subService:      sub,
		subJsonService:  NewSubJsonService(jsonFragment, jsonNoise, jsonMux, jsonRules, sub),
		subClashService: NewSubClashService(subClashTemplate, clashServers, sub),
	}
	a.initRouter(g)
	return a
}

// initRouter registers HTTP routes for subscription links and JSON endpoints
// on the provided router group.
func (a *SUBController) initRouter(g *gin.RouterGroup) {
	gLink := g.Group(a.subPath)
	gLink.GET(":subid", a.subs)
	if a.jsonEnabled {
		gJson := g.Group(a.subJsonPath)
		gJson.GET(":subid", a.subJsons)
	}
	if a.clashEnabled {
		gClash := g.Group(a.subClashPath)
		gClash.GET(":subid", a.clashSubs)
	}
}

// subs handles HTTP requests for subscription links, returning either HTML page or base64-encoded subscription data.
func (a *SUBController) subs(c *gin.Context) {
	subId := c.Param("subid")
	scheme, host, hostWithPort, hostHeader := a.subService.ResolveRequest(c)
	subs, lastOnline, traffic, err := a.subService.GetSubs(subId, host)
	if err != nil || len(subs) == 0 {
		c.String(400, "Error!")
	} else {
		result := ""
		for _, sub := range subs {
			result += sub + "\n"
		}

		// If the request expects HTML (e.g., browser) or explicitly asked (?html=1 or ?view=html), render the info page here
		accept := c.GetHeader("Accept")
		if strings.Contains(strings.ToLower(accept), "text/html") || c.Query("html") == "1" || strings.EqualFold(c.Query("view"), "html") {
			// Build page data in service
			subURL, subJsonURL := a.subService.BuildURLs(scheme, hostWithPort, a.subPath, a.subJsonPath, subId)
			if !a.jsonEnabled {
				subJsonURL = ""
			}
			subClashURL := ""
			if a.clashEnabled {
				subClashURL = a.subService.buildSingleURL("", scheme, hostWithPort, a.subClashPath, subId)
			}
			// Get base_path from context (set by middleware)
			basePath, exists := c.Get("base_path")
			if !exists {
				basePath = "/"
			}
			// Add subId to base_path for asset URLs
			basePathStr := basePath.(string)
			if basePathStr == "/" {
				basePathStr = "/" + subId + "/"
			} else {
				// Remove trailing slash if exists, add subId, then add trailing slash
				basePathStr = strings.TrimRight(basePathStr, "/") + "/" + subId + "/"
			}
			page := a.subService.BuildPageData(subId, hostHeader, traffic, lastOnline, subs, subURL, subJsonURL, basePathStr)
			c.HTML(200, "subpage.html", gin.H{
				"title":        "subscription.title",
				"cur_ver":      config.GetVersion(),
				"host":         page.Host,
				"base_path":    page.BasePath,
				"sId":          page.SId,
				"download":     page.Download,
				"upload":       page.Upload,
				"total":        page.Total,
				"used":         page.Used,
				"remained":     page.Remained,
				"expire":       page.Expire,
				"lastOnline":   page.LastOnline,
				"datepicker":   page.Datepicker,
				"downloadByte": page.DownloadByte,
				"uploadByte":   page.UploadByte,
				"totalByte":    page.TotalByte,
				"subUrl":       page.SubUrl,
				"subJsonUrl":   page.SubJsonUrl,
				"subClashUrl":  subClashURL,
				"result":       page.Result,
			})
			return
		}

		// Add headers
		header := fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", traffic.Up, traffic.Down, traffic.Total, traffic.ExpiryTime/1000)
		profileUrl := a.subProfileUrl
		if profileUrl == "" {
			profileUrl = fmt.Sprintf("%s://%s%s", scheme, hostWithPort, c.Request.RequestURI)
		}
		a.ApplyCommonHeaders(c, header, a.updateInterval, a.subTitle, a.subSupportUrl, profileUrl, a.subAnnounce, a.subEnableRouting, a.subRoutingRules)

		if a.subEncrypt {
			c.String(200, base64.StdEncoding.EncodeToString([]byte(result)))
		} else {
			c.String(200, result)
		}
	}
}

// subJsons handles HTTP requests for JSON subscription configurations.
func (a *SUBController) subJsons(c *gin.Context) {
	subId := c.Param("subid")
	scheme, host, hostWithPort, _ := a.subService.ResolveRequest(c)
	jsonSub, header, err := a.subJsonService.GetJson(subId, host)
	if err != nil || len(jsonSub) == 0 {
		c.String(400, "Error!")
	} else {
		// Add headers
		profileUrl := a.subProfileUrl
		if profileUrl == "" {
			profileUrl = fmt.Sprintf("%s://%s%s", scheme, hostWithPort, c.Request.RequestURI)
		}
		a.ApplyCommonHeaders(c, header, a.updateInterval, a.subTitle, a.subSupportUrl, profileUrl, a.subAnnounce, a.subEnableRouting, a.subRoutingRules)

		c.String(200, jsonSub)
	}
}

// clashSubs handles HTTP requests for Clash YAML subscription configurations.
func (a *SUBController) clashSubs(c *gin.Context) {
	subId := c.Param("subid")
	scheme, _, hostWithPort, _ := a.subService.ResolveRequest(c)
	clashYaml, header, err := a.subClashService.GetClash(subId)
	if err != nil || len(clashYaml) == 0 {
		c.String(400, "Error!")
	} else {
		// Add headers
		profileUrl := a.subProfileUrl
		if profileUrl == "" {
			profileUrl = fmt.Sprintf("%s://%s%s", scheme, hostWithPort, c.Request.RequestURI)
		}
		a.ApplyCommonHeaders(c, header, a.updateInterval, a.subTitle, a.subSupportUrl, profileUrl, a.subAnnounce, a.subEnableRouting, a.subRoutingRules)

		c.Header("Content-Type", "text/yaml; charset=utf-8")
		c.String(200, clashYaml)
	}
}

// ApplyCommonHeaders sets common HTTP headers for subscription responses including user info, update interval, and profile title.
func (a *SUBController) ApplyCommonHeaders(
	c *gin.Context,
	header,
	updateInterval,
	profileTitle string,
	profileSupportUrl string,
	profileUrl string,
	profileAnnounce string,
	profileEnableRouting bool,
	profileRoutingRules string,
) {
	c.Writer.Header().Set("Subscription-Userinfo", header)
	c.Writer.Header().Set("Profile-Update-Interval", updateInterval)

	//Basics
	if profileTitle != "" {
		c.Writer.Header().Set("Profile-Title", "base64:"+base64.StdEncoding.EncodeToString([]byte(profileTitle)))
	}
	if profileSupportUrl != "" {
		c.Writer.Header().Set("Support-Url", profileSupportUrl)
	}
	if profileUrl != "" {
		c.Writer.Header().Set("Profile-Web-Page-Url", profileUrl)
	}
	if profileAnnounce != "" {
		c.Writer.Header().Set("Announce", "base64:"+base64.StdEncoding.EncodeToString([]byte(profileAnnounce)))
	}

	//Advanced (Happ)
	c.Writer.Header().Set("Routing-Enable", strconv.FormatBool(profileEnableRouting))
	if profileRoutingRules != "" {
		c.Writer.Header().Set("Routing", profileRoutingRules)
	}
}
