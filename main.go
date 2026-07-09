package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bugsnag/bugsnag-go/v2"
	"github.com/cenkalti/backoff/v7"
	"github.com/joho/godotenv"
)

// Default endpoints. The retry helpers take the URL as a parameter so tests can
// point them at a local server.
const (
	defaultIPAddressURL = "https://api.ipify.org"
	defaultOVHUpdateURL = "https://www.ovh.com/nic/update"
)

type dynDNSRequest struct {
	ipAddress  string
	domainName string
	username   string
	password   string
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println(".env not provided, using environment variables instead")
	}
	bugsnagAPIKey, ok := os.LookupEnv("BUGSNAG_API_KEY")
	if ok {
		bugsnag.Configure(bugsnag.Configuration{
			APIKey:     bugsnagAPIKey,
			AppVersion: "v1.4.6", // x-release-please-version
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client := &http.Client{
		Timeout: time.Second * 30,
	}

	ticker := time.NewTicker(time.Duration(sleepDuration) * time.Second)
	defer ticker.Stop()

	var prevIPAddress string

	for {
		prevIPAddress = updateDomains(ctx, client, domains, username, password, prevIPAddress)

		select {
		case <-ctx.Done():
			log.Println("Shutting down")
			return
		case <-ticker.C:
		}
	}
}

// updateDomains fetches the current public IP and, if it has changed since
// prevIPAddress, pushes it to each domain's OVH DynDNS record. It returns the IP
// to use as the previous value on the next cycle.
func updateDomains(ctx context.Context, client *http.Client, domains []string, username, password, prevIPAddress string) string {
	ipAddress, err := getIPAddressWithRetry(ctx, client, defaultIPAddressURL)
	switch {
	case err != nil:
		notify(err)
		return prevIPAddress
	case prevIPAddress != ipAddress:
		for _, domainName := range domains {
			log.Printf("Settings domain: %s to ip: %s\n", domainName, ipAddress)
			requestArgs := dynDNSRequest{
				ipAddress:  ipAddress,
				domainName: domainName,
				username:   username,
				password:   password,
			}
			if err := setDyndnsIPAddressWithRetry(ctx, client, defaultOVHUpdateURL, requestArgs); err != nil {
				notify(err)
			}
		}
	default:
		log.Println("IP address is the same, skipping OVH set")
	}

	return ipAddress
}

func getIPAddressWithRetry(ctx context.Context, client *http.Client, url string, opts ...backoff.RetryOption) (string, error) {
	operation := func() (string, error) {
		return getIPAddress(ctx, client, url)
	}
	notify := func(err error, d time.Duration) {
		log.Printf("Problem getting IP address: %s, retrying in %s\n", err, d)
	}
	retryOpts := append([]backoff.RetryOption{
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithNotify(notify),
	}, opts...)
	ip, err := backoff.Retry(ctx, operation, retryOpts...)
	if err != nil {
		return "", fmt.Errorf("Unable to get IP address, retries exhausted: %w", err)
	}
	return ip, nil
}

func getIPAddress(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("Unable to create request to api.ipify.org: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Unable to get IP address: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Unable to get IP address, got code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Unable to read api.ipify.org body: %w", err)
	}
	return string(body), nil
}

func setDyndnsIPAddressWithRetry(ctx context.Context, client *http.Client, baseURL string, r dynDNSRequest, opts ...backoff.RetryOption) error {
	operation := func() (string, error) {
		return setDyndnsIPAddress(ctx, client, baseURL, r)
	}
	notify := func(err error, d time.Duration) {
		log.Printf("Unable to set dyndns ip for domain %s: %s, retrying in %s\n", r.domainName, err, d)
	}

	retryOpts := append([]backoff.RetryOption{
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithNotify(notify),
	}, opts...)
	result, err := backoff.Retry(ctx, operation, retryOpts...)
	if err != nil {
		return fmt.Errorf("Unable to set dyndns ip for domain %s, retries exhausted: %w", r.domainName, err)
	}
	log.Printf("Set dyndns ip for domain %s: %s\n", r.domainName, result)
	return nil
}

func setDyndnsIPAddress(ctx context.Context, client *http.Client, baseURL string, r dynDNSRequest) (string, error) {
	url := fmt.Sprintf("%s?system=dyndns&hostname=%s&myip=%s", baseURL, r.domainName, r.ipAddress)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("Unable to create request to set IP Address for domain %s: %w", r.domainName, err)
	}
	req.SetBasicAuth(r.username, r.password)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Unable to set IP Address for domain %s: %w", r.domainName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Unable to set IP Address for domain, got code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Unable to read response body for domain %s: %w", r.domainName, err)
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
