package models

import "gorm.io/gorm"

type OrderItem struct {
	gorm.Model
	OrderID   uint `gorm:"foreignKey:OrderID"`
	Order     Order
	ProductID uint `gorm:"foreignKey:ProductID"`
	Product   Product
	Quantity  uint `gorm:"not null"`
}
