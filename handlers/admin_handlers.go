package handlers

import (
	"Backend/models"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func isValidImageExtensions(file *multipart.FileHeader) bool {
	allowExtensions := []string{".jpg", ".jpeg", ".png"}
	fileExt := strings.ToLower(filepath.Ext(file.Filename))
	for _, allowExt := range allowExtensions {
		if fileExt == allowExt {
			return true
		}
	}
	return false
}

func makeUniqueFileName(file *multipart.FileHeader) string {
	fileExt := filepath.Ext(file.Filename)
	fileBase := strings.TrimSuffix(file.Filename, fileExt)
	return fmt.Sprintf("%s_%d%s", fileBase, time.Now().UnixNano(), fileExt)
}

// 查詢使用者列表
func GetUserListHandler(c *gin.Context, db *gorm.DB) {
	//嘗試獲取使用者列表
	var userList []struct {
		Id       uint
		Username string
	}
	err := db.
		Model(&models.User{}).
		Select("Id", "Username").
		Find(&userList).
		Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "無法獲取使用者列表",
			"error":   err.Error(),
		})
		return
	}

	//檢查使用者列表是否為空
	if len(userList) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "使用者列表為空",
		})
		return
	}

	//成功獲取使用者列表
	c.JSON(http.StatusOK, gin.H{
		"message":  "成功獲取使用者列表",
		"userList": userList,
	})
}

func GetProductAllDataHandler(c *gin.Context, db *gorm.DB) {
	productID := c.Param("productID")

	var product models.Product
	err := db.Preload("Categories").Find(&product, productID).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "查詢商品資料失敗",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "成功查詢商品資料",
		"product": product,
	})
}

func UploadImageHandler(c *gin.Context, db *gorm.DB) {
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "綁定圖片失敗",
			"error":   err.Error(),
		})
		return
	}

	if !isValidImageExtensions(file) {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "圖片檔案格式錯誤",
			"error":   err.Error(),
		})
		return
	}

	uploadsDir := "./uploads"
	//檢查uploads資料夾是否存在，如不存在則創建
	_, err = os.Stat(uploadsDir)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.Mkdir(uploadsDir, 0755); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "建立uploads資料夾失敗",
					"error":   err.Error(),
				})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "檢查uploads資料夾失敗",
				"error":   err.Error(),
			})
			return
		}
	}

	imageName := makeUniqueFileName(file)
	filePath := filepath.Join(uploadsDir, imageName)
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "儲存圖片失敗",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":   "成功上傳圖片",
		"imagePath": "/" + filepath.ToSlash(filePath),
	})
}

func CreateProductHandler(c *gin.Context, db *gorm.DB, rdb *redis.Client) {
	var newProduct struct {
		Name        string   `json:"name" binding:"required"`
		Price       uint     `json:"price" binding:"required"`
		Stock       uint     `json:"stock" binding:"required"`
		ImageURL    string   `json:"imageURL" binding:"required"`
		Description string   `json:"description"`
		Categories  []string `json:"categories"`
	}
	err := c.ShouldBindJSON(&newProduct)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "綁定請求資料錯誤",
			"error":   err.Error(),
		})
		return
	}

	//查詢已存在的標籤
	var mergeCategories []models.Category
	err = db.
		Model(&models.Category{}).
		Where("Name IN ?", newProduct.Categories).
		Find(&mergeCategories).
		Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "查詢標籤失敗",
			"error":   err.Error(),
		})
		return
	}

	//將尚未建立之標籤加入mergeCategories
	for _, categoryName := range newProduct.Categories {
		exists := false
		for _, mergeCategory := range mergeCategories {
			if categoryName == mergeCategory.Name {
				exists = true
			}
		}
		if !exists {
			mergeCategories = append(mergeCategories, models.Category{
				Name: categoryName,
			})
		}
	}

	product := models.Product{
		Name:        newProduct.Name,
		Price:       newProduct.Price,
		Stock:       newProduct.Stock,
		ImageURL:    newProduct.ImageURL,
		Description: newProduct.Description,
		Categories:  mergeCategories,
	}

	tx := db.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "開啟資料庫事務失敗",
			"error":   tx.Error.Error(),
		})
		return
	}

	err = tx.Create(&product).Error
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "新增商品失敗",
			"error":   err.Error(),
		})
		return
	}

	productJSON, err := json.Marshal(product)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "無法序列化商品資料",
			"error":   err.Error(),
		})
		return
	}

	err = rdb.ZAdd(c, "products", redis.Z{
		Score:  float64(product.ID),
		Member: productJSON,
	}).Err()
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "無法將商品資料加入Redis",
			"error":   err.Error(),
		})
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

	c.JSON(http.StatusCreated, gin.H{
		"message": "成功新增商品",
		"product": product,
	})
}

