FROM golang:{{.GoVersion}}-alpine

# The latest alpine images don't have some tools like (`git` and `bash`) for development.
# Adding git, bash, openssh, and other necessary tools to the image for development purposes.
RUN apk update && apk upgrade && apk add --no-cache bash git openssh make autoconf gcc libc-dev sudo procps curl

# Build delve debug tools
RUN go install github.com/go-delve/delve/cmd/dlv@latest

# Install live-reload tool
RUN go install github.com/cosmtrek/air@latest

WORKDIR /app
COPY . .
RUN go build

# {{.AppPort}} for your app and 40000 for debugging
EXPOSE {{.AppPort}} 40000

CMD [ "{{.ProjectObject.PkgPath}}", "start" ]