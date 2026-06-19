# GeoNS - Lightweight DNS Server for IP Geolocation

A simple, fast, and configurable DNS server written in Go that returns geolocation information (country code, name, etc.) for IP addresses using [MaxMind's GeoLite2](https://dev.maxmind.com/geoip/geoip2/geolite2/) Country databases

## Features

- **Lightweight DNS server** with minimal dependencies
- **MaxMind GeoLite2-Country support** for accurate IP geolocation
- **Configurable response fields** - extract any field from the GeoIP2 database structure
- **Access Control Lists (ACL)** - restrict access to specific CIDR ranges
- **Hot reload** - reload configuration and database without downtime (SIGHUP)
- **Graceful shutdown** - clean server termination (SIGINT/SIGTERM)
- **IPv4 and IPv6 support** - automatic IP version detection
- **Flexible TXT record format** - customizable separators and field selection

## Requirements

- Go 1.25 or higher
- MaxMind GeoLite2-Country.mmdb database file ([download here](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data))

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

3. Download GeoLite2-Country.mmdb from MaxMind and place it in the project directory.

## Configuration

Create a `config.yaml` file:

```yaml
server:
  port: 5300
  bind_address: "127.0.0.1"
  # The domain the server will respond to (the IP will be prepended to it)
  # example domain for ip: 8.8.8.8 is 8.8.8.8.geons
  # example query: dig +short TXT 8.8.8.8.geons @127.0.0.1 -p 5300
  domain_suffix: ".geons"
  # Whitelist of networks (CIDR) that are allowed to make requests
  allowed_clients:
    - "127.0.0.0/8"
  #  - "192.168.0.0/16"
  #  - "172.16.0.0/12"
  #  - "10.0.0.0/8"

database:
  path: "GeoLite2-Country.mmdb"

response:
  # Separator between field values ​​in a TXT record
  separator: "|"
  # Fields to extract (use field names from the structure geoip2.Country)
  # Example: Country.IsoCode, Country.Names.en, Country.Names.ru, Continent.Code
  # Example response for 8.8.8.8.geons: "US|United States"
  fields:
    - "Country.IsoCode"
    - "Country.Names.en"

```

### Configuration Options

#### `server`
- `port` - UDP port to listen on (default: 5300)
- `bind_address` - IP address to bind the UDP listener to (default: `127.0.0.1`)
- `domain_suffix` - Suffix for DNS queries (e.g., `.geons` means queries like `8.8.8.8.geons`)
- `allowed_clients` - List of CIDR ranges allowed to make queries

#### `database`
- `path` - Path to MaxMind GeoLite2-Country.mmdb file

#### `response`
- `separator` - String to separate field values in TXT response
- `fields` - List of fields to extract from GeoIP2 database

### Available Fields

The `fields` option supports any field from the `geoip2.Country` structure:

- `Country.IsoCode` - ISO 3166-1 alpha-2 country code (e.g., "US", "DE")
- `Country.Names.en` - Country name in English
- `Country.Names.ru` - Country name in Russian
- `Country.Names.*` - Country name in any available language
- `Continent.Code` - Continent code (e.g., "NA", "EU", "AS")
- `Continent.Names.en` - Continent name in English
- `RegisteredCountry.IsoCode` - Registered country ISO code
- `RepresentedCountry.IsoCode` - Represented country ISO code

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

Use `dig`, `host`, or any DNS client to query the server:

```bash
# Query country info for Google DNS (8.8.8.8)
dig TXT 8.8.8.8.geons @127.0.0.1 -p 5300

# Expected response:
# ;; ANSWER SECTION:
# 8.8.8.8.geons.    60  IN  TXT "US|United States"

# Query for Cloudflare DNS (1.1.1.1)
dig TXT 1.1.1.1.geons @127.0.0.1 -p 5300

# Expected response:
# 1.1.1.1.geons.    60  IN  TXT "AU|Australia"

# IPv6 query
dig TXT "2001:4860:4860::8888.geons" @127.0.0.1 -p 5300
```

### Using `host` Command

```bash
host -t txt 8.8.8.8.geons 127.0.0.1
```

### Hot Reload Configuration

Send SIGHUP signal to reload configuration and database without restarting:

```bash
# Find server PID
ps aux | grep geons

# Send reload signal
kill -HUP <PID>
```

The server will:
1. Re-read `config.yaml`
2. Re-open the MaxMind database file
3. Apply new settings atomically

This is useful when you update the GeoLite2 database or change configuration.

### Graceful Shutdown

Send SIGINT or SIGTERM to stop the server gracefully:

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
3. Domain name is parsed to extract IP address
4. IP is looked up in MaxMind GeoLite2-Country database
5. Configured fields are extracted using reflection
6. TXT record is returned with field values separated by configured separator

### Thread Safety

- Configuration and database are protected by `sync.RWMutex`
- Multiple concurrent queries can read configuration simultaneously
- Hot reload acquires write lock to atomically update configuration
- No request blocking during reload

### Performance

- Lightweight and fast with minimal overhead
- MaxMind MMDB format provides fast memory-mapped lookups
- Concurrent query processing with goroutines
- Low memory footprint

## Example Use Cases

1. **Geolocation API** - Quick IP-to-country lookup via DNS
2. **Network diagnostics** - Identify geographic location of IPs
3. **Content filtering** - Block or allow traffic based on country
4. **Analytics** - Track geographic distribution of traffic
5. **Load balancing** - Route traffic based on geographic location

## Troubleshooting

### Server won't start
- Check if port is already in use
- Verify `config.yaml` syntax
- Ensure GeoLite2-Country.mmdb file exists and is readable

### Access denied errors
- Check if client IP is in `allowed_clients` list
- Verify CIDR format is correct (e.g., `192.168.1.0/24`)

### Empty or incorrect responses
- Verify database file is valid GeoLite2-Country format
- Check field names in `response.fields` match geoip2.Country structure
- Enable debug logging to see detailed errors

## License

This project is licensed under the MIT License - see the LICENSE file for details.

All trademarks are the property of their respective owners.

- Database Copyright (c) [MaxMind](https://www.maxmind.com/), Inc.
- [GeoLite2 End User License Agreement](https://www.maxmind.com/en/geolite2/eula)
- [Creative Commons Corporation Attribution-ShareAlike 4.0 International License (the "Creative Commons License")](https://creativecommons.org/licenses/by-sa/4.0/)

## Acknowledgments

- [miekg/dns](https://github.com/miekg/dns) - DNS library for Go
- [oschwald/geoip2-golang](https://github.com/oschwald/geoip2-golang) - MaxMind GeoIP2 Reader
- [MaxMind](https://www.maxmind.com/) - GeoLite2 geolocation database

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Support

For issues and questions, please open an issue on GitHub.