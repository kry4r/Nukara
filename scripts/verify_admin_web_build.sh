#!/usr/bin/env bash
set -euo pipefail

cd Nukara_Admin_Web
npm run build
test -f dist/index.html
