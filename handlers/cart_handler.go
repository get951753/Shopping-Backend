package handlers

import (
	"Backend/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"net/http"
)

func generateAnonymousCartID() string {
	id := uuid.New()
	return id.String()
}

// 從Cookie讀取匿名購物車ID
func getAnonymousCartID(c *gin.Context) string {
	anonymousCartID, err := c.Request.Cookie("anonymous_cart_id")
	if err != nil {
		return ""
	}
	return anonymousCartID.Value
}

// 儲存匿名購物車ID至Cookie
func setAnonymousCartID(c *gin.Context, cartID string) {
	cookie := http.Cookie{
		Name:     "anonymous_cart_id",
		Value:    cartID,
		Path:     "/",
		HttpOnly: true,
	}
	http.SetCookie(c.Writer, &cookie)
}

func AddToCartHandler(c *gin.Context, db *gorm.DB) {
	var cartItemReq struct {
		ProductID uint
		Quantity  uint
	}
	err := c.BindJSON(&cartItemReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "綁定請求資料錯誤",
			"error":   err.Error(),
		})
	}

	userID, login := c.Get("UserID")
	var cart models.Cart
	query := db
	if !login {
		//判斷是否已有匿名購物車
		anonymousCartID := getAnonymousCartID(c)
		if anonymousCartID == "" {
			newAnonymousCartID := generateAnonymousCartID()
			setAnonymousCartID(c, newAnonymousCartID)

			//新增匿名購物車
			newCart := &models.Cart{
				UserID:            0,
				AnonymousCartUUID: newAnonymousCartID,
			}
			result := db.Create(&newCart)
			if result.Error != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "新增購物車失敗",
				})
				return
			}
			cart.ID = newCart.ID
		} else {
			query = query.Where("anonymous_cart_uuid = ?", anonymousCartID)
		}
	} else {
		query = query.Where("user_id = ?", userID)
	}

	//查詢購物車ID
	err = query.
		FirstOrCreate(&cart).
		Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查詢購物車失敗",
		})
		return
	}

	//查詢商品庫存數量
	var productStock uint
	err = db.
		Model(&models.Product{}).
		Select("Stock").
		Where("id = ?", cartItemReq.ProductID).
		First(&productStock).
		Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "查詢商品庫存錯誤",
			"error":   err.Error(),
		})
		return
	}

	//新增商品至購物車
	var cartItem models.CartItem
	err = db.
		Where("product_id = ? AND cart_id = ?", cartItemReq.ProductID, cart.ID).
		First(&cartItem).
		Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			//購物車沒有相同物品，新增此物品至購物車
			if cartItemReq.Quantity > productStock {
				cartItemReq.Quantity = productStock
			}
			err := db.Create(&models.CartItem{
				CartID:    cart.ID,
				ProductID: cartItemReq.ProductID,
				Quantity:  cartItemReq.Quantity,
			}).Error
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "新增物品至購物車失敗",
					"error":   err.Error(),
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message":   "成功新增物品至購物車",
				"productID": cartItemReq.ProductID,
				"Quantity":  cartItemReq.Quantity,
			})
			return
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "查詢購物車商品錯誤",
				"error":   err.Error(),
			})
			return
		}
	}
	//購物車有相同物品，增加商品數量
	cartItem.Quantity += cartItemReq.Quantity
	if cartItem.Quantity > productStock {
		cartItem.Quantity = productStock
	}
	db.Updates(&cartItem)
	c.JSON(http.StatusOK, gin.H{
		"message":   "成功更新購物車物品數量",
		"productID": cartItem.ProductID,
		"Quantity":  cartItem.Quantity,
	})
	return
}

// 減少購物車商品
func UpdateCartItemQuantityHandler(c *gin.Context, db *gorm.DB) {
	var cartItemReq struct {
		ProductID uint
		Quantity  uint
	}
	err := c.BindJSON(&cartItemReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "綁定請求資料錯誤",
			"error":   err.Error(),
		})
		return
	}

	if cartItemReq.Quantity < 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "商品數量不得小於1",
		})
		return
	}

	userID, login := c.Get("UserID")
	var cart models.Cart
	query := db
	if !login {
		//判斷是否已有匿名購物車
		anonymousCartID := getAnonymousCartID(c)
		if anonymousCartID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "尚未建立匿名購物車",
			})
			return
		} else {
			query = query.Where("anonymous_cart_uuid = ?", anonymousCartID)
		}
	} else {
		query = query.Where("user_id = ?", userID)
	}

	//查詢購物車ID
	err = query.
		First(&cart).
		Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "查無此購物車",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查詢購物車失敗",
		})
		return
	}

	//查詢購物車商品
	var cartItem models.CartItem
	err = db.
		Where("product_id = ? AND cart_id = ?", cartItemReq.ProductID, cart.ID).
		First(&cartItem).
		Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "購物車沒有此商品",
			})
			return
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "查詢購物車商品錯誤",
				"error":   err.Error(),
			})
			return
		}
	}
	//如果請求減少的數量大於庫存則更新為庫存數量
	if cartItemReq.Quantity > cartItem.Product.Stock {
		cartItem.Quantity = cartItem.Product.Stock
	} else {
		cartItem.Quantity = cartItemReq.Quantity
	}
	db.Updates(&cartItem)
	c.JSON(http.StatusOK, gin.H{
		"message":   "成功減少購物車物品數量",
		"productID": cartItem.ProductID,
		"Quantity":  cartItem.Quantity,
	})
	return
}

