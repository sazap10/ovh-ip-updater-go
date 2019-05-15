package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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
			AppVersion: "1.0.0",
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

	sleepDuration, ok := os.LookupEnv("SLEEP_DURATION")
	if ok {

	}

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
				setDyndnsIPAddress(ipAddress, domainName, username, password)
			}
		default:
			log.Println("IP address is the same, skipping OVH set")
		}

		time.Sleep(1 * time.Hour)
	}

}

func getIPAddress() (string, error) {
	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		return "", errors.New("Unable to get IP Address")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New("Unable to read api.ipify.org body")
	}
	return string(body), nil
}

func setDyndnsIPAddress(ipAddress string, domainName string, username string, password string) {
	client := &http.Client{}
	url := fmt.Sprintf("https://www.ovh.com/nic/update?system=dyndns&hostname=%s&myip=%s", domainName, ipAddress)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		notify(err)
	}
	req.SetBasicAuth(username, password)

	resp, err := client.Do(req)
	if err != nil {
		notify(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		notify(err)
	}
	log.Println(string(body))
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
