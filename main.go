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
	"strconv"
	"time"

	"github.com/tidwall/gjson"
)

var (
	// auth
	qbitHostname    = os.Getenv("qbitHostname")
	qbitApiKey      = os.Getenv("qbitApiKey")
	gluetunHostname = os.Getenv("gluetunHostname")
	gluetunApiKey   = os.Getenv("gluetunApiKey")

	// global HTTP client with a cookie jar
	jar, _ = cookiejar.New(nil)
	client = &http.Client{Jar: jar}
)

type PortForward struct {
	Port string `json:"port"`
}

func CheckIsSet(envName string) {
	env := os.Getenv(envName)
	if env != "" {
		logger.Debug(envName + " is set")
	} else {
		logger.Error(envName + " is not set")
		os.Exit(1)
	}
}

// CheckGluetunPort queries the gluetun API for the forwarded port and returns a string with this port.
// If an error is encountered, it is logged and an empty string is returned.
func CheckGluetunPort() (p PortForward, err error) {
	apiPath := gluetunHostname + "/v1/portforward"
	req, err := http.NewRequest("GET", apiPath, nil)
	req.Header.Set("X-API-Key", gluetunApiKey)
	resp, err := client.Do(req)
	if err != nil {
		logger.Debug("invalid request for port forwarded from gluetun")
		return PortForward{}, err
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if len(respBytes) == 0 {
		return PortForward{}, errors.New("received empty response from gluetun when requesting forwarded port")
	}
	logger.Debug("got response from gluetun", "status_code", resp.StatusCode, "response", string(respBytes))

	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		return PortForward{}, errors.New("invalid http status code received from gluetun")
	}

	if err := json.Unmarshal(respBytes, &p); err != nil {
		logger.Error("unable to unmarshal response from gluetun")
		return PortForward{}, err
	}

	port, err := strconv.Atoi(p.Port)
	if err != nil {
		logger.Error("unable to convert port string to integer")
		return PortForward{}, err
	}
	if port == 0 {
		return PortForward{}, errors.New("gluetun has not gotten a port forwarded yet")
	} else if port < 0 || port > 65535 {
		errmsg := fmt.Sprintf("invalid port of `%s` received from gluetun", p.Port)
		return PortForward{}, errors.New(errmsg)
	} else {
		return p, nil
	}
}

func GetOldQbitPort() (oldPort string, err error) {
	apiPath := qbitHostname + "/api/v2/app/preferences"
	req, err := http.NewRequest("GET", apiPath, nil)
	if err != nil {
		logger.Error("failed to create request to get old qbit port")
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+qbitApiKey)
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("failed to send request to get old qbit port")
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	result := gjson.GetBytes(respBody, "listen_port")
	oldPort = result.Str
	return oldPort, nil
}

// TODO
func SetQbitPort(port string) (err error) {
	apiPath := fmt.Sprintf(qbitHostname, "/api/v2/app/setPreferences")

	payload := map[string]string{"listen_port": port}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		logger.Error("error marshaling json to set port")
	}

	// prefix with "json=" per documentation:
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

	logger.Debug("sent request to set port", "status_code", resp.StatusCode, "response", string(respBody))
	return nil
}

func main() {
	InitializeLogging()
	logger.Info("starting up")

	CheckIsSet("qbitHostname")
	CheckIsSet("qbitApiKey")
	CheckIsSet("gluetunHostname")
	CheckIsSet("gluetunApiKey")

	logger.Info("sleeping for 15 seconds to give gluetun time to request port")
	time.Sleep(15 * time.Second)

	// // retry a few times in case qbit hasn't started up yet.
	// // if the program dies and restarts four or five times before it successfully authenticates, it'll clog up the logs
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

	// get app version for debugging purposes
	requestUrl := qbitHostname + "/api/v2/app/version"
	req, err := http.NewRequest("GET", requestUrl, nil)
	req.Header.Set("Authorization", "Bearer "+qbitApiKey)
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("failed to get version from qbit", "status_code", resp.StatusCode)
		logger.Error(err.Error())
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	logger.Info("retrieved qbit API version", "version", string(body))

	pf, err := CheckGluetunPort()
	if err != nil {
		logger.Error("failed to get port forwarded from gluetun. exiting to retry", "error", err.Error())
		os.Exit(1)
	}
	logger.Info("got forwarded port from gluetun", "forwarded_port", pf.Port)

	oldPort, err := GetOldQbitPort()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	if oldPort != pf.Port {
		logger.Info("old qbit port does not match port forwarded from gluetun", "qbit_port", oldPort, "forwarded_port", pf.Port)
		err := SetQbitPort(pf.Port)
		if err != nil {
			logger.Error(err.Error())
			os.Exit(1)
		}
	} else {
		logger.Info("qbit is correctly configured to listen on the port from gluetun")
	}
}
