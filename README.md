# 購物網站後端API

使用 Golang + Gin + Gorm + MySQL + Redis + JWT Token 撰寫。

具備註冊、登入、修改會員資料、購物車、訂單查詢和送出等功能。

使用Redis加速快取商品列表，在編輯商品和送出訂單時同步更新Redis資訊以防止資料不同步。

如客戶端請求需要權限之API，伺服器端會驗證身分並檢查Token是否已過期或登出，通過驗證才放行。

## 路由簡介

**以下路由不須登入即可請求，但仍然會檢查登入狀況。**

| 路由                               | 簡介                         |
|----------------------------------|-------------------------------|
| **GET** /api/products            | 查詢商品列表 (使用Redis加速)      |
| **GET** /api/products/categories | 搜尋完整包含標籤的所有商品         |
| **GET** /api/product/:productID  | 查詢商品詳細資料                 |
| **POST** /api/register           | 註冊帳號                        |
| **POST** /api/login              | 登入帳號                        |
| **POST** /api/carts/add          | 新增商品至購物車                 |
| **POST** /api/carts/update       | 更新購物車商品數量               |
| **DELETE** /api/carts/:productID | 刪除購物車商品                  |
| **GET** /api/carts               | 查詢購物車商品                  |
| **DELETE** /api/carts            | 清除購物車商品                  |

**以下路由須要登入才能請求。**

| 路由                      | 簡介                                  |
|-------------------------|----------------------------------------|
| **GET** /profile        | 查詢使用者資料                            |
| **PATCH** /profile/edit | 修改使用者資料                            |
| **POST** /carts/merge   | 合併匿名和使用者購物車(登入或註冊後呼叫)      |
| **POST** /order         | 送出訂單並清除購物車內對應商品               |
| **GET** /orders         | 查詢訂單列表                              |
| **GET** /order/:orderID | 查詢訂單詳細資訊                           |
| **POST** /logout        | 登出                                     |

**以下路由須要登入admin身分才能請求。**

| 路由                                 | 簡介                                    |
|------------------------------------|-----------------------------------------|
| **GET** /users                     | 查詢使用者列表                             |
| **GET** /product/:productID        | 查詢商品所有資料                            |
| **POST** /image                    | 上傳商品圖片                               |
| **POST** /product                  | 新增商品                                  |
| **PATCH** /product/:productID      | 修改商品                                  |
| **DELETE** /product/:productID     | 刪除商品                                  |
| **GET** /categories                | 查詢商品標籤列表                            |
| **DELETE** /categories/:categoryID | 刪除商品標籤                               |


## 執行前的設定

**1.需先執行Mysql和Redis，可用Docker Compose快速架設。**

```YAML
version: '3'
services:
  db:
    image: mysql
    ports:
      - 3306:3306
    environment:
      - MYSQL_ROOT_PASSWORD={YOUR_ROOT_PASSWORD}
      - MYSQL_DATABASE={YOUR_DATABASE}
      - MYSQL_USER={YOUR_USERNAME}
      - MYSQL_PASSWORD={YOUR_PASSWORD}
  admin:
    image: adminer
    ports:
      - 8888:8080
  redis:
    image: redis
    ports:
      - 6379:6379
    volumes:
      - redis-data:/data

volumes:
  redis-data:
```

**2.在config/config.yaml設定連線資訊。**

```YAML
database:
  username: {YOUR_USERNAME}
  password: {YOUR_PASSWORD}
  host: "127.0.0.1"
  port: "3306"
  database: {YOUR_DATABASE}

redis:
  addr: "127.0.0.1:6379"
  password: ""
  database: 0
```

**3.在jwt資料夾使用openssl生成公鑰和私鑰。**

```
openssl genpkey -algorithm RSA -out private.pem -pkeyopt rsa_keygen_bits:2048
openssl rsa -in private.pem -pubout -out public.pem
```