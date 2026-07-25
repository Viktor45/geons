#!/usr/bin/env bash
set -euo pipefail

dig +short TXT 8.8.8.8.geons @127.0.0.1 -p 5300
