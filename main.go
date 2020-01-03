package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bugsnag/bugsnag-go"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println(".env not provided, using environment variables instead")
	}
	bugsnagAPIKey, ok := os.LookupEnv("BUGSNAG_API_KEY")
	if ok {
		bugsnag.Configure(bugsnag.Configuration{
			APIKey:     bugsnagAPIKey,
			AppVersion: "1.1.2",
		})
	}

	domains, ok := getDomains()
	if !ok {
		log.Fatal("DOMAINS not set")
	}

	username, ok := os.LookupEnv("OVH_USERNAME")
	if !ok {
		log.Fatal("OVH_USERNAME not set")
	}

	password, ok := os.LookupEnv("OVH_PASSWORD")
	if !ok {
		log.Fatal("OVH_PASSWORD not set")
	}

	sleepDuration := envInt("SLEEP_DURATION", 3600)

	ipAddress := ""
	prevIPAddress := ""

	for {
		prevIPAddress = ipAddress

		ipAddress, err := getIPAddress()
		switch {
		case err != nil:
			notify(err)
		case prevIPAddress != ipAddress:
			for _, domainName := range domains {
				fmt.Printf("Settings domain: %s to ip: %s\n", domainName, ipAddress)
				err = setDyndnsIPAddress(ipAddress, domainName, username, password)
				if err != nil {
					notify(err)
				}
			}
		default:
			log.Println("IP address is the same, skipping OVH set")
		}

		time.Sleep(time.Duration(sleepDuration) * time.Second)
	}

}

func getIPAddress() (string, error) {
	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		return "", errors.New("Unable to get IP Address")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Unable to get IP Address, got code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New("Unable to read api.ipify.org body")
	}
	return string(body), nil
}

func setDyndnsIPAddress(ipAddress string, domainName string, username string, password string) error {
	client := &http.Client{}
	url := fmt.Sprintf("https://www.ovh.com/nic/update?system=dyndns&hostname=%s&myip=%s", domainName, ipAddress)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(username, password)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	log.Println(string(body))
	return nil
}

func getDomains() ([]string, bool) {
	domainsEnv, ok := os.LookupEnv("DOMAINS")
	if !ok {
		return nil, false
	}
	domains := strings.Split(domainsEnv, ",")

	return domains, true
}

func notify(err error) {
	_, ok := os.LookupEnv("BUGSNAG_API_KEY")

	if ok {
		bugsnag.Notify(err)
	} else {
		log.Println("Error: ", err)
	}
}

// Gets the environment variable with the specified key and parses as an integer
// if not set then returns the fallback value
func envInt(key string, fallback int) int {
	strValue, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	value, err := strconv.Atoi(strValue)
	if err != nil {
		return fallback
	}
	return value
}
