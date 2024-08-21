package v1

import (
	"database/sql"
	"strconv"
	"time"

	"github.com/codeArtisanry/go-boilerplate/config"
	"github.com/codeArtisanry/go-boilerplate/models"
	"go.uber.org/zap"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

type UserController struct {
	model  *models.Queries
	logger *zap.Logger
	cfg    config.AppConfig
}

func NewUserController(db *sql.DB, logger *zap.Logger, cfg config.AppConfig) (*UserController, error) {
	userModel := models.New(db)
	return &UserController{
		model:  userModel,
		logger: logger,
		cfg:    cfg,
	}, nil
}

func (u *UserController) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "id is required"})
	}

	// Delete the user
	idInt32, err := strconv.Atoi(id)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "id must be an integer"})
	}
	_, err = u.model.DeleteUser(c.Context(), int32(idInt32))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error while deleting the user": err.Error()})
	}

	return c.Status(200).JSON(fiber.Map{"message": "user deleted successfully"})
}

func (u *UserController) GetUsers(c *fiber.Ctx) error {

	// Get all users from model from function GetUsers and show whole user object in a array
	users, err := u.model.GetUsers(c.Context())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error while getting the users": err.Error()})
	}

	return c.Status(200).JSON(fiber.Map{"users": users})
}

func (u *UserController) GetUserByIdOrEmail(c *fiber.Ctx) error {
	if c.Params("id") != "" {
		id := c.Params("id")
		if id == "" {
			return c.Status(400).JSON(fiber.Map{"error": "id is required"})
		}

		// Get the user by ID
		idInt32, err := strconv.Atoi(id)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "id must be an integer"})
		}
		user, err := u.model.GetUserById(c.Context(), int32(idInt32))
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error while getting the user": err.Error()})
		}

		return c.Status(200).JSON(fiber.Map{"user": user})
	}
	if c.Params("email") != "" {
		email := c.Params("email")
		if email == "" {
			return c.Status(400).JSON(fiber.Map{"error": "email is required"})
		}

		// Get the user by email
		user, err := u.model.GetUserByEmail(c.Context(), email)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error while getting the user": err.Error()})
		}

		return c.Status(200).JSON(fiber.Map{"user": user})
	}

	return c.Status(400).JSON(fiber.Map{"error": "id or email is required"})

}

func (u *UserController) UpdateUser(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "id is required"})
	}

	user := new(models.User)
	if err := c.BodyParser(user); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	// Update the user
	idInt32, err := strconv.Atoi(id)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "id must be an integer"})
	}
	_, err = u.model.UpdateUser(c.Context(), models.UpdateUserParams{
		ID:       int32(idInt32),
		Name:     user.Name,
		Email:    user.Email,
		Password: user.Password,
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error while updating the user": err.Error()})
	}

	return c.Status(200).JSON(fiber.Map{"message": "user updated successfully"})
}

func (u *UserController) Register(c *fiber.Ctx) error {
	user := new(models.User)
	if err := c.BodyParser(user); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	if user.Name == "" || user.Password == "" || user.Email == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name, password, and email are required"})
	}

	// Logic for storing user in database (same as Create method)
	// Retrieve the last inserted ID
	lastID, err := u.model.GetLastId(c.Context())
	if err != nil {
		// Handle case where no users exist yet
		if err == sql.ErrNoRows {
			lastID = 0
		} else {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
	}

	// Increment the ID for the new user
	newID := lastID + 1

	// Create the new user
	_, err = u.model.CreateUser(c.Context(), models.CreateUserParams{
		ID:       newID,
		Name:     user.Name,
		Email:    user.Email,
		Password: user.Password,
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error while creating the user": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{"message": "user registered successfully", "id": newID})
}

func (u *UserController) Login(c *fiber.Ctx) error {
	loginData := new(struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	})
	if err := c.BodyParser(loginData); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	user, err := u.model.GetUserByEmail(c.Context(), loginData.Email)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "invalid credentials"})
	}

	if user.Password != loginData.Password {
		return c.Status(401).JSON(fiber.Map{"error": "invalid credentials"})
	}

	// Generate JWT
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["user_id"] = user.ID
	claims["exp"] = time.Now().Add(time.Hour * 72).Unix()

	t, err := token.SignedString([]byte(u.cfg.JWTSecret))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "could not login"})
	}

	return c.JSON(fiber.Map{"token": t})
}
