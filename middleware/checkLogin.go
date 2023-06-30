package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// 檢查是否有登入，沒有則中止請求
func CheckLoginMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := c.Get("UserID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "尚未登入",
			})
			c.Abort()
			return
		}

		c.Next()
		return
	}
}
