package handlers

import (
	"Backend/jwt"
	"Backend/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"log"
	"net/http"
	"regexp"
	"time"
	"unicode"
)

// 檢查使用者名稱是否合法
func ValidateUsername(username string) bool {
	if len(username) < 8 || len(username) > 20 {
		return false
	}
	pattern := "^[a-zA-Z0-9_-]+$"
	matched, _ := regexp.MatchString(pattern, username)
	return matched
}

// 檢查信箱是否合法
func ValidateEmail(email string) bool {
	pattern := "^[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\\.[a-zA-Z0-9-.]+$"
	matched, _ := regexp.MatchString(pattern, email)
	return matched
}

// 檢查密碼是否合法
func ValidatePassword(password string) bool {
	if len(password) < 8 || len(password) > 50 {
		return false
	}

	var (
		isUpper   = false
		isLower   = false
		isNumber  = false
		isSpecial = false
		isSpace   = false
	)

	for _, s := range password {
		switch {
		case unicode.IsSpace(s):
			isSpace = true
		case unicode.IsUpper(s):
			isUpper = true
		case unicode.IsLower(s):
			isLower = true
		case unicode.IsDigit(s):
			isNumber = true
		case unicode.IsPunct(s) || unicode.IsSymbol(s):
			isSpecial = true
		default:
		}
	}

	return isUpper && isLower && isNumber && isSpecial && !isSpace
}

// 檢查使用者名稱是否重複
func IsUserNameExists(db *gorm.DB, username string) (bool, error) {
	var user models.User
	err := db.First(&user, "Username = ?", username).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil //使用者名稱沒重複，不代表錯誤
		}
		return false, err //有錯誤
	}
	return true, nil //使用者名稱重複
}

// 檢查Email是否重複
func IsUserEmailExists(db *gorm.DB, email string) (bool, error) {
	var user models.User
	err := db.First(&user, "Email = ?", email).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil //信箱沒重複，不代表錯誤
		}
		return false, err //有錯誤
	}
	return true, nil //信箱重複
}

// 註冊使用者帳戶
func RegisterHandler(c *gin.Context, db *gorm.DB) {
	var newUser models.User
	if err := c.BindJSON(&newUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "綁定請求資料錯誤",
			"error":   err.Error(),
		})
		return
	}

	//檢查使用者名稱是否合法
	if !ValidateUsername(newUser.Username) {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "註冊失敗:不合法的使用者名稱",
		})
		return
	}

	//檢查信箱是否合法
	if !ValidateEmail(newUser.Email) {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "註冊失敗:不合法的信箱",
		})
		return
	}

	//檢查密碼是否合法
	if !ValidatePassword(newUser.Password) {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "註冊失敗:不合法的密碼",
		})
		return
	}

	//檢查使用者名稱是否重複
	result, err := IsUserNameExists(db, newUser.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "註冊失敗:檢查使用者名稱失敗",
			"error":   err.Error(),
		})
		return
	}
	if result {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "註冊失敗:使用者名稱已被使用",
		})
		return
	}

	//檢查Email是否重複
	result, err = IsUserEmailExists(db, newUser.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	if result {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "註冊失敗:信箱已被使用",
		})
		return
	}

	//將密碼Hash
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newUser.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "無法生成Hashed密碼",
			"error":   err.Error(),
		})
		return
	}

	newUser.Password = string(hashedPassword)
	newUser.Role = "user"

	//將newUser儲存到資料庫
	if err := db.Create(&newUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "無法儲存使用者資料至資料庫",
			"error":   err.Error(),
		})
		return
	}

	//成功註冊
	c.JSON(http.StatusCreated, gin.H{
		"message":  "使用者已成功註冊",
		"username": newUser.Username,
	})
	return
}

