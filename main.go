package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bugsnag/bugsnag-go/v2"
	"github.com/cenkalti/backoff/v5"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
)

type DynDNSRequest struct {
	IPAddress  string
	DomainName string
	Username   string
	Password   string
}

func main() {
	ctx := context.Background()
	err := godotenv.Load()
	if err != nil {
		log.Println(".env not provided, using environment variables instead")
	}
	bugsnagAPIKey, ok := os.LookupEnv("BUGSNAG_API_KEY")
	if ok {
		bugsnag.Configure(bugsnag.Configuration{
			APIKey:     bugsnagAPIKey,
			AppVersion: "v1.4.1",
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

	client := &http.Client{
		Timeout: time.Second * 30,
	}

	ipAddress := ""
	var prevIPAddress string

	for {
		prevIPAddress = ipAddress

		ipAddress, err := getIPAddressWithRetry(ctx, client)
		switch {
		case err != nil:
			notify(err)
		case prevIPAddress != ipAddress:
			for _, domainName := range domains {
				log.Printf("Settings domain: %s to ip: %s\n", domainName, ipAddress)
				requestArgs := DynDNSRequest{
					IPAddress:  ipAddress,
					DomainName: domainName,
					Username:   username,
					Password:   password,
				}
				err = setDyndnsIPAddressWithRetry(ctx, client, requestArgs)
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

func getIPAddressWithRetry(ctx context.Context, client *http.Client) (string, error) {
	operation := 	func() (string, error) {
		return getIPAddress(ctx, client)
	}
	notify := func(err error, d time.Duration) {
		log.Printf("Problem getting IP address: %s, retrying in %s\n", err, d)
	}
	ip, err := backoff.Retry(
		ctx,
		operation,
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithNotify(notify),
	)
	if err != nil {
		return "", errors.Wrapf(err, "Unable to get IP address, retries exhausted")
	}
	return ip, nil
}

func getIPAddress(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.ipify.org", nil)
	if err != nil {
		return "", errors.Wrap(err, "Unable to create request to api.ipify.org")
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "Unable to get IP address")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Unable to get IP address, got code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "Unable to read api.ipify.org body")
	}
	return string(body), nil
}

func setDyndnsIPAddressWithRetry(ctx context.Context, client *http.Client, r DynDNSRequest) error {
	operation := 	func() (string, error) {
		return setDyndnsIPAddress(ctx, client, r)
	}
	notify := func(err error, d time.Duration) {
		log.Printf("Unable to set dyndns ip for domain %s: %s, retrying in %s\n", r.DomainName, err, d)
	}

	result, err := backoff.Retry(
		ctx,
		operation,
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithNotify(notify),
	)
	if err != nil {
		return errors.Wrapf(err, "Unable to set dyndns ip for domain %s, retries exhausted", r.DomainName)
	}
	log.Printf("Set dyndns ip for domain %s: %s\n", r.DomainName, result)
	return nil
}

func setDyndnsIPAddress(ctx context.Context, client *http.Client, r DynDNSRequest) (string, error) {
	url := fmt.Sprintf("https://www.ovh.com/nic/update?system=dyndns&hostname=%s&myip=%s", r.DomainName, r.IPAddress)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", errors.Wrapf(err, "Unable to create request to set IP Address for domain: %s", r.DomainName)
	}
	req.SetBasicAuth(r.Username, r.Password)

	resp, err := client.Do(req)
	if err != nil {
		return "", errors.Wrapf(err, "Unable to set IP Address for domain: %s", r.DomainName)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Unable to set IP Address for domain, got code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrapf(err, "Unable to read response body for domain: %s", r.DomainName)
	}
	return string(body), nil
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
