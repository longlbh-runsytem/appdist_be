DB_URL ?= postgresql://root:Hoanglong2502@localhost:5432/appdist?sslmode=disable

postgres:
	docker run --name appdist-postgres -p 5432:5432 -e POSTGRES_USER=root -e POSTGRES_PASSWORD=secret -d postgres:17-alpine

createdb:
	docker exec -it appdist-postgres createdb --username=root --owner=root appdist

dropdb:
	docker exec -it appdist-postgres dropdb appdist

migrateup:
	migrate -path db/migrations -database "$(DB_URL)" -verbose up

migrateup1:
	migrate -path db/migrations -database "$(DB_URL)" -verbose up 1

migratedown:
	migrate -path db/migrations -database "$(DB_URL)" -verbose down

migratedown1:
	migrate -path db/migrations -database "$(DB_URL)" -verbose down 1

new_migration:
	migrate create -ext sql -dir db/migrations -seq $(name)

db_docs:
	dbdocs build db/docs/db.dbml

db_schema:
	dbml2sql db/docs/db.dbml --postgres -o db/docs/schema.sql

sqlc:
	sqlc generate

# go run thay vì binary cài sẵn: không phụ thuộc PATH, và CLI luôn cùng
# phiên bản với thư viện swag trong go.mod.
SWAG := go run github.com/swaggo/swag/cmd/swag@v1.16.6

swagger:
	$(SWAG) fmt
	$(SWAG) init -g main.go -o docs --parseInternal

test:
	go test -v -cover ./...

server:
	go run main.go

# make create_admin email=a@b.com name="Admin" password=secret123
create_admin:
	go run ./cmd/createadmin -email "$(email)" -name "$(name)" -password "$(password)"

# Image cho server x86: make docker_build version=1.0.0
# Không có registry thì đẩy qua SSH: make docker_push_ssh version=1.0.0 host=user@server
version ?= latest
docker_build:
	docker buildx build --platform linux/amd64 -t appdist-be:$(version) --load .

docker_push_ssh:
	docker save appdist-be:$(version) | gzip | ssh $(host) 'gunzip | docker load'

.PHONY: postgres createdb dropdb migrateup migrateup1 migratedown migratedown1 new_migration db_docs db_schema sqlc swagger test server create_admin docker_build docker_push_ssh