func LoginHandler(c *gin.Context, db *gorm.DB) {
	//檢查是否已經登入
	if _, ok := c.Get("UserID"); ok {
		c.JSON(http.StatusOK, gin.H{
			"message": "已經登入",
		})
		return
	}

	//從請求擷取帳號和密碼
	var loginReq struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&loginReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "綁定請求資料錯誤",
			"error":   err.Error(),
		})
		return
	}

	log.Println(loginReq)

	//檢查是否有此帳號
	var user models.User
	err := db.First(&user, "Username = ?", loginReq.Username).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "找不到此帳號",
				"error":   err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "資料庫錯誤",
			"error":   err.Error(),
		})
		return
	}

	//檢查密碼是否正確
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginReq.Password))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "密碼錯誤",
			"error":   err.Error(),
		})
		return
	}

	//生成JWT Token
	tokenExpiredTime := time.Now().Add(time.Hour * 24)
	token, err := jwt.GenerateToken(user.Model.ID, user.Role, tokenExpiredTime.Unix())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "生成JWT Token錯誤",
			"error":   err.Error(),
		})
		return
	}

	//儲存LoginToken
	loginToken := models.LoginToken{
		Token:          token,
		ExpirationTime: tokenExpiredTime,
		UserID:         user.ID,
		Role:           user.Role,
	}
	err = db.Create(&loginToken).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "儲存Login Token失敗",
			"error":   err.Error(),
		})
		return
	}

	//成功登入 回傳Token和成功訊息
	c.Header("Authorization", "Bearer "+token)
	c.JSON(http.StatusOK, gin.H{
		"message": "成功登入",
	})
}

func LogOutHandler(c *gin.Context, db *gorm.DB) {
	token, exists := c.Get("Token")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "無法取得Token",
		})
		return
	}

	//刪除此LoginToken
	var loginToken models.LoginToken
	result := db.Delete(&loginToken, "Token = ?", token)
	err := result.Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "資料庫錯誤",
			"error":   err.Error(),
		})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "找不到此token或已登出",
		})
		return
	}

	c.Header("Authorization", "")
	c.JSON(http.StatusOK, gin.H{
		"message": "成功登出",
	})
	return
}

// 查詢使用者資料
func GetUserProfileHandler(c *gin.Context, db *gorm.DB) {
	userID, ok := c.Get("UserID")
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "無法取得使用者ID",
		})
		return
	}

	//嘗試查詢使用者資料
	var user models.User
	err := db.
		First(&user, "id = ?", userID).
		Error

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	//成功查詢使用者資料
	c.JSON(http.StatusOK, gin.H{
		"message": "成功查詢使用者資料",
		"user":    user,
	})
}

// 變更使用者資料
func UpdateUserProfileHandler(c *gin.Context, db *gorm.DB) {
	userID, ok := c.Get("UserID")
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "無法取得使用者ID",
		})
		return
	}

	var user models.User
	err := db.First(&user, "id = ?", userID).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "發生錯誤:無法取得使用者資料",
			"error":   err.Error(),
		})
	}

	var newUserData struct {
		Email       string  `json:"email"`
		OldPassword string  `json:"oldPassword"`
		NewPassword string  `json:"newPassword"`
		Name        *string `json:"name"`
		Phone       *string `json:"phone"`
		Address     *string `json:"address"`
	}
	err = c.ShouldBindJSON(&newUserData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "綁定請求資料錯誤",
			"error":   err.Error(),
		})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(newUserData.OldPassword))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "舊密碼錯誤",
			"error":   err.Error(),
		})
		return
	}

	if newUserData.NewPassword != "" {
		if !ValidatePassword(newUserData.NewPassword) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "不合法的新密碼",
			})
			return
		}
		//將密碼Hash
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newUserData.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "無法生成Hashed密碼",
				"error":   err.Error(),
			})
			return
		}
		user.Password = string(hashedPassword)
	}

	if newUserData.Email != "" {
		if !ValidateEmail(newUserData.Email) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "不合法的Email",
			})
			return
		}
		user.Email = newUserData.Email
	}

	//如果使用者有提供資料則覆蓋(包含空字串)
	if newUserData.Name != nil {
		user.Name = *newUserData.Name
	}
	if newUserData.Phone != nil {
		user.Phone = *newUserData.Phone
	}
	if newUserData.Address != nil {
		user.Address = *newUserData.Address
	}

	result := db.Where("id = ?", userID).Save(&user)
	err = result.Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message": "沒有變更資料",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "成功修改使用者資料",
	})
}
