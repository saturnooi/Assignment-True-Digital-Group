up:
	docker-compose up --build

down:
	docker-compose down

reset:
	docker-compose down -v
	docker-compose up --build

app:
	docker-compose up --build app

db:
	docker-compose up -d postgres

redis:
	docker-compose up -d redis

migrate:
	docker-compose up migrate

seed:
	docker-compose up seed

logs:
	docker-compose logs -f app