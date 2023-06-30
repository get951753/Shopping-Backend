package models

import "gorm.io/gorm"

type CartItem struct {
	gorm.Model
	CartID    uint `gorm:"foreignKey:CartID"`
	Cart      Cart
	ProductID uint `gorm:"foreignKey:ProductID"`
	Product   Product
	Quantity  uint `gorm:"not null"`
}
