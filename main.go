package main

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	_ "github.com/martikan/recipes-api/docs"
	"github.com/martikan/recipes-api/handlers"
	"github.com/martikan/recipes-api/model"
	"github.com/swaggo/files"
	"github.com/swaggo/gin-swagger"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"log"
	"os"
	"strings"
)

// Recipes-API
//
// @title Recipes API
// @version 1.0.0
// @description This API provides operations for managing recipes.
// @contact.name Richard Martikan
// @contact.email ric.martikan@gmail.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @Schemes https
// @host localhost:8085
// @Accept json
// @Produce json
// @BasePath /

var (
	recipeHandler *handlers.RecipeHandler
)

// initDemoDB initializes a demo database with test-data
func initDemoDB(ctx context.Context, collections *mongo.Collection) {
	recipes := make([]model.Recipe, 0)
	file, _ := os.ReadFile("resources/init_recipes.json")
	_ = json.Unmarshal([]byte(file), &recipes)

	var listOfRecipes []interface{}
	for _, recipe := range recipes {
		listOfRecipes = append(listOfRecipes, recipe)
	}

	res, err := collections.InsertMany(ctx, listOfRecipes)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("Inserted recipes count: ", len(res.InsertedIDs))
}

// init initializes the API
func init() {
	ctx := context.Background()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST"),
		Password: "",
		DB:       0,
	})
	redisClientStatus := redisClient.Ping()
	log.Println(redisClientStatus)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err = client.Ping(context.Background(), readpref.Primary()); err != nil {
		log.Fatalln(err)
	}
	log.Println("Connected to database")

	collections := client.Database(os.Getenv("MONGO_DATABASE")).Collection("recipes")

	// Init demo-dataset when INIT env var is true
	if val, exist := os.LookupEnv("INIT"); exist && strings.ToLower(val) == "true" {
		initDemoDB(ctx, collections)
	}

	// Init handlers
	recipeHandler = handlers.NewRecipeHandler(ctx, collections, redisClient)
}

func main() {
	router := gin.Default()
	router.GET("/recipes", recipeHandler.ListRecipes)
	router.GET("/recipes/:id", recipeHandler.GetRecipe)
	router.PUT("/recipes/:id", recipeHandler.UpdateRecipe)
	router.POST("/recipes", recipeHandler.NewRecipe)
	router.DELETE("/recipes/:id", recipeHandler.DeleteRecipe)

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	err := router.RunTLS(":8085", "./cert/server.pem", "./cert/server.key")
	if err != nil {
		log.Fatalf("api error: %v\n", err)
	}
}
