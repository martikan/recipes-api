package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"github.com/martikan/recipes-api/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"net/http"
	"time"
)

type RecipeHandler struct {
	ctx         context.Context
	collection  *mongo.Collection
	redisClient *redis.Client
}

func NewRecipeHandler(ctx context.Context, collection *mongo.Collection, redisClient *redis.Client) *RecipeHandler {
	return &RecipeHandler{
		ctx:         ctx,
		collection:  collection,
		redisClient: redisClient,
	}
}

// recipeDTO is a DTO to create a new recipe
type recipeDTO struct {
	Name         string   `json:"name"`
	Tags         []string `json:"tags"`
	Ingredients  []string `json:"ingredients"`
	Instructions []string `json:"instructions"`
}

// ListRecipes Get all model.Recipe's
//
// @Summary List all recipes
// @Description Get a list of all available recipes
// @Tags Recipes
// @Success 200 {object} model.Recipe
// @Failure 500
// @Router /recipes [get]
func (handler *RecipeHandler) ListRecipes(c *gin.Context) {
	recipes := make([]model.Recipe, 0)
	val, err := handler.redisClient.Get("recipes").Result()
	if errors.Is(err, redis.Nil) {
		log.Println("recipes - call mongo")

		// If it's not cached then call mongodb to cache it
		cur, err := handler.collection.Find(handler.ctx, bson.M{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer cur.Close(handler.ctx)

		for cur.Next(handler.ctx) {
			var recipe model.Recipe
			err := cur.Decode(&recipe)
			if err != nil {
				log.Println("cannot decode recipe: ", err)
			}
			recipes = append(recipes, recipe)
		}

		data, err := json.Marshal(recipes)
		handler.redisClient.Set("recipes", string(data), 0)
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	} else {
		log.Println("recipes - call redis")

		// Use cached result-set
		err = json.Unmarshal([]byte(val), &recipes)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, recipes)
}

// GetRecipe Get model.Recipe by id
//
// @Summary Get recipe by id
// @Description Get a single recipe by id
// @Tags Recipes
// @Success 200 {object} model.Recipe
// @Failure 404
// @Failure 500
// @Router /recipes/{id} [get]
func (handler *RecipeHandler) GetRecipe(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	res := handler.collection.FindOne(handler.ctx, bson.M{
		"_id": id,
	})
	if err = res.Err(); err != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"msg": "Recipe has not found by the given id"})
			return
		}
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var recipe model.Recipe
	err = res.Decode(&recipe)
	if err != nil {
		log.Println("cannot decode recipe: ", err)
	}

	c.JSON(http.StatusOK, recipe)
}

// UpdateRecipe Updates model.Recipe by id
//
// @Summary Update recipe by id
// @Description Update recipe by id
// @Tags Recipes
// @Param id      path int          true "ID of the recipe"
// @Param request body recipeDTO    true "recipe for update"
// @Success 204
// @Failure 400
// @Failure 500
// @Router /recipes/{id} [put]
func (handler *RecipeHandler) UpdateRecipe(c *gin.Context) {
	var recipeDTO recipeDTO
	if err := c.ShouldBindJSON(&recipeDTO); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_, err = handler.collection.UpdateOne(handler.ctx, bson.M{
		"_id": id,
	}, bson.D{{"$set", bson.D{
		{"name", recipeDTO.Name},
		{"instructions", recipeDTO.Instructions},
		{"ingredients", recipeDTO.Ingredients},
		{"tags", recipeDTO.Tags},
	}}})
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	handler.redisClient.Del("recipes")

	c.JSON(http.StatusNoContent, gin.H{"msg": "Recipe has been updated"})
}

// NewRecipe Saves a new model.Recipe
//
// @Summary Save a new recipe
// @Description Save a new recipe
// @Tags Recipes
// @Param request body recipeDTO true "new recipe"
// @Success 201 {object} model.Recipe
// @Failure 400
// @Failure 500
// @Router /recipes [post]
func (handler *RecipeHandler) NewRecipe(c *gin.Context) {
	var recipeDTO recipeDTO
	if err := c.ShouldBindJSON(&recipeDTO); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	recipe := &model.Recipe{
		Id:           primitive.NewObjectID(),
		Name:         recipeDTO.Name,
		Tags:         recipeDTO.Tags,
		Ingredients:  recipeDTO.Ingredients,
		Instructions: recipeDTO.Instructions,
		PublishedAt:  time.Now(),
	}
	_, err := handler.collection.InsertOne(handler.ctx, recipe)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error while inserting a new recipe"})
		return
	}

	handler.redisClient.Del("recipes")

	c.JSON(http.StatusCreated, recipe)
}

// DeleteRecipe Deletes model.Recipe by id
//
// @Summary Delete recipe by id
// @Description Delete recipe by id
// @Tags Recipes
// @Param id path int true "ID of the recipe"
// @Success 204
// @Failure 400
// @Failure 500
// @Router /recipes/{id} [delete]
func (handler *RecipeHandler) DeleteRecipe(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_, err = handler.collection.DeleteOne(handler.ctx, bson.M{
		"_id": id,
	})
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	handler.redisClient.Del("recipes")

	c.JSON(http.StatusNoContent, gin.H{"msg": "Recipe has been removed"})
}
