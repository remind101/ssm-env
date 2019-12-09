#!/bin/sh -e

[ ! -z "$JENKINS_GITHUB_USERNAME" ] || (echo '$JENKINS_GITHUB_USERNAME not set!' && exit 1)
[ ! -z "$JENKINS_GITHUB_API_TOKEN" ] || (echo '$JENKINS_GITHUB_API_TOKEN not set!' && exit 1)
APP_NAME=${APP_NAME:-"ssm-env"}

# Publish on github
echo "Publishing on Github..."

USERNAME="${JENKINS_GITHUB_USERNAME}"
TOKEN="${JENKINS_GITHUB_API_TOKEN}"

# Get the last tag name
TAG=$(git describe --tags)

# Get the full message associated with this tag
MESSAGE="$(git for-each-ref refs/tags/${TAG} --format='%(contents)')"

# Get the title and the description as separated variables
NAME=$(echo "${MESSAGE}" | head -n1)
DESCRIPTION=$(echo "${DESCRIPTION}" | tail -n +3)
DESCRIPTION=$(echo "${DESCRIPTION}" | sed -z 's/\n/\\n/g') # Escape line breaks to prevent json parsing problems

# Create a release
RELEASE=$(curl -XPOST -H "Authorization:token ${TOKEN}" --data "{\"tag_name\": \"$TAG\", \"target_commitish\": \"master\", \"name\": \"${NAME}\", \"body\": \"${DESCRIPTION}\", \"draft\": false, \"prerelease\": true}" https://api.github.com/repos/${USERNAME}/${APP_NAME}/releases)

# Extract the id of the release from the creation response
ID=$(echo "${RELEASE}" | sed -n -e 's/"id":\ \([0-9]\+\),/\1/p' | head -n 1 | sed 's/[[:blank:]]//g')

zip artifact.zip ssm-env

# Upload the artifact
curl -XPOST -H "Authorization:token ${TOKEN}" -H "Content-Type:application/octet-stream" --data-binary @artifact.zip https://uploads.github.com/repos/${USERNAME}/${APP_NAME}>/releases/${ID}/assets?name=artifact.zip
