package jwt

import (
	"Backend/models"
	"crypto/rsa"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
	"log"
	"os"
)

var (
	privateKeyPath = "jwt/private_key.pem"
	publicKeyPath  = "jwt/public_key.pem"
)

// 讀取私鑰
func loadPrivateKey() (*rsa.PrivateKey, error) {
	keyBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, err
	}

	key, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// 讀取公鑰
func loadPublicKey() (*rsa.PublicKey, error) {
	keyBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, err
	}

	key, err := jwt.ParseRSAPublicKeyFromPEM(keyBytes)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// 生成JWT Token
func GenerateToken(userID uint, role string, expTime int64) (string, error) {
	privateKey, err := loadPrivateKey()
	if err != nil {
		return "", err
	}

	token := jwt.New(jwt.SigningMethodRS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["userID"] = userID
	claims["exp"] = expTime //time.Now().Add(time.Hour).Unix()
	claims["role"] = role

	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// 驗證JWT Token並回傳UserID
func VerifyToken(tokenString *string, db *gorm.DB) (uint, string, error) {
	publicKey, err := loadPublicKey()
	if err != nil {
		return 0, "", err
	}

	token, err := jwt.Parse(*tokenString, func(token *jwt.Token) (interface{}, error) {
		return publicKey, nil
	})
	if err != nil {
		return 0, "", err
	}

	if !token.Valid {
		return 0, "", jwt.ErrTokenSignatureInvalid
	}

	//從資料庫檢查Token是否刪除
	var loginToken models.LoginToken
	err = db.Where("token = ?", *tokenString).First(&loginToken).Error
	if err != nil {
		log.Println(err)
		return 0, "", err
	}

	claims := token.Claims.(jwt.MapClaims)
	userID := uint(claims["userID"].(float64))
	role := claims["role"].(string)

	return userID, role, nil
}
