#!/bin/sh

cd frontend
npm i
npm run build

cd ..
echo "Backend"

mkdir -p web/html
rm -fr web/html/*
cp -R frontend/dist/* web/html/

# Touch the problematic file to force recompilation
print_message "\e[33m" "Touching telegram/bot.go to force recompilation..."
touch telegram/bot.go

# Build backend
print_message "\e[36m" "Building backend..."
go build -x -ldflags "-w -s" -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor" -o sui main.go
