.PHONY: build down up restart logs build-docker ps clean dev attach reset export

# Start all services
up:
	sudo docker compose up -d

# Stop all services
down:
	sudo docker compose down

# Restart all services
restart: down up

# Show logs
logs:
	sudo docker compose logs

# Follow logs
logs-f:
	sudo docker compose logs -f

# Build the Go application
build:
	go build -o templates .

# Build Docker containers
build-docker:
	sudo docker compose build

# Start with forced rebuild
up-build:
	sudo docker compose up --build -d

# Show running containers
ps:
	sudo docker compose ps

# Development mode with live reload
dev:
	sudo docker compose up --build

# Clean up
clean:
	sudo docker compose down --volumes --remove-orphans
	sudo docker system prune -f

# Reset everything
reset: down clean up

# Export templates
export:
	./templates --export

# Attach to templates container
attach:
	sudo docker compose exec -it templates /bin/bash

# Templates service specific commands
templates-logs:
	sudo docker compose logs -f --tail 1000 templates

templates-restart:
	sudo docker compose restart templates

templates-build:
	sudo docker compose build templates

templates-reset:
	sudo docker compose stop templates
	sudo docker compose rm -f templates
	sudo docker compose up -d templates
	sudo docker compose logs -f templates

# Mailpit service specific commands
mailpit-logs:
	sudo docker compose logs -f mailer

mailpit-restart:
	sudo docker compose restart mailer

mailpit-reset:
	sudo docker compose stop mailer
	sudo docker compose rm -f mailer
	sudo docker compose up -d mailer

# SpamAssassin service specific commands
spamassassin-logs:
	sudo docker compose logs -f spamassassin

spamassassin-restart:
	sudo docker compose restart spamassassin

spamassassin-reset:
	sudo docker compose stop spamassassin
	sudo docker compose rm -f spamassassin
	sudo docker compose up -d spamassassin
