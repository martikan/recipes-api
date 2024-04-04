APP_NAME=recipes-api
BUILD_PATH=bin

.PHONY: clean
clean:
	rm -rf $(BUILD_PATH)/
	go mod tidy

.PHONY: build
build: clean
	go build -o $(BUILD_PATH)/$(APP_NAME) main.go

build-docs:
	swag init

start: build build-docs
	MONGO_URI="mongodb://admin:aaa@localhost:27017/recipes?authSource=admin" \
	MONGO_DATABASE="recipesApi" \
	REDIS_HOST="localhost:6379" \
	INIT="false" \
	./$(BUILD_PATH)/$(APP_NAME)