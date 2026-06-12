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

for placement in \
	"a:nth-child\\(1\\)[^{]*\\{[[:space:]]*grid-column:[[:space:]]*1;[[:space:]]*grid-row:[[:space:]]*1;" \
	"a:nth-child\\(2\\)[^{]*\\{[[:space:]]*grid-column:[[:space:]]*1;[[:space:]]*grid-row:[[:space:]]*2;" \
	"a:nth-child\\(3\\)[^{]*\\{[[:space:]]*grid-column:[[:space:]]*3;[[:space:]]*grid-row:[[:space:]]*1;" \
	"a:nth-child\\(4\\)[^{]*\\{[[:space:]]*grid-column:[[:space:]]*3;[[:space:]]*grid-row:[[:space:]]*2;"
do
	if [[ ! $css =~ $placement ]]; then
		printf 'hovercard links should pin each link to its intended grid cell\n' >&2
		exit 1
	fi
done
