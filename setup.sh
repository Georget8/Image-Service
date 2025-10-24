#!/bin/bash

echo "🚀 Creating Image Service project structure..."

# Create directories
mkdir -p cmd/server
mkdir -p internal/{handler,processor,cache,middleware}
mkdir -p pkg/config

# Create .env
cat > .env << 'EOF'
PORT=3000
REDIS_URL=localhost:6379
REDIS_PASSWORD=
ALLOWED_DOMAINS=*
CACHE_TTL=86400
MAX_IMAGE_SIZE=10485760
RATE_LIMIT=100
EOF

# Create .gitignore
cat > .gitignore << 'EOF'
*.exe
*.dll
*.so
*.dylib
server
*.test
*.out
go.work
.env
.env.local
.vscode/
.idea/
*.swp
*.swo
EOF

echo "✅ Project structure created!"
echo "📝 Next steps:"
echo "1. Create the Go files in each directory"
echo "2. Run: go mod tidy"
echo "3. Run: go run cmd/server/main.go"
