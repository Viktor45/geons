use std::net::{IpAddr, Ipv4Addr, SocketAddr};
use trust_dns_resolver::config::{NameServerConfig, NameServerConfigGroup, Protocol, ResolverConfig, ResolverOpts};
use trust_dns_resolver::TokioAsyncResolver;
use trust_dns_resolver::proto::rr::RecordType;

#[tokio::main]
async fn main() {
    let nameserver = SocketAddr::new(IpAddr::V4(Ipv4Addr::new(127, 0, 0, 1)), 5300);
    let mut ns_group = NameServerConfigGroup::new();
    ns_group.push(NameServerConfig::udp(nameserver));

    let config = ResolverConfig::from_parts(None, vec![], ns_group);
    let opts = ResolverOpts::default();
    let resolver = TokioAsyncResolver::tokio(config, opts).unwrap();

    let response = resolver
        .lookup("8.8.8.8.geons", RecordType::TXT)
        .await
        .unwrap();

    for record in response.iter() {
        println!("{}", record.to_string());
    }
}