func UpdateProductHandler(c *gin.Context, db *gorm.DB, rdb *redis.Client) {
	productID := c.Param("productID")

	var productDataReq struct {
		Name        *string  `json:"name"`
		Price       *uint    `json:"price"`
		Stock       *uint    `json:"stock"`
		ImageURL    *string  `json:"imageURL"`
		Description *string  `json:"description"`
		Categories  []string `json:"categories"`
	}
	err := c.ShouldBind(&productDataReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "綁定請求資料錯誤",
			"error":   err.Error(),
		})
		return
	}

	var product models.Product
	err = db.First(&product, productID).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	if len(productDataReq.Categories) > 0 {
		err = db.Model(&product).Association("Categories").Clear()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		//查詢每個標籤，如不存在就創建
		var categories []models.Category
		for _, categoryName := range productDataReq.Categories {
			var category models.Category
			err = db.
				Model(&models.Category{}).
				Where("Name = ?", categoryName).
				FirstOrCreate(&category).
				Error
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			categories = append(categories, category)
		}

		product.Categories = categories
	}

	if productDataReq.Name != nil {
		product.Name = *productDataReq.Name
	}
	if productDataReq.Price != nil {
		product.Price = *productDataReq.Price
	}
	if productDataReq.Stock != nil {
		product.Stock = *productDataReq.Stock
	}
	if productDataReq.ImageURL != nil {
		product.ImageURL = *productDataReq.ImageURL
	}
	if productDataReq.Description != nil {
		product.Description = *productDataReq.Description
	}

	tx := db.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "開啟資料庫事務失敗",
			"error":   tx.Error.Error(),
		})
		return
	}

	result := tx.Save(&product)
	err = result.Error
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	productJSON, err := json.Marshal(product)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "無法序列化商品資料",
			"error":   err.Error(),
		})
		return
	}

	score := strconv.Itoa(int(product.ID))

	err = rdb.ZRemRangeByScore(c, "products", score, score).Err()
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "無法將商品資料從Redis刪除",
			"error":   err.Error(),
		})
		return
	}

	err = rdb.ZAdd(c, "products", redis.Z{
		Score:  float64(product.ID),
		Member: productJSON,
	}).Err()
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "無法將商品資料加入Redis",
			"error":   err.Error(),
		})
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

	if result.RowsAffected == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message": "沒有變更資料",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "成功修改商品資料",
	})
}

func DeleteProductHandler(c *gin.Context, db *gorm.DB, rdb *redis.Client) {
	productID := c.Param("productID")

	var product models.Product

	tx := db.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "開啟資料庫事務失敗",
			"error":   tx.Error.Error(),
		})
		return
	}

	err := tx.Preload("Categories").First(&product, productID).Error
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "查找此商品失敗",
			"error":   err.Error(),
		})
		return
	}

	err = tx.Model(&product).Association("Categories").Clear()
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "清除商品標籤關聯失敗",
			"error":   err.Error(),
		})
		return
	}

	err = tx.Delete(&product).Error
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "刪除商品失敗",
			"error":   err.Error(),
		})
		return
	}

	score := strconv.Itoa(int(product.ID))

	err = rdb.ZRemRangeByScore(c, "products", score, score).Err()
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "無法將商品資料從Redis刪除",
			"error":   err.Error(),
		})
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

	c.JSON(http.StatusOK, gin.H{
		"message": "成功刪除商品",
	})
}

func DeleteCategoryHandler(c *gin.Context, db *gorm.DB) {
	categoryID := c.Param("categoryID")

	var category models.Category
	err := db.First(&category, categoryID).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "查詢商品標籤失敗",
			"error":   err.Error(),
		})
		return
	}

	err = db.Model(&category).Association("Products").Clear()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "刪除商品標籤關聯失敗",
			"error":   err.Error(),
		})
		return
	}

	err = db.Delete(&category).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "刪除標籤失敗",
			"error":   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "成功刪除標籤",
	})
}