//刪除購物車商品
func DeleteCartItemHandler(c *gin.Context, db *gorm.DB) {
	productID := c.Param("productID")

	userID, login := c.Get("UserID")
	var cart models.Cart
	query := db
	if !login {
		//判斷是否已有匿名購物車
		anonymousCartID := getAnonymousCartID(c)
		if anonymousCartID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "尚未建立匿名購物車",
			})
			return
		} else {
			query = query.Where("anonymous_cart_uuid = ?", anonymousCartID)
		}
	} else {
		query = query.Where("user_id = ?", userID)
	}

	//查詢購物車ID
	err := query.
		First(&cart).
		Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "查無此購物車",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查詢購物車失敗",
		})
		return
	}

	//刪除購物車商品
	var cartItem models.CartItem
	err = db.
		Where("product_id = ? AND cart_id = ?", productID, cart.ID).
		Delete(&cartItem).
		Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "購物車沒有此商品",
			})
			return
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "刪除購物車商品錯誤",
				"error":   err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "成功刪除購物車物品",
		"productID": productID,
	})
	return
}

func MergeCartHandler(c *gin.Context, db *gorm.DB) {
	userID, ok := c.Get("UserID")
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "無法取得使用者ID",
		})
		return
	}

	//判斷是否已有匿名購物車
	anonymousCartID := getAnonymousCartID(c)
	if anonymousCartID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "尚未創建匿名購物車，無須合併",
		})
		return
	}

	//查詢匿名購物車
	var anonymousCart models.Cart
	err := db.
		Where("anonymous_cart_uuid = ?", anonymousCartID).
		Preload("CartItems").
		Preload("CartItems.Product").
		First(&anonymousCart).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查詢匿名購物車失敗",
		})
		return
	}

	//查詢會員購物車
	var cart models.Cart
	err = db.
		Where("user_id = ?", userID).
		Preload("CartItems").
		Preload("CartItems.Product").
		First(&cart).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			cartItems := make([]models.CartItem, len(anonymousCart.CartItems))
			for i, anonCartItem := range anonymousCart.CartItems {
				cartItems[i].CartID = cart.ID
				cartItems[i].ProductID = anonCartItem.ProductID
				cartItems[i].Quantity = anonCartItem.Quantity
			}
			cart = models.Cart{
				UserID:    userID.(uint),
				CartItems: cartItems,
			}
			err := db.Create(&cart).Error
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "合併購物車商品失敗",
					"error":   err.Error(),
				})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "查詢購物車失敗",
				"error":   err.Error(),
			})
			return
		}
	} else {
		//檢查重複商品並合併購物車商品
		for _, anonCartItem := range anonymousCart.CartItems {
			itemExists := false
			for _, cartItem := range cart.CartItems {
				if cartItem.ProductID == anonCartItem.ProductID {
					itemExists = true
					cartItem.Quantity += anonCartItem.Quantity
					if cartItem.Quantity > cartItem.Product.Stock {
						cartItem.Quantity = cartItem.Product.Stock
					}
					err := db.Updates(&cartItem).Error
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{
							"message": "更新購物車商品數量失敗",
							"error":   err.Error(),
						})
						return
					}
					break
				}
			}
			if !itemExists {
				cart.CartItems = append(cart.CartItems, models.CartItem{
					CartID:    cart.ID,
					ProductID: anonCartItem.ProductID,
					Quantity:  anonCartItem.Quantity,
				})
			}
		}

		err := db.Updates(&cart).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "合併購物車商品失敗",
				"error":   err.Error(),
			})
			return
		}
	}

	err = db.Where("cart_id = ?", &anonymousCart.ID).Delete(&models.CartItem{}).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "成功合併購物車商品，清空匿名購物車失敗",
		})
		return
	}

	err = db.Delete(&anonymousCart).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "成功合併購物車商品且清空匿名購物車，刪除匿名購物車失敗",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "成功合併商品至購物車且刪除匿名購物車",
	})
}

func GetCartHandler(c *gin.Context, db *gorm.DB) {
	userID, login := c.Get("UserID")

	query := db
	var cart models.Cart
	if !login {
		anonymousCartID := getAnonymousCartID(c)
		if anonymousCartID == "" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "尚未創建匿名購物車",
			})
			return
		}
		query = query.Where("anonymous_cart_uuid = ?", anonymousCartID)
	} else {
		query = query.Where("user_id = ?", userID)
	}

	err := query.
		Preload("CartItems").
		Preload("CartItems.Product").
		First(&cart).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "查詢購物車失敗",
			"error":   err.Error(),
		})
		return
	}

	var cartItemsData []gin.H
	for _, cartItem := range cart.CartItems {
		cartItemsData = append(cartItemsData, gin.H{
			"ProductID": cartItem.Product.ID,
			"Name":      cartItem.Product.Name,
			"Price":     cartItem.Product.Price,
			"ImageURL":  cartItem.Product.ImageURL,
			"Quantity":  cartItem.Quantity,
			"Stock":     cartItem.Product.Stock,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "成功查詢購物車",
		"cartItemsData": cartItemsData,
	})
}

func ClearCartHandler(c *gin.Context, db *gorm.DB) {
	userID, login := c.Get("UserID")

	query := db
	var cart models.Cart
	if !login {
		anonymousCartID := getAnonymousCartID(c)
		if anonymousCartID == "" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "尚未創建匿名購物車",
			})
			return
		}
		query = query.Where("anonymous_cart_uuid = ?", anonymousCartID)
	} else {
		query = query.Where("user_id = ?", userID)
	}

	err := query.First(&cart).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "查詢購物車失敗",
			"error":   err.Error(),
		})
		return
	}

	err = db.Where("cart_id = ?", &cart.ID).Delete(&models.CartItem{}).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "清空購物車失敗",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "成功清空購物車",
	})
}
