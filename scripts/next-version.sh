#!/usr/bin/env bash
# Print the next ChairLift calendar version tag.
#
# Scheme: vYY.MM.N[-PRERELEASE]
#
#   YY  two-digit year, MM zero-padded month — the release's calendar slot,
#       matching how the Bluefin images are dated (stable-YYYYMMDD).
#   N   sequence within that month, starting at 0.
#
# The leading zero in MM is deliberate and is why this is not svu: it reads
# as a date. GoReleaser's semver parser accepts it and normalises 26.09.0 to
# 26.9.0 internally, so `{{ .Version }}` renders without the zero while the
# tag and the About dialog keep it. That is why the release build injects
# `{{ .Tag }}`, not `{{ .Version }}`.
#
# Usage:
#   scripts/next-version.sh            -> v26.09.0   (or v26.09.1 if 0 exists)
#   scripts/next-version.sh alpha.1    -> v26.09.0-alpha.1
#   scripts/next-version.sh alpha.3    -> v26.09.0-alpha.3 (after alpha.2)
set -euo pipefail

prerelease="${1:-}"

# NEXT_VERSION_SLOT pins the calendar slot (YY.MM); tests use it so the
# answer does not depend on today's date.
slot="${NEXT_VERSION_SLOT:-$(date +%y.%m)}"

# Highest N already tagged in this calendar slot. Release and prerelease tags
# share the sequence, so an alpha does not silently reuse a released number.
highest="$(
	git tag --list "v${slot}.*" |
		sed -E "s/^v${slot}\.([0-9]+).*$/\1/" |
		grep -E '^[0-9]+$' |
		sort -n |
		tail -1 ||
		true
)"

# A prerelease belongs to the version it precedes: while vSLOT.N has only
# prerelease tags, N is unreleased, so its next prerelease and its final
# release both stay on N. Only a final vSLOT.N moves the sequence on.
if [ -z "${highest}" ]; then
	next=0
elif git rev-parse -q --verify "refs/tags/v${slot}.${highest}" >/dev/null; then
	next=$((highest + 1))
else
	next="${highest}"
fi

version="v${slot}.${next}"
if [ -n "${prerelease}" ]; then
	version="${version}-${prerelease}"
fi

if git rev-parse -q --verify "refs/tags/${version}" >/dev/null; then
	echo "next-version: ${version} is already tagged" >&2
	exit 1
fi

printf '%s\n' "${version}"
