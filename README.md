# GeoNS - Lightweight DNS Server for IP Geolocation

A simple, fast, and configurable DNS server-like microservice that returns geolocation information for IP addresses using [MaxMind GeoLite2](https://dev.maxmind.com/geoip/geoip2/geolite2/) databases. Supports **Country**, **City**, and **ASN** databases with multi-zone configuration.

Essentially, it's a database access interface via the DNS protocol, designed for high-load systems; it eliminates the need for such projects to implement dependencies for database updates and read interfaces.

- [GeoNS - Lightweight DNS Server for IP Geolocation](#geons---lightweight-dns-server-for-ip-geolocation)
  - [Features](#features)
  - [Requirements](#requirements)
  - [Installation](#installation)
  - [Configuration](#configuration)
    - [Example Configuration with All Three Zones](#example-configuration-with-all-three-zones)
    - [Minimal Configuration (Single Zone)](#minimal-configuration-single-zone)
  - [Configuration Options](#configuration-options)
    - [`server`](#server)
    - [`zones`](#zones)
  - [Available Fields](#available-fields)
    - [GeoLite2-Country (`type: country`)](#geolite2-country-type-country)
    - [GeoLite2-City (`type: city`)](#geolite2-city-type-city)
    - [GeoLite2-ASN (`type: asn`)](#geolite2-asn-type-asn)
  - [Usage](#usage)
    - [Running the Server](#running-the-server)
    - [Querying the Server](#querying-the-server)
      - [Country zone queries (`.geons`)](#country-zone-queries-geons)
      - [City zone queries (`.geocity`)](#city-zone-queries-geocity)
      - [ASN zone queries (`.asn`)](#asn-zone-queries-asn)
    - [Using `host` Command](#using-host-command)
    - [Hot Reload Configuration](#hot-reload-configuration)
    - [Graceful Shutdown](#graceful-shutdown)
  - [Architecture](#architecture)
    - [Request Flow](#request-flow)
    - [Multi-Zone Support](#multi-zone-support)
    - [Thread Safety](#thread-safety)
    - [Performance](#performance)
  - [Example Use Cases](#example-use-cases)
  - [Troubleshooting](#troubleshooting)
    - [Server won't start](#server-wont-start)
    - [Access denied errors](#access-denied-errors)
    - [Empty or incorrect responses](#empty-or-incorrect-responses)
    - [Field path not found](#field-path-not-found)
    - [Zone not responding](#zone-not-responding)
  - [License](#license)
  - [Acknowledgments](#acknowledgments)
  - [Contributing](#contributing)
  - [Support](#support)


## Features

- **Lightweight DNS microservice** with minimal dependencies
- **Multi-database support** - GeoLite2-Country, GeoLite2-City, and GeoLite2-ASN
- **Multi-zone configuration** - serve multiple databases on different domain suffixes
- **Configurable response fields** - extract any field from the GeoIP2 database structure using dot-notation paths
- **Array indexing support** - access slice elements like `Subdivisions[0].Names.en`
- **Access Control Lists (ACL)** - restrict access to specific CIDR ranges
- **Hot reload** - reload configuration and database without downtime (`SIGHUP`)
- **Graceful shutdown** - clean server termination (`SIGINT`/`SIGTERM`)
- **IPv4 and IPv6 support** - automatic IP version detection
- **Flexible TXT record format** - customizable separators and field selection

## Requirements

- Go 1.25 or higher
- One or more MaxMind GeoLite2 database files ([download here](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data)):
  - `GeoLite2-Country.mmdb`
  - `GeoLite2-City.mmdb`
  - `GeoLite2-ASN.mmdb`

## Installation

1. Clone the repository:
```bash
git clone https://github.com/viktor45/geons.git
cd geons
```

2. Install dependencies:
```bash
go mod download
```

3. Download one or more GeoLite2 `.mmdb` files from MaxMind and place them in the project directory.

## Configuration

Create a `config.yaml` file. The server supports multiple zones, each with its own database and response configuration.

### Example Configuration with All Three Zones

```yaml
server:
  port: 5300
  bind_address: "127.0.0.1"
  # Whitelist of networks (CIDR) that are allowed to make requests
  allowed_clients:
    - "127.0.0.0/8"
    - "192.168.0.0/16"
    - "10.0.0.0/8"

# Zones configuration - only configured zones will be loaded
zones:
  # Country zone
  - name: ".geons"
    database:
      path: "GeoLite2-Country.mmdb"
      type: "country"
    response:
      separator: "|"
      fields:
        - "Country.IsoCode"
        - "Country.Names.en"

  # ASN zone
  - name: ".asn"
    database:
      path: "GeoLite2-ASN.mmdb"
      type: "asn"
    response:
      separator: "|"
      fields:
        - "AutonomousSystemNumber"
        - "AutonomousSystemOrganization"

  # City zone
  - name: ".geocity"
    database:
      path: "GeoLite2-City.mmdb"
      type: "city"
    response:
      separator: "|"
      fields:
        - "City.Names.en"
        - "Subdivisions[0].Names.en"
        - "Country.IsoCode"
        - "Location.Latitude"
        - "Location.Longitude"
        - "Location.TimeZone"
```

### Minimal Configuration (Single Zone)

```yaml
server:
  port: 5300
  bind_address: "127.0.0.1"
  allowed_clients:
    - "127.0.0.0/8"

zones:
  - name: ".geo"
    database:
      path: "GeoLite2-Country.mmdb"
      type: "country"
    response:
      separator: "|"
      fields:
        - "Country.IsoCode"
```

## Configuration Options

### `server`

- `port` - UDP port to listen on (required)
- `bind_address` - IP address to bind the UDP listener to (default: `127.0.0.1`)
- `allowed_clients` - List of CIDR ranges allowed to make queries

### `zones`

Array of zone configurations. Each zone has:

- `name` - Domain suffix for this zone (e.g., `.geons`, `.city`, `.asn`)
- `database.path` - Path to MaxMind GeoLite2 `.mmdb` file
- `database.type` - Database type: `country`, `city`, or `asn`
- `response.separator` - String to separate field values in TXT response
- `response.fields` - List of fields to extract from the GeoIP2 database (dot-notation paths)

## Available Fields

The `fields` option supports any field from the corresponding GeoIP2 structure. Use dot-notation paths to access nested fields.

### GeoLite2-Country (`type: country`)

Based on the `geoip2.Country` structure:

- `Country.IsoCode` - ISO 3166-1 alpha-2 country code (e.g., "US", "DE")
- `Country.Names.en` - Country name in English
- `Country.Names.ru` - Country name in Russian
- `Country.Names.*` - Country name in any available language
- `Continent.Code` - Continent code (e.g., "NA", "EU", "AS")
- `Continent.Names.en` - Continent name in English
- `RegisteredCountry.IsoCode` - Registered country ISO code
- `RepresentedCountry.IsoCode` - Represented country ISO code

### GeoLite2-City (`type: city`)

Based on the `geoip2.City` structure:

- `City.Names.en` - City name in English
- `City.Names.ru` - City name in Russian
- `City.GeoNameId` - City GeoName ID
- `Subdivisions[0].IsoCode` - First subdivision (state/region) ISO code
- `Subdivisions[0].Names.en` - First subdivision name in English
- `Subdivisions[1].Names.en` - Second subdivision name (if available)
- `Country.IsoCode` - Country ISO code
- `Country.Names.en` - Country name in English
- `Location.Latitude` - Latitude coordinate
- `Location.Longitude` - Longitude coordinate
- `Location.TimeZone` - IANA time zone (e.g., "America/Los_Angeles")
- `Location.MetroCode` - Metro code (US only)
- `Location.AccuracyRadius` - Accuracy radius in kilometers
- `Postal.Code` - Postal/ZIP code
- `Continent.Code` - Continent code

> **Note:** Array indexing is supported using `[N]` syntax (e.g., `Subdivisions[0]`). If the index is out of bounds, an empty string is returned.

### GeoLite2-ASN (`type: asn`)

Based on the `geoip2.ASN` structure:

- `AutonomousSystemNumber` - ASN number (e.g., `15169`)
- `AutonomousSystemOrganization` - Organization name (e.g., "Google LLC")

## Usage

### Running the Server

```bash
go run main.go
```

Or build and run:

```bash
go build -o geons
./geons
```

### Querying the Server

Use `dig`, `host`, or any DNS client to query the server.

#### Country zone queries (`.geons`)

```bash
# Query country info for Google DNS (8.8.8.8)
dig TXT 8.8.8.8.geons @127.0.0.1 -p 5300

# Expected response:
# ;; ANSWER SECTION:
# 8.8.8.8.geons.    60  IN  TXT "US|United States"

# Query for Cloudflare DNS (1.1.1.1)
dig TXT 1.1.1.1.geons @127.0.0.1 -p 5300
# 1.1.1.1.geons.    60  IN  TXT "AU|Australia"

# IPv6 query
dig TXT "2001:4860:4860::8888.geons" @127.0.0.1 -p 5300
```

#### City zone queries (`.geocity`)

```bash
# Query detailed location info
dig TXT 8.8.8.8.geocity @127.0.0.1 -p 5300

# Expected response:
# 8.8.8.8.geocity.    60  IN  TXT "Mountain View|California|US|37.4056|-122.0775|America/Los_Angeles"

# Query for Yandex DNS
dig TXT 77.88.8.8.geocity @127.0.0.1 -p 5300
# 77.88.8.8.geocity.    60  IN  TXT "Moscow|Moscow|RU|55.7527|37.6175|Europe/Moscow"
```

#### ASN zone queries (`.asn`)

```bash
# Query ASN info for Google DNS
dig TXT 8.8.8.8.asn @127.0.0.1 -p 5300

# Expected response:
# 8.8.8.8.asn.     60  IN  TXT "15169|Google LLC"

# Query for Cloudflare DNS
dig TXT 1.1.1.1.asn @127.0.0.1 -p 5300
# 1.1.1.1.asn.     60  IN  TXT "13335|Cloudflare, Inc."

# Query for Yandex DNS
dig TXT 77.88.8.8.asn @127.0.0.1 -p 5300
# 77.88.8.8.asn.   60  IN  TXT "13238|Yandex LLC"
```

### Using `host` Command

```bash
host -t txt 8.8.8.8.geons 127.0.0.1
host -t txt 8.8.8.8.geocity 127.0.0.1
host -t txt 8.8.8.8.asn 127.0.0.1
```

### Hot Reload Configuration

Send `SIGHUP` signal to reload configuration and database without restarting:

```bash
# Find server PID
ps aux | grep geons

# Send reload signal
kill -HUP <PID>
```

The server will:
1. Re-read `config.yaml`
2. Re-open all configured MaxMind database files
3. Apply new settings atomically

This is useful when you update the GeoLite2 database or change configuration.

### Graceful Shutdown

Send `SIGINT` or `SIGTERM` to stop the server gracefully:

```bash
# Using Ctrl+C
# or
kill -INT <PID>
# or
kill -TERM <PID>
```

The server will:
1. Stop accepting new connections
2. Finish processing current requests
3. Close all resources
4. Exit cleanly

## Architecture

### Request Flow

1. DNS query arrives at the server
2. Client IP is checked against `allowed_clients` whitelist
3. Domain name is parsed to determine the zone (by suffix) and extract IP address
4. IP is looked up in the zone's configured MaxMind database (Country/City/ASN)
5. Configured fields are extracted using reflection and dot-notation paths
6. TXT record is returned with field values separated by configured separator

### Multi-Zone Support

- Each zone operates independently with its own database and configuration
- Zones are matched by domain suffix (e.g., `.geons`, `.city`, `.asn`)
- Only configured zones are loaded into memory
- Hot reload updates all zones atomically

### Thread Safety

- Configuration and databases are protected by `sync.RWMutex`
- Multiple concurrent queries can read configuration simultaneously
- Hot reload acquires write lock to atomically update configuration
- No request blocking during reload

### Performance

- Lightweight and fast with minimal overhead
- MaxMind MMDB format provides fast memory-mapped lookups
- Concurrent query processing with goroutines
- Low memory footprint

## Example Use Cases

- **Geolocation API** - Quick IP-to-country/city lookup via DNS
- **ASN lookup** - Identify the ISP/organization behind an IP
- **Network diagnostics** - Identify geographic location of IPs
- **Content filtering** - Block or allow traffic based on country or ASN
- **Analytics** - Track geographic and ISP distribution of traffic
- **Load balancing** - Route traffic based on geographic location
- **Fraud detection** - Detect mismatches between user location and IP

## Troubleshooting

### Server won't start

- Check if port is already in use
- Verify `config.yaml` syntax
- Ensure at least one zone is configured
- Verify all `.mmdb` files exist and are readable
- Verify `database.type` is set to one of: `country`, `city`, `asn`

### Access denied errors

- Check if client IP is in `allowed_clients` list
- Verify CIDR format is correct (e.g., `192.168.1.0/24`)

### Empty or incorrect responses

- Verify database file matches the configured `database.type` (e.g., don't use a Country database with `type: city`)
- Check field names in `response.fields` match the corresponding GeoIP2 structure
- For array fields like `Subdivisions`, ensure the index exists (use `[0]` for safety)
- Enable debug logging to see detailed errors

### Field path not found

- Ensure you're using the correct structure for your database type
- Check capitalization - field names are case-sensitive (e.g., `Country.IsoCode`, not `country.isocode`)
- For map fields like `Names`, use the language code as key (e.g., `Country.Names.en`)

### Zone not responding

- Verify the zone name in config matches your query suffix (e.g., `.geons` for queries like `8.8.8.8.geons`)
- Check that the zone is properly configured in the `zones` array
- Verify the database file path is correct

## License

This project is licensed under the MIT License - see the LICENSE file for details.

All trademarks are the property of their respective owners.

Database Copyright (c) [MaxMind](https://www.maxmind.com/), Inc.
- [GeoLite2 End User License Agreement](https://www.maxmind.com/en/geolite2/eula)
- [Creative Commons Corporation Attribution-ShareAlike 4.0 International License](https://creativecommons.org/licenses/by-sa/4.0/)

## Acknowledgments

- [miekg/dns](https://github.com/miekg/dns) - DNS library for Go
- [oschwald/geoip2-golang](https://github.com/oschwald/geoip2-golang) - MaxMind GeoIP2 Reader
- [MaxMind](https://www.maxmind.com/) - GeoLite2 geolocation database

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Support

For issues and questions, please open an issue on GitHub.