package domain

import (
	"errors"
	"time"
)

var (
	ErrNoCredentialsProvided = errors.New("no credentials provided")
	ErrUserNotFound          = errors.New("user not found")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrJWTGeneration         = errors.New("error generating jwt token")
	ErrUnauthorized          = errors.Join(ErrInvalidCredentials, ErrNoCredentialsProvided, ErrUserNotFound)
)

const (
	JWTExpiration = time.Hour * 72
)

type UserSignup struct {
	Email    string
	Password string
}

type UserCredentials struct {
	Email    string
	Password string
}

type User struct {
	ID       uint
	UUID     string
	Email    string
	Password string
}
