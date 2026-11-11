@echo off
cd /d "%~dp0..\apps\web"
if not exist node_modules npm install
npm run dev
