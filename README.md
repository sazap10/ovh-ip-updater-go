# OVH IP Updater
Updates the IP addresses for Dynamic DNS hosts in OVH with the current machine's IP.

## Docker
Run the following command:

```
docker run -d \
  -e OVH_USERNAME="YOUR OVH DYNDNS USERNAME" \
  -e OVH_PASSWORD="YOUR OVH DYNDNS PASSWORD" \
  -e DOMAINS="COMMA SEPARATED LIST OF DOMAINS TO UPDATE" \
  -e BUGSNAG_API_KEY="YOUR BUGSNAG API KEY HERE(OPTIONAL)" \
  -e SLEEP_DURATION="HOW OFTEN TO UPDATE IP IN SECONDS(DEFAULTS TO 3600)" \
  sazap10/ovh-ip-updater-go
```

### Docker Compose
Run the following command:

```
docker-compose up -d
```

Make sure to have an .env file in your current directory with the following:

```
OVH_USERNAME=<YOUR OVH DYNDNS USERNAME>
OVH_PASSWORD=<YOUR OVH DYNDNS PASSWORD>
DOMAINS=<COMMA SEPARATED LIST OF DOMAINS TO UPDATE>
BUGSNAG_API_KEY=<YOUR BUGSNAG API KEY HERE(OPTIONAL)>
SLEEP_DURATION=<HOW OFTEN TO UPDATE IP IN SECONDS(DEFAULTS TO 3600)>
```
