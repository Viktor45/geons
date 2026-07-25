#!/usr/bin/env python3

import dns.resolver

resolver = dns.resolver.Resolver(configure=False)
resolver.nameservers = ["127.0.0.1"]
resolver.port = 5300

answers = resolver.resolve("8.8.8.8.geons", "TXT")
for answer in answers:
    print(answer.to_text())
