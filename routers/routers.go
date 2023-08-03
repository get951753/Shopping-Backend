package routers

import (
	"Backend/handlers"
	"Backend/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"net/http"
)

func SetupRouters(db *gorm.DB, rdb *redis.Client) *gin.Engine {
	//建立Gin路由器
	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Authorization")
		c.Next()
	})
	err := router.SetTrustedProxies(nil)
	if err != nil {
		return nil
	}

	//設定商品圖片靜態資源路徑
	router.Static("/uploads", "./uploads")

	router.OPTIONS("/*path", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	////無須權限，使用中間件檢查是否登入
	router.Use(middleware.AuthMiddleware(db))
	{
		//查詢商品列表
		router.GET("/api/v1/products", func(context *gin.Context) {
			handlers.GetProductListHandler(context, db, rdb)
		})
		//搜尋完整包含標籤的所有商品
		router.GET("/api/v1/products/categories", func(context *gin.Context) {
			handlers.GetProductsFromCategoriesHandler(context, db, rdb)
		})
		//查詢商品詳細資料
		router.GET("/api/v1/products/:productID", func(context *gin.Context) {
			handlers.GetProductDataHandler(context, db)
		})
		//註冊帳號
		router.POST("/api/v1/register", func(context *gin.Context) {
			handlers.RegisterHandler(context, db)
		})
		//登入帳號
		router.POST("/api/v1/login", func(context *gin.Context) {
			handlers.LoginHandler(context, db)
		})
		//新增商品至購物車
		router.POST("/api/v1/carts/add", func(context *gin.Context) {
			handlers.AddToCartHandler(context, db)
		})
		//更新購物車商品數量
		router.POST("/api/v1/carts/update", func(context *gin.Context) {
			handlers.UpdateCartItemQuantityHandler(context, db)
		})
		//刪除購物車商品
		router.DELETE("/api/v1/carts/:productID", func(context *gin.Context) {
			handlers.DeleteCartItemHandler(context, db)
		})
		//查詢購物車商品
		router.GET("/api/v1/carts", func(context *gin.Context) {
			handlers.GetCartHandler(context, db)
		})
		//清除購物車商品
		router.DELETE("/api/v1/carts", func(context *gin.Context) {
			handlers.ClearCartHandler(context, db)
		})

		////需要登入，使用中間件檢查是否登入
		loginRequired := router.Group("/api/v1/user")
		loginRequired.Use(middleware.CheckLoginMiddleware())
		{
			//查詢使用者資料
			loginRequired.GET("/profile", func(context *gin.Context) {
				handlers.GetUserProfileHandler(context, db)
			})
			//修改使用者資料
			loginRequired.PATCH("/profile/edit", func(context *gin.Context) {
				handlers.UpdateUserProfileHandler(context, db)
			})
			//合併匿名和使用者購物車(登入或註冊後呼叫)
			loginRequired.POST("/carts/merge", func(context *gin.Context) {
				handlers.MergeCartHandler(context, db)
			})
			//送出訂單並清除購物車內對應商品
			loginRequired.POST("/orders", func(context *gin.Context) {
				handlers.SendOrderHandler(context, db, rdb)
			})
			//查詢訂單列表
			loginRequired.GET("/orders", func(context *gin.Context) {
				handlers.GetOrderListHandler(context, db)
			})
			//查詢訂單詳細資訊
			loginRequired.GET("/orders/:orderID", func(context *gin.Context) {
				handlers.GetOrderDataHandler(context, db)
			})
			//登出
			loginRequired.POST("/logout", func(context *gin.Context) {
				handlers.LogOutHandler(context, db)
			})
		}

		////需要admin身分，使用中間件檢查是否登入及admin權限
		adminRequired := router.Group("/api/v1/admin")
		adminRequired.Use(middleware.CheckLoginMiddleware(), middleware.CheckAdminPermissionMiddleware())
		{
			//查詢使用者列表
			adminRequired.GET("/users", func(context *gin.Context) {
				handlers.GetUserListHandler(context, db)
			})
			//上傳商品圖片
			adminRequired.POST("/image", func(context *gin.Context) {
				handlers.UploadImageHandler(context)
			})
			//查詢商品完整資料
			adminRequired.GET("/products/:productID", func(context *gin.Context) {
				handlers.GetProductAllDataHandler(context, db)
			})
			//新增商品
			adminRequired.POST("/products", func(context *gin.Context) {
				handlers.CreateProductHandler(context, db, rdb)
			})
			//修改商品
			adminRequired.PATCH("/products/:productID", func(context *gin.Context) {
				handlers.UpdateProductHandler(context, db, rdb)
			})
			//刪除商品
			adminRequired.DELETE("/products/:productID", func(context *gin.Context) {
				handlers.DeleteProductHandler(context, db, rdb)
			})
			//查詢商品標籤列表
			adminRequired.GET("/categories", func(context *gin.Context) {
				handlers.GetCategoryListHandler(context, db)
			})
			//刪除商品標籤
			adminRequired.DELETE("/categories/:categoryID", func(context *gin.Context) {
				handlers.DeleteCategoryHandler(context, db)
			})
		}
	}

	return router
}
