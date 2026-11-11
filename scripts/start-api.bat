@echo off
cd /d "%~dp0..\apps\api"
if not exist .env copy .env.example .env
go mod tidy
go run ./cmd/api
