package handlers

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
	"log"
	"net/http"
	"time"

	"audio-atlas-api/models"
)

type AuthHandler struct {
	OAuthConfig *oauth2.Config
	OAuthState  string
	DB          *gorm.DB
	JWTSecret   []byte
}

func NewAuthHandler(
	clientID string,
	clientSecret string,
	state string,
	redirectURL string,
	db *gorm.DB,
	jwtSecret string,
) *AuthHandler {
	return &AuthHandler{
		OAuthConfig: &oauth2.Config{
			RedirectURL:  redirectURL,
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes: []string{
				"user-read-private",
				"user-read-email",
				"playlist-read-private",
			},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.spotify.com/authorize",
				TokenURL: "https://accounts.spotify.com/api/token",
			},
		},
		OAuthState: state,
		DB:         db,
		JWTSecret:  []byte(jwtSecret),
	}
}

// ExchangeCodeForToken handles exchanging the authorization code for an access token
func (a *AuthHandler) ExchangeCodeForToken(c *gin.Context) {
	var requestData struct {
		Code string `json:"code"`
	}

	if err := c.BindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	token, err := a.OAuthConfig.Exchange(c, requestData.Code)
	if err != nil {
		log.Printf("Token exchange failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange token"})
		return
	}

	// Create Spotify client
	client := a.OAuthConfig.Client(c, token)

	// Fetch Spotify profile
	resp, err := client.Get("https://api.spotify.com/v1/me")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch profile"})
		return
	}
	defer resp.Body.Close()

	var profile struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid profile response"})
		return
	}

	// Find or create user
	var user models.User
	err = a.DB.Where("email = ?", profile.Email).First(&user).Error
	if err != nil {
		user = models.User{
			ID:    uuid.New(),
			Email: profile.Email,
		}
		a.DB.Create(&user)
	}

	// Upsert provider connection
	var provider models.UserProvider
	err = a.DB.Where("provider = ? AND provider_user_id = ?", "spotify", profile.ID).
		First(&provider).Error

	if err != nil {
		provider = models.UserProvider{
			ID:             uuid.New(),
			UserID:         user.ID,
			Provider:       "spotify",
			ProviderUserID: profile.ID,
		}
	}

	provider.AccessToken = token.AccessToken
	provider.RefreshToken = token.RefreshToken
	provider.TokenExpiry = token.Expiry

	a.DB.Save(&provider)

	// Generate JWT
	jwtToken, err := a.generateJWT(user.ID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": jwtToken,
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
		},
	})
}

func (a *AuthHandler) RefreshToken(c *gin.Context) {
	var requestData struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	tokenSource := a.OAuthConfig.TokenSource(c, &oauth2.Token{
		RefreshToken: requestData.RefreshToken,
	})
	newToken, err := tokenSource.Token()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": newToken.AccessToken,
		"expires_in":   int(newToken.Expiry.Sub(time.Now()).Seconds()),
	})
}

func (a *AuthHandler) Me(c *gin.Context) {
	userIDRaw, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userID, _ := uuid.Parse(userIDRaw.(string))

	var user models.User
	if err := a.DB.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":    user.ID,
		"email": user.Email,
	})
}

func (a *AuthHandler) generateJWT(userID string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.JWTSecret)
}
