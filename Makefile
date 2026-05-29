.PHONY: dummyjira build test lint demo

build:
	go build -o jira-kanban .

test:
	go test ./...

lint:
	golangci-lint run

dummyjira:
	go run ./cmd/dummyjira

demo: build
	@echo "1. Run 'make dummyjira' in another terminal"
	@echo "2. Set config:"
	@echo "   mkdir -p ~/.config/jira-kanban"
	@echo '   cat > ~/.config/jira-kanban/config.yml << EOF'
	@echo '   base_url: "http://localhost:9191"'
	@echo '   email: "demo@example.com"'
	@echo '   api_token: "dummy-token"'
	@echo '   board_id: 1'
	@echo '   EOF'
	@echo "3. Run ./jira-kanban"