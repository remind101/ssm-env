#!/bin/sh -e

[ ! -z "$JENKINS_GITHUB_USERNAME" ] || (echo '$JENKINS_GITHUB_USERNAME not set!' && exit 1)
[ ! -z "$JENKINS_GITHUB_API_TOKEN" ] || (echo '$JENKINS_GITHUB_API_TOKEN not set!' && exit 1)
APP_NAME=${APP_NAME:-"ssm-env"}

# Publish on github
echo "Publishing on Github..."

cd /output
cp /go/src/github.com/remind101/ssm-env/bin/ssm-env .

# Get the last tag name
TAG="v$(cat version)"

# Setup release artifact filename
RELEASE_FILENAME="${TAG}.zip"
zip "${RELEASE_FILENAME}" ssm-env

# Setup API credentials
USERNAME="${JENKINS_GITHUB_USERNAME}"
TOKEN="${JENKINS_GITHUB_API_TOKEN}"

# Get the full message associated with this tag
MESSAGE="$(git log -1 --pretty=%B)"

# Get the title and the description as separated variables
NAME="${TAG}"
DESCRIPTION=$(echo "${MESSAGE}" | sed -z 's/\n/\\n/g') # Escape line breaks to prevent json parsing problems

# Create a release
RELEASE=$(curl -s -XPOST -H "Authorization:token ${TOKEN}" --data "{\"tag_name\": \"$TAG\", \"target_commitish\": \"intellihr\", \"name\": \"${NAME}\", \"body\": \"${DESCRIPTION}\", \"draft\": false, \"prerelease\": false}" https://api.github.com/repos/intellihr/${APP_NAME}/releases)

# Extract the id of the release from the creation response
ID=$(echo "${RELEASE}" | sed -n -e 's/"id":\ \([0-9]\+\),/\1/p' | head -n 1 | sed 's/[[:blank:]]//g')

# Upload the artifact
curl -s -XPOST -H "Authorization:token ${TOKEN}" -H "Content-Type:application/octet-stream" --data-binary @${RELEASE_FILENAME} https://uploads.github.com/repos/intellihr/${APP_NAME}/releases/${ID}/assets?name=${RELEASE_FILENAME}
