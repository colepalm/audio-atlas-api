package auth

import (
	"gorm.io/gorm"
)

type Handler struct {
	DB        *gorm.DB
	JWTSecret []byte
}

func NewHandler(db *gorm.DB, jwtSecret string) *Handler {
	return &Handler{
		DB:        db,
		JWTSecret: []byte(jwtSecret),
	}
}
