package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bugsnag/bugsnag-go"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	bugsnagAPIKey, ok := os.LookupEnv("BUGSNAG_API_KEY")
	if !ok {
		log.Fatal("BUGSNAG_API_KEY not set")
	}
	bugsnag.Configure(bugsnag.Configuration{
		APIKey: bugsnagAPIKey,
	})

	domainName, ok := os.LookupEnv("DOMAIN_NAME")
	if !ok {
		log.Fatal("DOMAIN_NAME not set")
	}

	username, ok := os.LookupEnv("OVH_USERNAME")
	if !ok {
		log.Fatal("OVH_USERNAME not set")
	}

	password, ok := os.LookupEnv("OVH_PASSWORD")
	if !ok {
		log.Fatal("OVH_PASSWORD not set")
	}

	ipAddress := ""
	prevIPAddress := ""

	for {
		prevIPAddress = ipAddress

		ipAddress, err = getIPAddress()
		if err != nil {
			bugsnag.Notify(err)
		} else if prevIPAddress != ipAddress {
			setDyndnsIPAddress(ipAddress, domainName, username, password)
		} else {
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
		bugsnag.Notify(err)
	}
	req.SetBasicAuth(username, password)

	resp, err := client.Do(req)
	if err != nil {
		bugsnag.Notify(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		bugsnag.Notify(err)
	}
	log.Println(string(body))
}
