# CLASP — Claude Analytics & Standards Platform

## Build
go build -o clasp .

## Test
go test ./...

## Project structure
- cmd/ — CLI commands (cobra)
- internal/ — Core packages (collector, redactor, uploader, auth, sync, watermark, config, platform)
- scripts/ — Install scripts for macOS/Windows

## Module
github.com/donnycrash/clasp
