package handlers

import (
	"Backend/models"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"log"
	"net/http"
)

func SendOrderHandler(c *gin.Context, db *gorm.DB, rdb *redis.Client) {
	userID, ok := c.Get("UserID")
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "無法取得使用者ID",
		})
		return
	}

	var orderReq struct {
		Name           string             `json:"name" binding:"required"`
		Address        string             `json:"address" binding:"required"`
		Phone          string             `json:"phone" binding:"required"`
		ShippingMethod string             `json:"shippingMethod" binding:"required"`
		OrderItems     []models.OrderItem `json:"orderItems" binding:"required"`
	}

	err := c.ShouldBindJSON(&orderReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "取得訂單資料錯誤",
			"error":   err.Error(),
		})
		return
	}

	newOrder := models.Order{
		UserID:     userID.(uint),
		OrderItems: orderReq.OrderItems,
		Name:       orderReq.Name,
		Address:    orderReq.Address,
		Phone:      orderReq.Phone,
		Status:     "待處理",
	}

	var orderProductIDs []uint
	totalPrice := uint(0)

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "開啟資料庫事務失敗",
			"error":   tx.Error.Error(),
		})
		return
	}

	for _, orderItem := range orderReq.OrderItems {
		var product models.Product
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", orderItem.ProductID).
			First(&product).
			Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "查詢庫存失敗",
				"error":   err.Error(),
			})
			return
		}

		if product.Stock < orderItem.Quantity {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "商品庫存不足",
			})
			return
		}

		product.Stock -= orderItem.Quantity
		if err := tx.Save(&product).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "更新庫存失敗",
				"error":   err.Error(),
			})
			return
		}

		err, msg := UpdateProductToRedis(c, rdb, &product)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": msg,
				"error":   err.Error(),
			})
			return
		}

		orderProductIDs = append(orderProductIDs, orderItem.ProductID)
		totalPrice += product.Price
	}

	newOrder.Total = totalPrice

	err = tx.Create(&newOrder).Error
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "提交訂單失敗",
			"error":   err.Error(),
		})
		log.Printf("提交訂單失敗 Error: %s, %v", err.Error(), newOrder.OrderItems)
		return
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "提交事務失敗",
			"error":   err.Error(),
		})
		return
	}

	var cart models.Cart
	err = db.Where("user_id = ?", userID).First(&cart).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "訂單已送出，但清除購物車對應商品失敗",
			"error":   err.Error(),
		})
		return
	}

	err = db.
		Where("cart_id = ? AND product_id IN ?", cart.ID, orderProductIDs).
		Delete(&models.CartItem{}).
		Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "訂單已送出，但清除購物車對應商品失敗",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "訂單已送出，成功清除購物車對應商品",
	})
}

func GetOrderListHandler(c *gin.Context, db *gorm.DB) {
	userID, ok := c.Get("UserID")
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "無法取得使用者ID",
		})
		return
	}

	var orders []models.Order
	err := db.Where("user_id = ?", userID).Find(&orders).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "查詢訂單列表失敗",
			"error":   err.Error(),
		})
		return
	}

	var orderList []gin.H
	for _, order := range orders {
		orderList = append(orderList, gin.H{
			"OrderID":        order.ID,
			"OrderTime":      order.CreatedAt,
			"ShippingMethod": order.ShippingMethod,
			"Total":          order.Total,
			"Status":         order.Status,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "成功查詢訂單列表",
		"orderList": orderList,
	})
}

func GetOrderDataHandler(c *gin.Context, db *gorm.DB) {
	orderID := c.Param("orderID")
	userID, ok := c.Get("UserID")
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "無法取得使用者ID",
		})
		return
	}

	var order models.Order
	err := db.
		Where("id = ? AND user_id = ?", orderID, userID).
		Preload("OrderItems").
		Preload("OrderItems.Product").
		First(&order).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "查詢訂單失敗",
			"error":   err.Error(),
		})
		return
	}

	var orderItemsData []gin.H
	for _, orderItem := range order.OrderItems {
		orderItemsData = append(orderItemsData, gin.H{
			"ProductID": orderItem.Product.ID,
			"Name":      orderItem.Product.Name,
			"Price":     orderItem.Product.Price,
			"ImageURL":  orderItem.Product.ImageURL,
			"Quantity":  orderItem.Quantity,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "成功查詢訂單",
		"OrderID":        order.ID,
		"Name":           order.Name,
		"Address":        order.Address,
		"Phone":          order.Phone,
		"ShippingMethod": order.ShippingMethod,
		"Total":          order.Total,
		"OrderTime":      order.CreatedAt,
		"Status":         order.Status,
		"orderItemsData": orderItemsData,
	})
}
