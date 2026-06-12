#!/usr/bin/env bash
set -euo pipefail

css=$(<"$(dirname "$0")/../src/styles/custom.css")

if [[ ! $css =~ grid-template-columns:[[:space:]]*max-content[[:space:]]+1px[[:space:]]+max-content\; ]]; then
	printf 'hovercard links should reserve an explicit divider column\n' >&2
	exit 1
fi

if [[ ! $css =~ grid-column:[[:space:]]*2\; ]]; then
	printf 'hovercard divider should render in the reserved divider column\n' >&2
	exit 1
fi
