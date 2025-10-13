package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"time"
)

var (
	// auth
	qbitBaseUrl  = os.Getenv("qbitBaseUrl")
	qbitUsername = os.Getenv("qbitUsername")
	qbitPassword = os.Getenv("qbitPassword")

	// global HTTP client with a cookie jar
	jar, _ = cookiejar.New(nil)
	client = &http.Client{Jar: jar}
)

func CheckIsSet(envName string) {
	env := os.Getenv(envName)
	if env != "" {
		logger.Debug(envName + " is set")
	} else {
		logger.Error(envName + " is not set")
		os.Exit(1)
	}
}

// CheckPort queries the gluetun API for the forwarded port and returns a string with this port.
// If an error is encountered, it is logged and an empty string is returned.
func CheckPort() (port string) {
	apiPath := fmt.Sprintf(qbitBaseUrl, "/v1/openvpn/portforwarded")
	resp, err := http.Get(apiPath)
	respBytes, _ := io.ReadAll(resp.Body)
	respString := string(respBytes)
	defer resp.Body.Close()
	if err != nil {
		logger.Error("returned error when requesting forwarded port",
			"status_code", resp.StatusCode, "response", respString, "error", err.Error())
		return ""
	}

	if len(respBytes) == 0 {
		logger.Error("received empty response when requesting forwarded port")
		return ""
	}

	logger.Debug("received response when requesting forwarded port",
		"status_code", resp.StatusCode, "response", respString)

	if respString == "0" {
		logger.Error("gluetun has not received a forwarded port yet, exiting to retry")
		return ""
	}
	// if response is `grep -qE '^[0-9]{1,5}$'`, then return success
	regex := regexp.MustCompile(`^[0-9]{1,5}$`)
	if regex.MatchString(respString) {
		logger.Info("received valid response when requesting forwarded port",
			"status_code", resp.StatusCode, "response", respString)
		return respString // success condition
	}
	logger.Error("invalid response received when requesting forwarded port",
		"status_code", resp.StatusCode, "response", respString)
	return ""
}

// TODO
func SetPort(port string) {
	apiPath := fmt.Sprintf(qbitBaseUrl, "/api/v2/app/setPreferences")

	payload := map[string]string{"listen_port": port}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		logger.Error("error marshaling json to set port")
	}

	// prefix with "json=" per documentation:
	// https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1)#set-application-preferences
	body := append([]byte("json="), jsonBytes...)

	resp, err := http.Post(apiPath, "application/x-www-form-urlencoded", bytes.NewBuffer(body))
	if err != nil {
		logger.Error("error sending post request to set port")
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	logger.Debug("sent request to set port", "status_code", resp.StatusCode, "response", string(respBody))
}

func main() {
	InitializeLogging()
	logger.Info("starting up")

	CheckIsSet("qbitBaseUrl")
	CheckIsSet("qbitUsername")
	CheckIsSet("qbitPassword")

	logger.Info("sleeping for 15 seconds to give gluetun time to request port")
	time.Sleep(15 * time.Second)

	// authenticate and get session cookie
	requestUrl := qbitBaseUrl + "/api/v2/auth/login"
	data := url.Values{"username": {qbitUsername}, "password": {qbitPassword}}

	// retry a few times in case qbit hasn't started up yet.
	// if the program dies and restarts four or five times before it successfully authenticates, it'll clog up the logs
	var resp *http.Response
	var err error
	for i := 1; i <= 5; i++ {
		resp, err = client.PostForm(requestUrl, data)
		if err == nil {
			logger.Debug("successfully authenticated to qbit")
			break
		}
		logger.Debug("qbit authentication request returned an error", "error", err.Error())
		time.Sleep(2 * time.Second) // wait before retrying
	}
	if err != nil { // if can't connect after 5 attempts, exit
		logger.Error("unable to authenticate to qbit", "error", err.Error())
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	authResponse := string(body)
	if resp.StatusCode != http.StatusOK {
		logger.Error("qbit authentication request returned an error", "status_code", resp.StatusCode)
		os.Exit(1)
	}
	if authResponse != "Ok." {
		logger.Error("invalid credentials for qbit", "response", authResponse)
		os.Exit(1)
	}

	// get app version for debugging purposes
	requestUrl = qbitBaseUrl + "/api/v2/app/version"
	resp, err = client.Get(requestUrl)
	if err != nil {
		logger.Error("failed to get version from qbit", "status_code", resp.StatusCode)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ = io.ReadAll(resp.Body)
	logger.Info("retrieved qbit API version", "version", string(body))

	port := CheckPort()
	SetPort(port)
}
