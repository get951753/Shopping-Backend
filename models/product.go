package models

import "gorm.io/gorm"

type Product struct {
	gorm.Model
	Name        string `gorm:"not null"`
	Price       uint   `gorm:"not null"`
	Stock       uint   `gorm:"not null"`
	Description string
	ImageURL    string
	Categories  []Category `gorm:"many2many:category_products;"`
}
