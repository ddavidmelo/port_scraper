help: ## This help.
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# DOCKER TASKS
build: ## Build the container
	docker build --tag port_scraper .

run: ## Run the container
	docker run --name port_scraper port_scraper 

run-d: ## Run the container in the background
	docker run -d --name port_scraper port_scraper

start: ## Start the container
	docker start port_scraper

stop: ## Stop the container
	docker stop port_scraper

rm: ## Delete the container
	docker rm port_scraper

# DOCKER-COMPOSE TASKS
up: ## Run docker-compose
	docker-compose up

up-d: ## Run docker-compose  in the background
	docker-compose up -d

stop-c: ## Stop docker-compose
	docker compose stop

down: ## Stop and remove docker-compose
	docker compose down