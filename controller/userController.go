package controller

import (
	"context"
	"hotel/database"
	helper "hotel/helpers"
	"hotel/models"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

var userCollection *mongo.Collection = database.OpenCollection(database.Client, "input")

func GetUsers() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)

		recordPerPage, err := strconv.Atoi(c.Query("recordPerPage"))
		if err != nil {

		}

		page, err1 := strconv.Atoi(c.Query("page"))
		if err1 != nil {

		}

		startIndex := (page - 1) * recordPerPage
		startIndex, err = strconv.Atoi(c.Query("startIndex"))

		matchStage := bson.D{{"$match", bson.D{{}}}}
		projectStage := bson.D{
			{"$project", bson.D{
				{"_id", 0},
				{"total_count", 1},
				{"input_items", bson.D{{"$slice", []interface{}{"$data", startIndex, recordPerPage}}}},
			}}}

		result, err := userCollection.Aggregate(ctx, mongo.Pipeline{
			matchStage, projectStage})
		defer cancel()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error occured while listing user items"})
		}

		var allUsers []bson.M
		if err = result.All(ctx, &allUsers); err != nil {
			log.Fatal(err)
		}
		c.JSON(http.StatusOK, allUsers[0])

	}
}

func HashPassword(password string) string {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		log.Panic(err)
	}

	return string(bytes)
}

func GetUser() gin.HandlerFunc {
	return func(c *gin.Context) {

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)

		var input models.User

		err := userCollection.FindOne(ctx, bson.M{"input_id": input}).Decode(&input)

		defer cancel()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error occured while listing user items"})
		}
		c.JSON(http.StatusOK, input)
	}
}

func SignUp() gin.HandlerFunc {
	return func(c *gin.Context) {

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)

		var input models.User

		err := c.ShouldBind(&input)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return

		}

		var validate *validator.Validate

		validate = validator.New()

		err1 := validate.Struct(input)

		if err1 != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err1.Error()})
			return
		}

		userCollection.CountDocuments(ctx, bson.M{"email": input.Email})

		password := HashPassword(*input.Password)
		input.Password = &password

		count, err := userCollection.CountDocuments(ctx, bson.M{"phone": input.Phone})

		if count > 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "this email or phone number already exsits"})
			return
		}
		input.Created_at, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
		input.Updated_at, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
		input.ID = primitive.NewObjectID()
		input.User_id = input.ID.Hex()

		token, refreshToken, _ := helper.GenerateAllTokens(*input.Email, *input.First_name, *input.Last_name, input.User_id)
		input.Token = &token
		input.Refresh_Token = &refreshToken

		resultInsertionNumber, insertErr := userCollection.InsertOne(ctx, input)
		if insertErr != nil {
			msg := "User item was not created"
			c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
			return
		}
		defer cancel()

		c.JSON(http.StatusOK, resultInsertionNumber)
	}
}

func VerifyPassword(userPassword string, providedPassword string) (bool, string) {

	err := bcrypt.CompareHashAndPassword([]byte(providedPassword), []byte(userPassword))
	check := true
	msg := ""

	if err != nil {
		msg = "login or password is incorrect"
		check = false
	}
	return check, msg
}

func Login() gin.HandlerFunc {
	return func(c *gin.Context) {

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)

		var input models.User
		var foundUser models.User

		er := c.ShouldBind(&input)

		if er != nil {
			c.JSON(http.StatusInternalServerError, &input)
			return
		}

		userCollection.FindOne(ctx, bson.M{"email": input.Email}).Decode(&foundUser)

		passwordIsValid, msg := VerifyPassword(*input.Password, *foundUser.Password)
		defer cancel()
		if passwordIsValid != true {
			c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
			return
		}

		token, refreshToken, _ := helper.GenerateAllTokens(*foundUser.Email, *foundUser.First_name, *foundUser.Last_name, foundUser.User_id)

		helper.UpdateAllTokens(token, refreshToken, foundUser.User_id)

		c.JSON(http.StatusOK, foundUser)

	}
}
