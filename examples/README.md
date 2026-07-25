# Examples

These [examples](./) show how to resolve TXT records from a running GeoNS server at 127.0.0.1:5300.

Examples are for demonstration purpose only and may contain errors or old code versions.

The examples use the synthetic domain for an IP address, for example:

- 8.8.8.8.geons
- 4.4.2.2.geocity
- 8.8.8.8.asn

## Run the examples

- Go: `go run ./examples/go/resolve.go`
- Python: `python3 ./examples/python/resolve.py`
- Node.js: `node ./examples/nodejs/resolve.js`
- Bash: `bash ./examples/bash/resolve.sh`
- PHP: `php ./examples/php/resolve.php`
- Ruby: `ruby ./examples/ruby/resolve.rb`
- C#: `dotnet run --project ./examples/csharp`
- Rust: `cargo run --manifest-path ./examples/rust/Cargo.toml`

## Notes

The PHP, C# and Rust examples now use popular DNS client libraries:

- PHP uses `pear/net_dns2`
- C# uses `DnsClient`
- Rust uses `trust-dns-resolver`

For the bash example, `dig` is used directly.

Adjust the domain name to match the zone you configured in your GeoNS server.


