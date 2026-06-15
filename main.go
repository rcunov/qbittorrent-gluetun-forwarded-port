package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"
)

var (
	// Auth
	qbitHostname    = os.Getenv("qbitHostname")
	qbitApiKey      = os.Getenv("qbitApiKey")
	gluetunHostname = os.Getenv("gluetunHostname")
	gluetunApiKey   = os.Getenv("gluetunApiKey")

	// Global HTTP client with a cookie jar
	jar, _ = cookiejar.New(nil)
	client = &http.Client{Jar: jar}
)

type GluetunResponse struct {
	Port int `json:"port"`
}

type QbittorrentResponse struct {
	Port int `json:"listen_port"`
}

// Exits if environment variable is not set.
func CheckIsSet(envName string) {
	env := os.Getenv(envName)
	if env != "" {
		logger.Debug(envName + " is set")
	} else {
		logger.Error(envName + " is not set")
		os.Exit(1)
	}
}

// Queries the gluetun API for the forwarded port and returns a string with this port.
// If an error is encountered, it is logged and an empty string is returned.
func CheckGluetunPort() (p GluetunResponse, err error) {
	apiPath := gluetunHostname + "/v1/portforward"
	req, err := http.NewRequest("GET", apiPath, nil)
	req.Header.Set("X-API-Key", gluetunApiKey)
	resp, err := client.Do(req)
	if err != nil {
		logger.Debug("invalid request for port forwarded from gluetun")
		return GluetunResponse{}, err
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if len(respBytes) == 0 {
		return GluetunResponse{}, errors.New("received empty response from gluetun when requesting forwarded port")
	}
	logger.Debug("got response from gluetun", "status_code", resp.StatusCode, "response", string(respBytes))

	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		return GluetunResponse{}, errors.New("invalid http status code received from gluetun")
	}

	if err := json.Unmarshal(respBytes, &p); err != nil {
		logger.Error("unable to unmarshal response from gluetun")
		return GluetunResponse{}, err
	}

	if p.Port == 0 {
		return GluetunResponse{}, errors.New("gluetun has not gotten a port forwarded yet")
	} else if p.Port < 0 || p.Port > 65535 {
		errmsg := fmt.Sprintf("invalid port of `%d` received from gluetun", p.Port)
		return GluetunResponse{}, errors.New(errmsg)
	} else {
		return p, nil
	}
}

// Gets current qBittorrent config from the API and parses it for the "listen_port" value.
func GetOldQbitPort() (p QbittorrentResponse, err error) {
	apiPath := qbitHostname + "/api/v2/app/preferences"
	req, err := http.NewRequest("GET", apiPath, nil)
	if err != nil {
		logger.Error("failed to create request to get old qbit port")
		return QbittorrentResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+qbitApiKey)
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("failed to send request to get old qbit port")
		return QbittorrentResponse{}, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	// logger.Debug("old qbit port response received", "body", string(respBody))
	if err := json.Unmarshal(respBody, &p); err != nil {
		logger.Error("unable to unmarshal response from gluetun")
		return QbittorrentResponse{}, err
	}
	logger.Debug("got old qbit port", "qbit_port", p.Port)
	return p, nil
}

// Updates the qBittorrent config with the new forwarded port.
func SetQbitPort(p int) (err error) {
	apiPath := qbitHostname + "/api/v2/app/setPreferences"

	// port := strconv.Itoa(p)
	payload := map[string]int{"listen_port": p}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		logger.Error("error marshaling json to set port")
	}

	// Prefix with "json=" per documentation:
	// https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1)#set-application-preferences
	body := append([]byte("json="), jsonBytes...)

	req, err := http.NewRequest("POST", apiPath, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+qbitApiKey)
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("error sending post request to set port")
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	logger.Debug("sent request to qbit to set port", "status_code", resp.StatusCode, "response", string(respBody))
	return nil
}

func GetQbitAPIVersion(hostname string) (err error) {
	requestUrl := hostname + "/api/v2/app/version"
	req, err := http.NewRequest("GET", requestUrl, nil)
	req.Header.Set("Authorization", "Bearer "+qbitApiKey)
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("failed to get version from qbit", "status_code", resp.StatusCode)
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	logger.Debug("retrieved qbit API version", "version", string(body))
	return nil
}

func main() {
	InitializeLogging()
	logger.Debug("starting up")

	CheckIsSet("qbitHostname")
	CheckIsSet("qbitApiKey")
	CheckIsSet("gluetunHostname")
	CheckIsSet("gluetunApiKey")

	// TODO: A less dumb way of making sure Gluetun is healthy before continuing
	logger.Info("sleeping for 15 seconds to give gluetun time to request port")
	time.Sleep(15 * time.Second)

	// // Retry a few times in case qbit hasn't started up yet.
	// // If the program dies and restarts four or five times before it successfully authenticates, it'll clog up the logs
	// var resp *http.Response
	// var err error
	// for i := 1; i <= 5; i++ {
	// 	resp, err = client.PostForm(requestUrl, data)
	// 	if err == nil {
	// 		logger.Debug("successfully authenticated to qbit")
	// 		break
	// 	}
	// 	logger.Debug("qbit authentication request returned an error", "error", err.Error())
	// 	time.Sleep(2 * time.Second) // wait before retrying
	// }
	// if err != nil { // if can't connect after 5 attempts, exit
	// 	logger.Error("unable to authenticate to qbit", "error", err.Error())
	// 	os.Exit(1)
	// }
	// defer resp.Body.Close()

	// body, _ := io.ReadAll(resp.Body)
	// authResponse := string(body)
	// if resp.StatusCode > 299 || resp.StatusCode < 200 {
	// 	logger.Error("qbit authentication request returned an error", "status_code", resp.StatusCode, "response", authResponse)
	// 	os.Exit(1)
	// }

	if err := GetQbitAPIVersion(qbitHostname); err != nil {
		logger.Error(err.Error())
	}

	gt, err := CheckGluetunPort()
	if err != nil {
		logger.Error("failed to get port forwarded from gluetun. exiting to retry", "error", err.Error())
		os.Exit(1)
	}
	logger.Debug("got forwarded port from gluetun", "gluetun_port", gt.Port)

	qb, err := GetOldQbitPort()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	if qb.Port != gt.Port {
		logger.Info("old qbit port does not match port forwarded from gluetun, updating qbit config", "qbit_port", qb.Port, "gluetun_port", gt.Port)
		err := SetQbitPort(gt.Port)
		if err != nil {
			logger.Error(err.Error())
			os.Exit(1)
		}
	} else {
		logger.Info("qbit is correctly configured to listen on the port from gluetun", "port", qb.Port)
	}
}
