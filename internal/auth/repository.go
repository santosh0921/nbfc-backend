package auth

import (
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

func CreateUser(user *models.User) error {
	return database.DB.Create(user).Error
}