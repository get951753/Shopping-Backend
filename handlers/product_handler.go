package handlers

import (
	"Backend/models"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"log"
	"net/http"
	"strconv"
)

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// 查詢商品列表
func GetProductListHandler(c *gin.Context, db *gorm.DB, rdb *redis.Client) {
	limit := c.DefaultQuery("limit", "10")
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "查詢數量輸入錯誤",
			"error":   err.Error(),
		})
	}
	//限制最高查詢數量為50
	if limitInt > 50 {
		limitInt = 50
	}

	offset := c.DefaultQuery("offset", "0")
	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "offset輸入錯誤",
			"error":   err.Error(),
		})
	}

	//嘗試從Redis讀取商品列表，如失敗則從資料庫讀取並儲存至Redis
	redisProducts, err := rdb.ZRange(c, "products", int64(offsetInt), int64(offsetInt+limitInt-1)).Result()
	if err != nil || rdb.ZCard(c, "products").Val() == 0 {
		log.Println("err")
		var products []models.Product
		err = db.Preload("Categories").Find(&products).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "無法讀取商品列表",
				"error":   err.Error(),
			})
			return
		}

		rdb.Del(c, "products")

		for _, product := range products {
			productJSON, err := json.Marshal(product)
			if err != nil {
				fmt.Printf("無法序列化商品資料: %v\n", err)
				continue
			}

			err = rdb.ZAdd(c, "products", redis.Z{
				Score:  float64(product.ID),
				Member: productJSON,
			}).Err()
			if err != nil {
				fmt.Printf("無法將商品資料加入Redis: %v\n", err)
				continue
			}
		}

		//再次嘗試從Redis讀取商品列表
		redisProducts, err = rdb.ZRange(c, "products", int64(offsetInt), int64(offsetInt+limitInt-1)).Result()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "無法從Redis讀取商品列表",
				"error":   err.Error(),
			})
			return
		}
	}

	var productsData []struct {
		ID       uint
		Name     string
		Price    uint
		Stock    uint
		ImageURL string
	}

	for _, redisProduct := range redisProducts {
		var productUnmarshal models.Product
		err = json.Unmarshal([]byte(redisProduct), &productUnmarshal)
		if err != nil {
			fmt.Printf("無法反序列化商品資料: %v\n", err)
			continue
		}

		productsData = append(productsData, struct {
			ID       uint
			Name     string
			Price    uint
			Stock    uint
			ImageURL string
		}{
			ID:       productUnmarshal.ID,
			Name:     productUnmarshal.Name,
			Price:    productUnmarshal.Price,
			Stock:    productUnmarshal.Stock,
			ImageURL: productUnmarshal.ImageURL,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "成功讀取商品列表",
		"products":   productsData,
		"totalCount": rdb.ZCard(c, "products").Val(),
	})
}

// 搜尋完整包含標籤的所有商品
func GetProductsFromCategoriesHandler(c *gin.Context, db *gorm.DB, rdb *redis.Client) {
	limit := c.DefaultQuery("limit", "10")
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "查詢數量輸入錯誤",
			"error":   err.Error(),
		})
	}
	//限制最高查詢數量為50
	if limitInt > 50 {
		limitInt = 50
	}

	offset := c.DefaultQuery("offset", "0")
	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "offset輸入錯誤",
			"error":   err.Error(),
		})
	}

	var categoriesReq []struct {
		CategoryID uint `json:"categoryID" binding:"required"`
	}
	err = c.ShouldBindJSON(&categoriesReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "綁定請求資料錯誤",
			"error":   err.Error(),
		})
		return
	}

	redisProducts, err := rdb.ZRange(c, "products", 0, -1).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "無法取得商品列表",
			"error":   err.Error(),
		})
		return
	}

	//遍歷從Redis讀出的商品列表，找出含有所有標籤的商品
	var productsData []gin.H
	for _, redisProduct := range redisProducts {
		var productUnmarshal models.Product
		err = json.Unmarshal([]byte(redisProduct), &productUnmarshal)
		if err != nil {
			fmt.Printf("無法反序列化商品資料: %v\n", err)
			continue
		}

		hasALLTags := true

		for _, categoryReq := range categoriesReq {
			found := false
			for _, productCategory := range productUnmarshal.Categories {
				if productCategory.ID == categoryReq.CategoryID {
					found = true
					break
				}
			}
			if !found {
				hasALLTags = false
				break
			}
		}

		if hasALLTags == true {
			categoriesData := make([]gin.H, len(productUnmarshal.Categories))
			for i, category := range productUnmarshal.Categories {
				categoriesData[i] = gin.H{
					"name": category.Name,
					"ID":   category.ID,
				}
			}
			productsData = append(productsData, gin.H{
				"name":       productUnmarshal.Name,
				"price":      productUnmarshal.Price,
				"stock":      productUnmarshal.Stock,
				"imageURL":   productUnmarshal.ImageURL,
				"Categories": categoriesData,
			})
		}
	}

	totalCount := len(productsData)

	//預防offset跟limit超出搜尋結果切片
	if offsetInt >= totalCount {
		c.JSON(http.StatusBadRequest, gin.H{
			"message":    "offset超過商品數量",
			"totalCount": len(productsData),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "成功讀取商品列表",
		"products":   productsData[offsetInt:min(offsetInt+limitInt, totalCount)],
		"totalCount": len(productsData),
	})
}

// 查詢商品詳細資料
func GetProductDataHandler(c *gin.Context, db *gorm.DB) {
	productID := c.Param("productID")

	var product struct {
		ID          uint
		Name        string
		Price       uint
		Stock       uint
		Description string
		ImageURL    string
	}
	err := db.Model(&models.Product{}).Where("id = ?", productID).First(&product).Error
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

// 查詢商品標籤列表
func GetCategoryListHandler(c *gin.Context, db *gorm.DB) {
	var categories []struct {
		Id   uint
		Name string
	}
	err := db.
		Model(&models.Category{}).
		Select("Id", "Name").
		Find(&categories).
		Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "無法讀取商品標籤列表",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "成功讀取商品標籤列表",
		"products": categories,
	})
}
