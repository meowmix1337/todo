package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/meowmix1337/the_recipe_book/internal/model/domain"
	"github.com/meowmix1337/the_recipe_book/internal/model/endpoint"
	"github.com/meowmix1337/the_recipe_book/internal/repo"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

type UserService interface {
	SignUp(ctx context.Context, userSignup *domain.UserSignup) error
	Login(ctx context.Context, userCredentials *domain.UserCredentials) (*endpoint.JWTResponse, error)
	Logout(ctx context.Context, token string, claims *domain.JWTCustomClaims) error
	RefreshToken(ctx context.Context, jwtToken string, user *domain.User, refreshToken string, expiresAt time.Time) (*endpoint.JWTResponse, error)

	ByEmail(ctx context.Context, email string) (*domain.User, error)
	ByEmailWithPassword(ctx context.Context, email string) (*domain.User, error)
}

type userService struct {
	*BaseService

	authService AuthService

	userRepo repo.UserRepo
}

func NewUserService(base *BaseService, authService AuthService, userRepo repo.UserRepo) *userService {
	return &userService{
		BaseService: base,
		authService: authService,
		userRepo:    userRepo,
	}
}

// check UserService interface implementation on compile time.
var _ UserService = (*userService)(nil)

func (u *userService) SignUp(ctx context.Context, userSignup *domain.UserSignup) error {
	if userSignup == nil {
		return fmt.Errorf("no user sign up details provided")
	}

	// check if email exists already
	user, err := u.ByEmail(ctx, userSignup.Email)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return err
	}

	if user != nil {
		return domain.ErrUserAlreadyExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(userSignup.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Err(err).Msg("error generating hash password")
		return err
	}

	// generate uuid
	uuid := u.GenerateUUIDHash("user")

	err = u.userRepo.Create(ctx, uuid, userSignup.Email, string(hashedPassword))
	if err != nil {
		log.Err(err).Msg("error creating user")
		return fmt.Errorf("error creating user: %w", err)
	}

	return nil
}

func (u *userService) Login(ctx context.Context, userCredentials *domain.UserCredentials) (*endpoint.JWTResponse, error) {
	if userCredentials == nil {
		log.Err(domain.ErrNoCredentialsProvided).Msg("no credentials were provided")
		return nil, fmt.Errorf("no user login credentials provided: %w", domain.ErrNoCredentialsProvided)
	}

	user, err := u.ByEmailWithPassword(ctx, userCredentials.Email)
	if err != nil {
		return nil, err
	}

	// Compare the stored hash with the provided password
	if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(userCredentials.Password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			log.Err(domain.ErrInvalidCredentials).Msg("invalid credentials")
			return nil, fmt.Errorf("invalid credentials: %w", domain.ErrInvalidCredentials)
		}
		log.Err(err).Msg("error comparing password")
		return nil, err
	}

	token, err := u.authService.GenerateToken(ctx, user)
	if err != nil {
		return nil, err
	}

	refreshToken, err := u.authService.GenerateRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &endpoint.JWTResponse{
		Token:        token,
		RefreshToken: refreshToken,
	}, nil
}

func (u *userService) Logout(ctx context.Context, token string, claims *domain.JWTCustomClaims) error {
	err := u.authService.BlacklistToken(ctx, token, claims.UserID, claims.ExpiresAt.Time)
	if err != nil {
		return err
	}

	return u.authService.DeleteRefreshToken(ctx, claims.UserID)
}

func (u *userService) RefreshToken(ctx context.Context, jwtToken string, user *domain.User, refreshToken string, expiresAt time.Time) (*endpoint.JWTResponse, error) {
	// make sure refresh token exists
	rt, err := u.authService.ByRefreshToken(ctx, user.ID, refreshToken)
	if err != nil {
		log.Err(err).Msg("could not find refresh token")
		return nil, err
	}

	// if the token has expired, delete it and return unauthorized
	if rt.ExpiresAt.Before(time.Now()) {
		err = u.authService.DeleteRefreshToken(ctx, user.ID)
		if err != nil {
			return nil, err
		}

		return nil, domain.ErrUnauthorized
	}

	// refresh the token and generate a JWT token
	newJwtToken, err := u.authService.GenerateToken(ctx, user)
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := u.authService.GenerateRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// blacklist jwtToken
	err = u.authService.BlacklistToken(ctx, jwtToken, user.ID, expiresAt)
	if err != nil {
		return nil, err
	}

	return &endpoint.JWTResponse{
		Token:        newJwtToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (u *userService) ByEmail(ctx context.Context, email string) (*domain.User, error) {
	user, err := u.userRepo.ByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Err(domain.ErrUserNotFound).Msg("user not found")
			return nil, fmt.Errorf("user not found: %w", domain.ErrUserNotFound)
		}
		log.Err(err).Msg("error retreiving user by email")
		return nil, err
	}
	return user, nil
}

func (u *userService) ByEmailWithPassword(ctx context.Context, email string) (*domain.User, error) {
	user, err := u.userRepo.ByEmailWithPassword(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Err(domain.ErrUserNotFound).Msg("user not found")
			return nil, fmt.Errorf("user not found: %w", domain.ErrUserNotFound)
		}
		log.Err(err).Msg("error retreiving user by email")
		return nil, err
	}
	return user, nil
}
